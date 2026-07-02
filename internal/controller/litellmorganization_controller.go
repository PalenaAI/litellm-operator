/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMOrganizationReconciler reconciles a LiteLLMOrganization object.
type LiteLLMOrganizationReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmorganizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmorganizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmorganizations/finalizers,verbs=update

func (r *LiteLLMOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var org litellmv1alpha1.LiteLLMOrganization
	if err := r.Get(ctx, req.NamespacedName, &org); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !org.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &org)
	}

	if !controllerutil.ContainsFinalizer(&org, FinalizerName) {
		controllerutil.AddFinalizer(&org, FinalizerName)
		if err := r.Update(ctx, &org); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, org.Namespace, org.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		emitEvent(r.Recorder, &org, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", org.Spec.InstanceRef.Name, err)
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "InstanceNotReady", Message: err.Error(),
			ObservedGeneration: org.Generation,
		})
		_ = r.Status().Update(ctx, &org)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileOrganization(ctx, &org, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			emitEvent(r.Recorder, &org, corev1.EventTypeWarning, EventReasonEnterpriseRequired,
				"Organization %q requires a LiteLLM Enterprise license", org.Spec.OrganizationAlias)
			meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: org.Generation,
			})
			_ = r.Status().Update(ctx, &org)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to reconcile organization")
		emitEvent(r.Recorder, &org, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile organization %q: %v", org.Spec.OrganizationAlias, err)
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "SyncFailed", Message: err.Error(),
			ObservedGeneration: org.Generation,
		})
	} else {
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionTrue, Reason: "Synced", Message: "Organization synced to LiteLLM",
			ObservedGeneration: org.Generation,
		})
	}

	if statusErr := r.Status().Update(ctx, &org); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

func (r *LiteLLMOrganizationReconciler) reconcileOrganization(
	ctx context.Context,
	org *litellmv1alpha1.LiteLLMOrganization,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))

	if org.Status.LiteLLMOrganizationID == "" {
		req := litellm.OrganizationCreateRequest{
			OrganizationAlias: org.Spec.OrganizationAlias,
			Models:            org.Spec.Models,
			MaxBudget:         org.Spec.MaxBudget,
			BudgetDuration:    org.Spec.BudgetDuration,
			TpmLimit:          org.Spec.TpmLimit,
			RpmLimit:          org.Spec.RpmLimit,
			SoftBudget:        org.Spec.SoftBudget,
			ModelRPMLimit:     org.Spec.ModelRPMLimit,
			ModelTPMLimit:     org.Spec.ModelTPMLimit,
			ObjectPermission:  mapObjectPermission(org.Spec.ObjectPermission),
			Metadata:          org.Spec.Metadata,
		}
		resp, err := apiClient.Organizations().Create(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create organization: %w", err)
		}
		org.Status.LiteLLMOrganizationID = resp.OrganizationID
		org.Status.Synced = true
		if err := r.Status().Update(ctx, org); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if org.Annotations == nil {
			org.Annotations = map[string]string{}
		}
		org.Annotations[AnnotationSyncHash] = computeSpecHash(org.Spec)
		if err := r.Update(ctx, org); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created organization", "organizationId", resp.OrganizationID)
		emitEvent(r.Recorder, org, corev1.EventTypeNormal, EventReasonCreated,
			"Organization %q registered with LiteLLM (id=%s)", org.Spec.OrganizationAlias, resp.OrganizationID)
	} else {
		currentHash := computeSpecHash(org.Spec)
		if org.Annotations[AnnotationSyncHash] != currentHash {
			req := litellm.OrganizationUpdateRequest{
				OrganizationID:    org.Status.LiteLLMOrganizationID,
				OrganizationAlias: org.Spec.OrganizationAlias,
				Models:            org.Spec.Models,
				MaxBudget:         org.Spec.MaxBudget,
				BudgetDuration:    org.Spec.BudgetDuration,
				TpmLimit:          org.Spec.TpmLimit,
				RpmLimit:          org.Spec.RpmLimit,
				SoftBudget:        org.Spec.SoftBudget,
				ModelRPMLimit:     org.Spec.ModelRPMLimit,
				ModelTPMLimit:     org.Spec.ModelTPMLimit,
				ObjectPermission:  mapObjectPermission(org.Spec.ObjectPermission),
				Metadata:          org.Spec.Metadata,
			}
			if err := apiClient.Organizations().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update organization: %w", err)
			}
			if org.Annotations == nil {
				org.Annotations = map[string]string{}
			}
			org.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, org); err != nil {
				return ctrl.Result{}, err
			}
			org.Status.Synced = true
			log.Info("updated organization", "organizationId", org.Status.LiteLLMOrganizationID)
			emitEvent(r.Recorder, org, corev1.EventTypeNormal, EventReasonUpdated,
				"Organization %q updated in LiteLLM", org.Spec.OrganizationAlias)
		}
	}

	// Reconcile members only when spec has members defined
	if len(org.Spec.Members) > 0 {
		if err := r.reconcileMembers(ctx, org, apiClient.Organizations()); err != nil {
			log.Error(err, "failed to reconcile members, will retry")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("reconcile members: %w", err)
		}
	}

	// Refresh spend and member count from API (best-effort)
	info, err := apiClient.Organizations().Get(ctx, org.Status.LiteLLMOrganizationID)
	if err == nil && info != nil {
		org.Status.CurrentSpend = info.Spend
		org.Status.MemberCount = len(info.Members)
	} else if err != nil {
		log.V(1).Info("failed to refresh organization info from API", "error", err)
	}

	now := metav1.Now()
	org.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMOrganizationReconciler) reconcileMembers(
	ctx context.Context,
	org *litellmv1alpha1.LiteLLMOrganization,
	orgSvc litellm.OrganizationService,
) error {
	log := logf.FromContext(ctx)

	info, err := orgSvc.Get(ctx, org.Status.LiteLLMOrganizationID)
	if err != nil {
		return fmt.Errorf("get organization info: %w", err)
	}

	desired := make(map[string]string, len(org.Spec.Members))
	for _, m := range org.Spec.Members {
		desired[m.Email] = m.Role
	}

	actual := make(map[string]string, len(info.Members))
	for _, m := range info.Members {
		actual[m.Email] = m.Role
	}

	// Add missing members
	for _, m := range org.Spec.Members {
		if _, exists := actual[m.Email]; !exists {
			role := m.Role
			if role == "" {
				role = "internal_user"
			}
			req := litellm.OrgMemberAddRequest{
				OrganizationID: org.Status.LiteLLMOrganizationID,
				Member: litellm.OrgMemberRequest{
					UserEmail: m.Email,
					Role:      role,
				},
			}
			if err := orgSvc.AddMember(ctx, req); err != nil {
				log.Error(err, "failed to add organization member", "email", m.Email)
				continue
			}
		}
	}

	// Remove members not in spec
	for _, am := range info.Members {
		if _, exists := desired[am.Email]; !exists {
			if err := orgSvc.DeleteMember(ctx, org.Status.LiteLLMOrganizationID, am.UserID); err != nil {
				log.Error(err, "failed to remove organization member", "email", am.Email)
				continue
			}
		}
	}

	org.Status.MemberCount = len(org.Spec.Members)
	return nil
}

func (r *LiteLLMOrganizationReconciler) handleDeletion(ctx context.Context, org *litellmv1alpha1.LiteLLMOrganization) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(org, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if org.Status.LiteLLMOrganizationID != "" {
		resolved, err := resolveInstance(ctx, r.Client, org.Namespace, org.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))
			if err := apiClient.Organizations().Delete(ctx, org.Status.LiteLLMOrganizationID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete organization from LiteLLM")
				emitEvent(r.Recorder, org, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete organization %q from LiteLLM: %v", org.Spec.OrganizationAlias, err)
			} else {
				emitEvent(r.Recorder, org, corev1.EventTypeNormal, EventReasonDeleted,
					"Organization %q deleted from LiteLLM", org.Spec.OrganizationAlias)
			}
		}
	}
	controllerutil.RemoveFinalizer(org, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, org)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMOrganization{}).
		Named("litellmorganization").
		Complete(r)
}

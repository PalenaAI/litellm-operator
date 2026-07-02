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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMUserReconciler reconciles a LiteLLMUser object.
type LiteLLMUserReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers/finalizers,verbs=update

func (r *LiteLLMUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var user litellmv1alpha1.LiteLLMUser
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !user.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &user)
	}

	if !controllerutil.ContainsFinalizer(&user, FinalizerName) {
		controllerutil.AddFinalizer(&user, FinalizerName)
		if err := r.Update(ctx, &user); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, user.Namespace, user.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		emitEvent(r.Recorder, &user, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", user.Spec.InstanceRef.Name, err)
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "InstanceNotReady", Message: err.Error(),
		})
		_ = r.Status().Update(ctx, &user)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileUser(ctx, &user, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			emitEvent(r.Recorder, &user, corev1.EventTypeWarning, EventReasonEnterpriseRequired,
				"User %q requires a LiteLLM Enterprise license", user.Spec.UserEmail)
			meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: user.Generation,
			})
			_ = r.Status().Update(ctx, &user)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to reconcile user")
		emitEvent(r.Recorder, &user, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile user %q: %v", user.Spec.UserEmail, err)
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "SyncFailed", Message: err.Error(),
		})
	} else {
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionTrue, Reason: "Synced", Message: "User synced to LiteLLM",
		})
	}

	if statusErr := r.Status().Update(ctx, &user); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

func (r *LiteLLMUserReconciler) reconcileUser(
	ctx context.Context,
	user *litellmv1alpha1.LiteLLMUser,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))

	teamIDs, err := r.resolveTeamRefs(ctx, user)
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("resolve team refs: %w", err)
	}

	req := litellm.UserCreateRequest{
		UserID:           user.Spec.UserID,
		UserEmail:        user.Spec.UserEmail,
		UserRole:         user.Spec.UserRole,
		MaxBudget:        user.Spec.MaxBudget,
		BudgetDuration:   user.Spec.BudgetDuration,
		Models:           user.Spec.Models,
		Teams:            teamIDs,
		TPMLimit:         user.Spec.TPMLimit,
		RPMLimit:         user.Spec.RPMLimit,
		SoftBudget:       user.Spec.SoftBudget,
		ModelRPMLimit:    user.Spec.ModelRPMLimit,
		ModelTPMLimit:    user.Spec.ModelTPMLimit,
		ObjectPermission: mapObjectPermission(user.Spec.ObjectPermission),
		Metadata:         user.Spec.Metadata,
		Blocked:          user.Spec.Blocked,
	}

	if user.Status.LiteLLMUserID == "" {
		resp, err := apiClient.Users().Create(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create user: %w", err)
		}
		user.Status.LiteLLMUserID = resp.UserID
		user.Status.Synced = true
		if err := r.Status().Update(ctx, user); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if user.Annotations == nil {
			user.Annotations = map[string]string{}
		}
		user.Annotations[AnnotationSyncHash] = computeSpecHash(user.Spec)
		if err := r.Update(ctx, user); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created user", "userId", resp.UserID)
		emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonCreated,
			"User %q registered with LiteLLM (id=%s)", user.Spec.UserEmail, resp.UserID)
	} else {
		currentHash := computeSpecHash(user.Spec)
		if user.Annotations[AnnotationSyncHash] != currentHash {
			if err := apiClient.Users().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update user: %w", err)
			}
			if user.Annotations == nil {
				user.Annotations = map[string]string{}
			}
			user.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, user); err != nil {
				return ctrl.Result{}, err
			}
			user.Status.Synced = true
			log.Info("updated user", "userId", user.Status.LiteLLMUserID)
			emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonUpdated,
				"User %q updated in LiteLLM", user.Spec.UserEmail)
		}
	}

	info, err := apiClient.Users().Get(ctx, user.Status.LiteLLMUserID)
	if err == nil && info != nil {
		user.Status.CurrentSpend = info.Spend
	}

	now := metav1.Now()
	user.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMUserReconciler) resolveTeamRefs(
	ctx context.Context,
	user *litellmv1alpha1.LiteLLMUser,
) ([]litellm.UserTeam, error) {
	teams := make([]litellm.UserTeam, 0, len(user.Spec.Teams))
	for _, t := range user.Spec.Teams {
		teamID := t.TeamID
		if t.TeamRef != nil {
			var team litellmv1alpha1.LiteLLMTeam
			if err := r.Get(ctx, types.NamespacedName{
				Name: t.TeamRef.Name, Namespace: user.Namespace,
			}, &team); err != nil {
				return nil, fmt.Errorf("resolve team ref %q: %w", t.TeamRef.Name, err)
			}
			teamID = team.Status.LiteLLMTeamID
			if teamID == "" {
				return nil, fmt.Errorf("team %q not yet synced (no litellmTeamId)", t.TeamRef.Name)
			}
		}
		teams = append(teams, litellm.UserTeam{
			TeamID:          teamID,
			Role:            t.Role,
			MaxBudgetInTeam: t.MaxBudgetInTeam,
		})
	}
	return teams, nil
}

func (r *LiteLLMUserReconciler) handleDeletion(ctx context.Context, user *litellmv1alpha1.LiteLLMUser) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(user, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if user.Status.LiteLLMUserID != "" {
		resolved, err := resolveInstance(ctx, r.Client, user.Namespace, user.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))
			if err := apiClient.Users().Delete(ctx, user.Status.LiteLLMUserID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete user from LiteLLM")
				emitEvent(r.Recorder, user, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete user %q from LiteLLM: %v", user.Spec.UserEmail, err)
			} else {
				emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonDeleted,
					"User %q deleted from LiteLLM", user.Spec.UserEmail)
			}
		}
	}
	controllerutil.RemoveFinalizer(user, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, user)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMUser{}).
		Named("litellmuser").
		Complete(r)
}

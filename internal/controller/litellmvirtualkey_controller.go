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
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// LiteLLMVirtualKeyReconciler reconciles a LiteLLMVirtualKey object.
type LiteLLMVirtualKeyReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmvirtualkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmvirtualkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmvirtualkeys/finalizers,verbs=update

func (r *LiteLLMVirtualKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var vk litellmv1alpha1.LiteLLMVirtualKey
	if err := r.Get(ctx, req.NamespacedName, &vk); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !vk.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &vk)
	}

	if !controllerutil.ContainsFinalizer(&vk, FinalizerName) {
		controllerutil.AddFinalizer(&vk, FinalizerName)
		if err := r.Update(ctx, &vk); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, vk.Namespace, vk.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		emitEvent(r.Recorder, &vk, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", vk.Spec.InstanceRef.Name, err)
		meta.SetStatusCondition(&vk.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "InstanceNotReady", Message: err.Error(),
		})
		_ = r.Status().Update(ctx, &vk)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileKey(ctx, &vk, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			emitEvent(r.Recorder, &vk, corev1.EventTypeWarning, EventReasonEnterpriseRequired,
				"VirtualKey %q requires a LiteLLM Enterprise license", vk.Spec.KeyAlias)
			meta.SetStatusCondition(&vk.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: vk.Generation,
			})
			_ = r.Status().Update(ctx, &vk)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to reconcile virtual key")
		emitEvent(r.Recorder, &vk, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile virtual key %q: %v", vk.Spec.KeyAlias, err)
		meta.SetStatusCondition(&vk.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "SyncFailed", Message: err.Error(),
		})
	} else {
		meta.SetStatusCondition(&vk.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionTrue, Reason: "Synced", Message: "Virtual key synced to LiteLLM",
		})
	}

	if statusErr := r.Status().Update(ctx, &vk); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

func (r *LiteLLMVirtualKeyReconciler) reconcileKey(
	ctx context.Context,
	vk *litellmv1alpha1.LiteLLMVirtualKey,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))

	// Resolve team and user refs
	teamID, err := r.resolveTeamRef(ctx, vk)
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	userID, err := r.resolveUserRef(ctx, vk)
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	if vk.Status.LiteLLMKeyToken == "" {
		// Generate key
		req := litellm.KeyGenerateRequest{
			KeyAlias:            vk.Spec.KeyAlias,
			TeamID:              teamID,
			UserID:              userID,
			Models:              vk.Spec.Models,
			MaxBudget:           vk.Spec.MaxBudget,
			BudgetDuration:      vk.Spec.BudgetDuration,
			ExpiresAt:           vk.Spec.ExpiresAt,
			TPMLimit:            vk.Spec.TPMLimit,
			RPMLimit:            vk.Spec.RPMLimit,
			Metadata:            vk.Spec.Metadata,
			ModelMaxBudget:      parseModelMaxBudget(vk.Spec.ModelMaxBudget),
			MaxParallelRequests: vk.Spec.MaxParallelRequests,
			Guardrails:          vk.Spec.Guardrails,
			Blocked:             vk.Spec.Blocked,
			SoftBudget:          vk.Spec.SoftBudget,
			ModelRPMLimit:       vk.Spec.ModelRPMLimit,
			ModelTPMLimit:       vk.Spec.ModelTPMLimit,
			ObjectPermission:    mapObjectPermission(vk.Spec.ObjectPermission),
		}

		resp, err := apiClient.Keys().Generate(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("generate key: %w", err)
		}

		// Store key in a Secret
		secretName := vk.Spec.KeySecretName
		if secretName == "" {
			secretName = vk.Name + "-key"
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: vk.Namespace,
				Labels: map[string]string{
					LabelInstanceName: vk.Spec.InstanceRef.Name,
					LabelResourceType: "virtual-key",
				},
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"api_key": resp.Key,
			},
		}

		if err := controllerutil.SetControllerReference(vk, secret, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("set owner ref on secret: %w", err)
		}

		if err := r.Create(ctx, secret); err != nil {
			if apierrors.IsAlreadyExists(err) {
				var existing corev1.Secret
				if getErr := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: vk.Namespace}, &existing); getErr != nil {
					return ctrl.Result{}, getErr
				}
				existing.StringData = secret.StringData
				if updateErr := r.Update(ctx, &existing); updateErr != nil {
					return ctrl.Result{}, fmt.Errorf("update key secret: %w", updateErr)
				}
			} else {
				return ctrl.Result{}, fmt.Errorf("create key secret: %w", err)
			}
		}

		vk.Status.LiteLLMKeyToken = resp.Token
		vk.Status.KeySecretRef = &litellmv1alpha1.SecretKeyRef{Name: secretName, Key: "api_key"}
		vk.Status.IsActive = true
		vk.Status.Synced = true
		if err := r.Status().Update(ctx, vk); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after generate: %w", err)
		}
		if vk.Annotations == nil {
			vk.Annotations = map[string]string{}
		}
		vk.Annotations[AnnotationSyncHash] = computeSpecHash(vk.Spec)
		if err := r.Update(ctx, vk); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("generated virtual key", "alias", vk.Spec.KeyAlias, "secret", secretName)
		emitEvent(r.Recorder, vk, corev1.EventTypeNormal, EventReasonCreated,
			"Virtual key %q generated, stored in Secret %q", vk.Spec.KeyAlias, secretName)
	} else {
		// Update key if spec changed
		currentHash := computeSpecHash(vk.Spec)
		if vk.Annotations[AnnotationSyncHash] != currentHash {
			req := litellm.KeyUpdateRequest{
				Token:               vk.Status.LiteLLMKeyToken,
				Models:              vk.Spec.Models,
				MaxBudget:           vk.Spec.MaxBudget,
				BudgetDuration:      vk.Spec.BudgetDuration,
				TPMLimit:            vk.Spec.TPMLimit,
				RPMLimit:            vk.Spec.RPMLimit,
				Metadata:            vk.Spec.Metadata,
				ModelMaxBudget:      parseModelMaxBudget(vk.Spec.ModelMaxBudget),
				MaxParallelRequests: vk.Spec.MaxParallelRequests,
				Guardrails:          vk.Spec.Guardrails,
				Blocked:             vk.Spec.Blocked,
				SoftBudget:          vk.Spec.SoftBudget,
				ModelRPMLimit:       vk.Spec.ModelRPMLimit,
				ModelTPMLimit:       vk.Spec.ModelTPMLimit,
				ObjectPermission:    mapObjectPermission(vk.Spec.ObjectPermission),
			}
			if err := apiClient.Keys().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update key: %w", err)
			}
			if vk.Annotations == nil {
				vk.Annotations = map[string]string{}
			}
			vk.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, vk); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("updated virtual key", "alias", vk.Spec.KeyAlias)
			emitEvent(r.Recorder, vk, corev1.EventTypeNormal, EventReasonUpdated,
				"Virtual key %q updated in LiteLLM", vk.Spec.KeyAlias)
		}

		// Refresh spend info
		info, err := apiClient.Keys().Get(ctx, vk.Status.LiteLLMKeyToken)
		if err == nil && info != nil {
			vk.Status.CurrentSpend = info.Spend
			vk.Status.IsActive = info.IsActive
		}
	}

	now := metav1.Now()
	vk.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMVirtualKeyReconciler) resolveTeamRef(ctx context.Context, vk *litellmv1alpha1.LiteLLMVirtualKey) (string, error) {
	if vk.Spec.TeamRef == nil {
		return "", nil
	}
	var team litellmv1alpha1.LiteLLMTeam
	if err := r.Get(ctx, types.NamespacedName{Name: vk.Spec.TeamRef.Name, Namespace: vk.Namespace}, &team); err != nil {
		return "", fmt.Errorf("resolve team ref %q: %w", vk.Spec.TeamRef.Name, err)
	}
	if team.Status.LiteLLMTeamID == "" {
		return "", fmt.Errorf("team %q not yet synced", vk.Spec.TeamRef.Name)
	}
	return team.Status.LiteLLMTeamID, nil
}

func (r *LiteLLMVirtualKeyReconciler) resolveUserRef(ctx context.Context, vk *litellmv1alpha1.LiteLLMVirtualKey) (string, error) {
	if vk.Spec.UserRef == nil {
		return "", nil
	}
	var user litellmv1alpha1.LiteLLMUser
	if err := r.Get(ctx, types.NamespacedName{Name: vk.Spec.UserRef.Name, Namespace: vk.Namespace}, &user); err != nil {
		return "", fmt.Errorf("resolve user ref %q: %w", vk.Spec.UserRef.Name, err)
	}
	if user.Status.LiteLLMUserID == "" {
		return "", fmt.Errorf("user %q not yet synced", vk.Spec.UserRef.Name)
	}
	return user.Status.LiteLLMUserID, nil
}

func (r *LiteLLMVirtualKeyReconciler) handleDeletion(ctx context.Context, vk *litellmv1alpha1.LiteLLMVirtualKey) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(vk, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if vk.Status.LiteLLMKeyToken != "" {
		resolved, err := resolveInstance(ctx, r.Client, vk.Namespace, vk.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))
			if err := apiClient.Keys().Delete(ctx, vk.Status.LiteLLMKeyToken); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete key from LiteLLM")
				emitEvent(r.Recorder, vk, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete virtual key %q from LiteLLM: %v", vk.Spec.KeyAlias, err)
			} else {
				emitEvent(r.Recorder, vk, corev1.EventTypeNormal, EventReasonDeleted,
					"Virtual key %q deleted from LiteLLM", vk.Spec.KeyAlias)
			}
		}
	}
	controllerutil.RemoveFinalizer(vk, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, vk)
}

// parseModelMaxBudget converts CRD string-valued budget map to float64 for the API.
func parseModelMaxBudget(budgets map[string]string) map[string]float64 {
	if len(budgets) == 0 {
		return nil
	}
	result := make(map[string]float64, len(budgets))
	for model, val := range budgets {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			result[model] = f
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMVirtualKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMVirtualKey{}).
		Owns(&corev1.Secret{}).
		Named("litellmvirtualkey").
		Complete(r)
}

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
	"errors"
	"fmt"
	"maps"
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
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	// APIReader reads straight from the API server, bypassing the informer cache.
	// Used only to confirm that the key Secret is really gone before rotating the
	// key, since acting on a stale cached read would be destructive. Defaulted
	// from the manager in SetupWithManager.
	APIReader            client.Reader
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
			// Requeue so the key mints itself once the license lands (see the const
			// doc); otherwise the VirtualKey — and anything waiting on its minted key
			// Secret, e.g. an auto-wired ChatUI — stays stuck until it is recreated.
			return ctrl.Result{RequeueAfter: enterpriseLicenseRetryInterval}, nil
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

	// The Secret holds the only copy of the key material — LiteLLM stores just a
	// hash — so once it is gone the key is unusable no matter what the API says.
	// Drop the orphaned key and fall through to minting a replacement.
	if vk.Status.LiteLLMKeyToken != "" {
		exists, err := r.keySecretExists(ctx, vk)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("check key secret: %w", err)
		}
		if !exists {
			log.Info("key Secret is missing, rotating virtual key",
				"alias", vk.Spec.KeyAlias, "secret", keySecretName(vk))
			emitEvent(r.Recorder, vk, corev1.EventTypeWarning, EventReasonKeySecretMissing,
				"Secret %q holding the key for %q is missing; rotating the key to recreate it",
				keySecretName(vk), vk.Spec.KeyAlias)
			if delErr := apiClient.Keys().Delete(ctx, vk.Status.LiteLLMKeyToken); delErr != nil {
				// Best effort: nobody holds the old key material, so it is already
				// unusable. A failed cleanup must not block minting the replacement.
				log.Error(delErr, "failed to delete orphaned virtual key", "alias", vk.Spec.KeyAlias)
			}
			vk.Status.LiteLLMKeyToken = ""
			vk.Status.KeySecretRef = nil
			vk.Status.Synced = false
		}
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
		secretName := keySecretName(vk)
		if err := r.reconcileKeySecret(ctx, vk, resp.Key); err != nil {
			return ctrl.Result{}, err
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
		vk.Annotations[AnnotationSyncHash] = keySyncHash(vk.Spec)
		if err := r.Update(ctx, vk); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("generated virtual key", "alias", vk.Spec.KeyAlias, "secret", secretName)
		emitEvent(r.Recorder, vk, corev1.EventTypeNormal, EventReasonCreated,
			"Virtual key %q generated, stored in Secret %q", vk.Spec.KeyAlias, secretName)
	} else {
		// Update key if spec changed
		currentHash := keySyncHash(vk.Spec)
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

		// Reconcile the key Secret's metadata on every pass, not just when the
		// LiteLLM-facing spec changes: spec.keySecretTemplate is deliberately
		// excluded from the sync hash, so an edit to it moves no hash.
		if err := r.reconcileKeySecret(ctx, vk, ""); err != nil {
			if errors.Is(err, errKeySecretGone) {
				// Raced with a delete between keySecretExists and here; the next
				// pass takes the rotation path above.
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
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

// errKeySecretGone reports that the Secret holding the generated API key no
// longer exists. LiteLLM stores only a hash of the key, so the material cannot be
// re-read from the API and the key has to be rotated to restore service.
var errKeySecretGone = errors.New("key secret no longer exists")

// keySyncHash hashes the parts of the spec that are pushed to LiteLLM. The
// Kubernetes-side Secret plumbing is excluded so that editing Secret metadata
// does not trigger a pointless POST /key/update.
func keySyncHash(spec litellmv1alpha1.LiteLLMVirtualKeySpec) string {
	spec.KeySecretName = ""
	spec.KeySecretTemplate = nil
	return computeSpecHash(spec)
}

// keySecretName returns the Secret the operator manages for this key. Once a key
// has been minted the name is pinned by status.keySecretRef, so that editing
// spec.keySecretName cannot orphan the only copy of the key material.
func keySecretName(vk *litellmv1alpha1.LiteLLMVirtualKey) string {
	if vk.Status.KeySecretRef != nil && vk.Status.KeySecretRef.Name != "" {
		return vk.Status.KeySecretRef.Name
	}
	if vk.Spec.KeySecretName != "" {
		return vk.Spec.KeySecretName
	}
	return vk.Name + "-key"
}

// keySecretExists reports whether the key Secret is present, reading straight
// from the API server. The informer cache can lag behind a delete, and rotating a
// working key on a stale read would destroy it for every consumer.
func (r *LiteLLMVirtualKeyReconciler) keySecretExists(ctx context.Context, vk *litellmv1alpha1.LiteLLMVirtualKey) (bool, error) {
	reader := client.Reader(r.Client)
	if r.APIReader != nil {
		reader = r.APIReader
	}
	var secret corev1.Secret
	err := reader.Get(ctx, types.NamespacedName{Name: keySecretName(vk), Namespace: vk.Namespace}, &secret)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// reconcileKeySecret ensures the Secret holding the generated API key exists and
// carries the labels and annotations declared on spec.keySecretTemplate. apiKey is
// the freshly minted key material, or "" on a pass where no new key was generated
// — in that case a missing Secret is reported as errKeySecretGone rather than
// created without a key.
func (r *LiteLLMVirtualKeyReconciler) reconcileKeySecret(
	ctx context.Context,
	vk *litellmv1alpha1.LiteLLMVirtualKey,
	apiKey string,
) error {
	name := keySecretName(vk)

	// The operator's own labels win over the template's on conflict, so a template
	// cannot break the selectors the controller relies on.
	desiredLabels := map[string]string{
		LabelInstanceName: vk.Spec.InstanceRef.Name,
		LabelResourceType: "virtual-key",
	}
	var desiredAnnotations map[string]string
	if tmpl := vk.Spec.KeySecretTemplate; tmpl != nil {
		desiredLabels = mergeStringMaps(tmpl.Labels, desiredLabels)
		desiredAnnotations = tmpl.Annotations
	}

	var existing corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: vk.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		if apiKey == "" {
			return errKeySecretGone
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   vk.Namespace,
				Labels:      desiredLabels,
				Annotations: desiredAnnotations,
			},
			Type:       corev1.SecretTypeOpaque,
			StringData: map[string]string{"api_key": apiKey},
		}
		if err := controllerutil.SetControllerReference(vk, secret, r.Scheme); err != nil {
			return fmt.Errorf("set owner ref on key secret: %w", err)
		}
		if err := r.Create(ctx, secret); err != nil {
			return fmt.Errorf("create key secret: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get key secret: %w", err)
	}

	changed := false
	// Adopt a Secret that a user pre-created by hand, so it is still garbage
	// collected with the VirtualKey. A Secret owned by some other controller
	// surfaces as an error rather than being hijacked.
	if !metav1.IsControlledBy(&existing, vk) {
		if err := controllerutil.SetControllerReference(vk, &existing, r.Scheme); err != nil {
			return fmt.Errorf("set owner ref on key secret: %w", err)
		}
		changed = true
	}
	if merged := mergeStringMaps(existing.Labels, desiredLabels); !maps.Equal(existing.Labels, merged) {
		existing.Labels = merged
		changed = true
	}
	if merged := mergeStringMaps(existing.Annotations, desiredAnnotations); !maps.Equal(existing.Annotations, merged) {
		existing.Annotations = merged
		changed = true
	}
	if apiKey != "" {
		existing.StringData = map[string]string{"api_key": apiKey}
		changed = true
	}
	if !changed {
		return nil
	}
	return r.Update(ctx, &existing)
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
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMVirtualKey{}).
		Owns(&corev1.Secret{}).
		Named("litellmvirtualkey").
		Complete(r)
}

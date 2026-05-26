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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMCredentialReconciler reconciles a LiteLLMCredential object.
//
// Credentials are registered against the LiteLLM proxy's /credentials API
// (DB-backed). These are visible in the Admin UI and merged into models
// referenced via `litellm_credential_name` at request time. Rotation of the
// referenced Kubernetes Secret triggers an immediate PATCH /credentials/{name}
// via the Secret watch.
type LiteLLMCredentialReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcredentials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcredentials/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcredentials/finalizers,verbs=update
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmmodels,verbs=get;list;watch

func (r *LiteLLMCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cred litellmv1alpha1.LiteLLMCredential
	if err := r.Get(ctx, req.NamespacedName, &cred); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cred.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &cred)
	}

	if !controllerutil.ContainsFinalizer(&cred, FinalizerName) {
		controllerutil.AddFinalizer(&cred, FinalizerName)
		if err := r.Update(ctx, &cred); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, cred.Namespace, cred.Spec.InstanceRef)
	if err != nil {
		reason := "InstanceNotReady"
		if apierrors.IsNotFound(errors.Unwrap(err)) {
			reason = "InstanceNotFound"
		}
		emitEvent(r.Recorder, &cred, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Credential %q: %v", cred.Spec.CredentialName, err)
		r.setStatus(ctx, &cred, false, reason, err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	apiKey, err := r.fetchAPIKey(ctx, &cred)
	if err != nil {
		reason := "SecretFetchFailed"
		if apierrors.IsNotFound(errors.Unwrap(err)) {
			reason = "SecretNotFound"
		}
		if errors.Is(err, errSecretKeyMissing) {
			reason = "SecretKeyMissing"
		}
		emitEvent(r.Recorder, &cred, corev1.EventTypeWarning, EventReasonSecretNotFound,
			"Credential %q: %v", cred.Spec.CredentialName, err)
		r.setStatus(ctx, &cred, false, reason, err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	payload := buildCredentialPayload(&cred, apiKey)
	currentHash := computeSpecHash(struct {
		Spec    litellmv1alpha1.LiteLLMCredentialSpec
		APIKey  string
		Payload litellm.CredentialPayload
	}{cred.Spec, apiKey, payload})

	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)
	desiredAction := r.planAction(&cred, currentHash)

	switch desiredAction {
	case actionCreate:
		if err := apiClient.Credentials().Create(ctx, payload); err != nil {
			// Treat 4xx conflict (already exists) as an update — covers the
			// case where we crashed after creating in LiteLLM but before
			// persisting the sync-hash.
			if apiErr, ok := litellm.IsAPIError(err); ok && apiErr.StatusCode == 400 {
				if uerr := apiClient.Credentials().Update(ctx, payload); uerr != nil {
					return r.reportAPIError(ctx, &cred, "create credential (fallback to update)", uerr)
				}
			} else {
				return r.reportAPIError(ctx, &cred, "create credential", err)
			}
		}
		log.Info("created credential", "name", cred.Spec.CredentialName)
		emitEvent(r.Recorder, &cred, corev1.EventTypeNormal, EventReasonCreated,
			"Credential %q registered with LiteLLM", cred.Spec.CredentialName)

	case actionUpdate:
		if err := apiClient.Credentials().Update(ctx, payload); err != nil {
			// If the credential was deleted out-of-band, recreate it.
			if apiErr, ok := litellm.IsAPIError(err); ok && apiErr.IsNotFound() {
				if cerr := apiClient.Credentials().Create(ctx, payload); cerr != nil {
					return r.reportAPIError(ctx, &cred, "recreate credential after 404", cerr)
				}
			} else {
				return r.reportAPIError(ctx, &cred, "update credential", err)
			}
		}
		log.Info("updated credential", "name", cred.Spec.CredentialName)
		emitEvent(r.Recorder, &cred, corev1.EventTypeNormal, EventReasonUpdated,
			"Credential %q updated in LiteLLM", cred.Spec.CredentialName)

	case actionNoop:
		// hash matches; nothing to push.
	}

	// Persist sync-hash on the CR (annotation, not status) — same pattern
	// model/team/user controllers use.
	if cred.Annotations == nil {
		cred.Annotations = map[string]string{}
	}
	if cred.Annotations[AnnotationSyncHash] != currentHash {
		cred.Annotations[AnnotationSyncHash] = currentHash
		if err := r.Update(ctx, &cred); err != nil {
			return ctrl.Result{}, err
		}
	}

	refCount, _ := r.countReferencingModels(ctx, &cred)
	cred.Status.ReferencedByModels = refCount
	r.setStatus(ctx, &cred, true, "Synced", "Credential registered with LiteLLM")

	// Periodic resync as a safety net for missed Secret-watch events.
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

type credentialAction int

const (
	actionNoop credentialAction = iota
	actionCreate
	actionUpdate
)

// planAction decides what to push to LiteLLM based on local state.
func (r *LiteLLMCredentialReconciler) planAction(cred *litellmv1alpha1.LiteLLMCredential, currentHash string) credentialAction {
	if !cred.Status.Configured {
		return actionCreate
	}
	if cred.Annotations[AnnotationSyncHash] != currentHash {
		return actionUpdate
	}
	return actionNoop
}

var errSecretKeyMissing = errors.New("key not found in Secret")

func (r *LiteLLMCredentialReconciler) fetchAPIKey(ctx context.Context, cred *litellmv1alpha1.LiteLLMCredential) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      cred.Spec.APIKeySecretRef.Name,
		Namespace: cred.Namespace,
	}, &secret); err != nil {
		return "", fmt.Errorf("get Secret %q: %w", cred.Spec.APIKeySecretRef.Name, err)
	}
	val, ok := secret.Data[cred.Spec.APIKeySecretRef.Key]
	if !ok {
		return "", fmt.Errorf("Secret %q: %w: %s",
			cred.Spec.APIKeySecretRef.Name, errSecretKeyMissing, cred.Spec.APIKeySecretRef.Key)
	}
	return string(val), nil
}

// buildCredentialPayload constructs the body sent to POST/PATCH /credentials.
// credential_values holds the provider auth (api_key, api_base, api_version,
// plus any free-form params). credential_info is reserved for metadata.
func buildCredentialPayload(cred *litellmv1alpha1.LiteLLMCredential, apiKey string) litellm.CredentialPayload {
	values := map[string]interface{}{
		"api_key": apiKey,
	}
	if cred.Spec.APIBase != "" {
		values["api_base"] = cred.Spec.APIBase
	}
	if cred.Spec.APIVersion != "" {
		values["api_version"] = cred.Spec.APIVersion
	}
	for k, v := range cred.Spec.Params {
		if _, reserved := values[k]; reserved {
			continue
		}
		values[k] = v
	}
	return litellm.CredentialPayload{
		CredentialName:   cred.Spec.CredentialName,
		CredentialValues: values,
		CredentialInfo: map[string]interface{}{
			"managed_by": "litellm-operator",
		},
	}
}

func (r *LiteLLMCredentialReconciler) setStatus(ctx context.Context, cred *litellmv1alpha1.LiteLLMCredential, configured bool, reason, message string) {
	cred.Status.Configured = configured
	now := metav1.Now()
	cred.Status.LastSyncTime = &now
	status := metav1.ConditionFalse
	if configured {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cred.Generation,
	})
	if err := r.Status().Update(ctx, cred); err != nil {
		logf.FromContext(ctx).Error(err, "failed to update credential status")
	}
}

func (r *LiteLLMCredentialReconciler) reportAPIError(ctx context.Context, cred *litellmv1alpha1.LiteLLMCredential, op string, err error) (ctrl.Result, error) {
	emitEvent(r.Recorder, cred, corev1.EventTypeWarning, EventReasonReconcileFailed,
		"Credential %q %s: %v", cred.Spec.CredentialName, op, err)
	r.setStatus(ctx, cred, false, "APIError", fmt.Sprintf("%s: %v", op, err))
	if apiErr, ok := litellm.IsAPIError(err); ok && !apiErr.IsTransient() {
		// Permanent error: don't requeue immediately; wait for next periodic
		// resync or the next spec/Secret change.
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *LiteLLMCredentialReconciler) countReferencingModels(ctx context.Context, cred *litellmv1alpha1.LiteLLMCredential) (int, error) {
	var models litellmv1alpha1.LiteLLMModelList
	if err := r.List(ctx, &models, client.InNamespace(cred.Namespace)); err != nil {
		return 0, err
	}
	count := 0
	for _, m := range models.Items {
		if m.Spec.LiteLLMParams.CredentialRef != nil && m.Spec.LiteLLMParams.CredentialRef.Name == cred.Name {
			count++
		}
	}
	return count, nil
}

func (r *LiteLLMCredentialReconciler) handleDeletion(ctx context.Context, cred *litellmv1alpha1.LiteLLMCredential) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cred, FinalizerName) {
		return ctrl.Result{}, nil
	}
	log := logf.FromContext(ctx)

	// Best-effort delete on LiteLLM. If the instance is unreachable we still
	// remove the finalizer — orphaning a DB credential is preferable to
	// blocking CR deletion forever, and consistent with how the other API-
	// backed controllers handle this case.
	if cred.Status.Configured {
		resolved, err := resolveInstance(ctx, r.Client, cred.Namespace, cred.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)
			if derr := apiClient.Credentials().Delete(ctx, cred.Spec.CredentialName); derr != nil {
				if apiErr, ok := litellm.IsAPIError(derr); !ok || !apiErr.IsNotFound() {
					log.Error(derr, "failed to delete credential from LiteLLM (proceeding with finalizer removal)",
						"name", cred.Spec.CredentialName)
				}
			}
		} else {
			log.Info("instance not reachable during deletion; removing finalizer anyway",
				"name", cred.Spec.CredentialName, "error", err.Error())
		}
	}

	controllerutil.RemoveFinalizer(cred, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, cred)
}

// findCredentialsForSecret enqueues every LiteLLMCredential in the namespace
// of the changed Secret whose apiKeySecretRef points at it. This is what gives
// us near-real-time key rotation.
func (r *LiteLLMCredentialReconciler) findCredentialsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	var creds litellmv1alpha1.LiteLLMCredentialList
	if err := r.List(ctx, &creds, client.InNamespace(secret.Namespace)); err != nil {
		return nil
	}
	var out []reconcile.Request
	for _, c := range creds.Items {
		if c.Spec.APIKeySecretRef.Name == secret.Name {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      c.Name,
				Namespace: c.Namespace,
			}})
		}
	}
	return out
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMCredential{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.findCredentialsForSecret)).
		Named("litellmcredential").
		Complete(r)
}

/*
Copyright 2026.

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
)

// LiteLLMCredentialReconciler reconciles a LiteLLMCredential object.
//
// Credentials are config-level resources: the actual credential_list entries
// are materialized by the LiteLLMInstance controller when it rebuilds the
// ConfigMap + Deployment. This controller's job is to validate the CR,
// surface status conditions, and count models that reference it. Instance
// reconciliation is triggered via a Watch on LiteLLMCredential from the
// instance controller, so we do not need to poke it from here.
type LiteLLMCredentialReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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

	// Validate the referenced instance exists (but does not have to be Ready;
	// credentials are config-level so they can be defined before the instance
	// rollout completes).
	var instance litellmv1alpha1.LiteLLMInstance
	err := r.Get(ctx, types.NamespacedName{Name: cred.Spec.InstanceRef.Name, Namespace: cred.Namespace}, &instance)
	if err != nil {
		reason := "InstanceFetchFailed"
		if apierrors.IsNotFound(err) {
			reason = "InstanceNotFound"
		}
		emitEvent(r.Recorder, &cred, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Credential %q: %s: %v", cred.Spec.CredentialName, reason, err)
		meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            err.Error(),
			ObservedGeneration: cred.Generation,
		})
		cred.Status.Configured = false
		_ = r.Status().Update(ctx, &cred)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Validate the API key Secret exists and has the expected key.
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      cred.Spec.APIKeySecretRef.Name,
		Namespace: cred.Namespace,
	}, &secret); err != nil {
		reason := "SecretFetchFailed"
		if apierrors.IsNotFound(err) {
			reason = "SecretNotFound"
		}
		emitEvent(r.Recorder, &cred, corev1.EventTypeWarning, EventReasonSecretNotFound,
			"Credential %q API key Secret %q: %v", cred.Spec.CredentialName, cred.Spec.APIKeySecretRef.Name, err)
		meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            fmt.Sprintf("API key Secret %q: %s", cred.Spec.APIKeySecretRef.Name, err.Error()),
			ObservedGeneration: cred.Generation,
		})
		cred.Status.Configured = false
		_ = r.Status().Update(ctx, &cred)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if _, ok := secret.Data[cred.Spec.APIKeySecretRef.Key]; !ok {
		emitEvent(r.Recorder, &cred, corev1.EventTypeWarning, EventReasonValidationFailed,
			"Credential %q: key %q not found in Secret %q", cred.Spec.CredentialName, cred.Spec.APIKeySecretRef.Key, cred.Spec.APIKeySecretRef.Name)
		meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "SecretKeyMissing",
			Message:            fmt.Sprintf("key %q not found in Secret %q", cred.Spec.APIKeySecretRef.Key, cred.Spec.APIKeySecretRef.Name),
			ObservedGeneration: cred.Generation,
		})
		cred.Status.Configured = false
		_ = r.Status().Update(ctx, &cred)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Count LiteLLMModel CRs in the same namespace that reference this credential.
	refCount, err := r.countReferencingModels(ctx, &cred)
	if err != nil {
		log.V(1).Info("failed to count referencing models", "error", err)
	} else {
		cred.Status.ReferencedByModels = refCount
	}

	wasReady := meta.IsStatusConditionTrue(cred.Status.Conditions, ConditionReady)
	cred.Status.Configured = true
	now := metav1.Now()
	cred.Status.LastSyncTime = &now
	meta.SetStatusCondition(&cred.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Validated",
		Message:            "Credential is valid; referenced Secret exists",
		ObservedGeneration: cred.Generation,
	})
	if !wasReady {
		emitEvent(r.Recorder, &cred, corev1.EventTypeNormal, EventReasonSynced,
			"Credential %q validated (referenced by %d model(s))", cred.Spec.CredentialName, cred.Status.ReferencedByModels)
	}

	if err := r.Status().Update(ctx, &cred); err != nil {
		log.Error(err, "failed to update credential status")
	}

	// Requeue periodically to refresh referenced-by count and re-validate the Secret.
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
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
	// Nothing to clean up remotely — credential_list entries live in the
	// instance's ConfigMap, which the instance controller rewrites when this
	// CR is removed (it observes the deletion via its Watch).
	controllerutil.RemoveFinalizer(cred, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, cred)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMCredential{}).
		Named("litellmcredential").
		Complete(r)
}

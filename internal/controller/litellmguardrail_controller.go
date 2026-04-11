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

// LiteLLMGuardrailReconciler reconciles a LiteLLMGuardrail object.
//
// Guardrails are config-level resources: the actual `guardrails` config
// entries are materialized by the LiteLLMInstance controller when it
// rebuilds the ConfigMap + Deployment. This controller's job is to
// validate the spec, surface status conditions, and verify the referenced
// API key Secret exists. Instance reconciliation is triggered via a Watch
// on LiteLLMGuardrail from the instance controller, so we do not need to
// poke it from here.
type LiteLLMGuardrailReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmguardrails,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmguardrails/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmguardrails/finalizers,verbs=update

func (r *LiteLLMGuardrailReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var g litellmv1alpha1.LiteLLMGuardrail
	if err := r.Get(ctx, req.NamespacedName, &g); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !g.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &g)
	}

	if !controllerutil.ContainsFinalizer(&g, FinalizerName) {
		controllerutil.AddFinalizer(&g, FinalizerName)
		if err := r.Update(ctx, &g); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Validate the referenced instance exists. Like credentials, guardrails
	// are config-level so they don't require the instance to be Ready; the
	// instance controller will pick them up on its next reconcile.
	var instance litellmv1alpha1.LiteLLMInstance
	if err := r.Get(ctx, types.NamespacedName{Name: g.Spec.InstanceRef.Name, Namespace: g.Namespace}, &instance); err != nil {
		reason := "InstanceFetchFailed"
		if apierrors.IsNotFound(err) {
			reason = "InstanceNotFound"
		}
		emitEvent(r.Recorder, &g, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Guardrail %q: %s: %v", g.Spec.GuardrailName, reason, err)
		meta.SetStatusCondition(&g.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            err.Error(),
			ObservedGeneration: g.Generation,
		})
		g.Status.Configured = false
		_ = r.Status().Update(ctx, &g)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// If an API key is declared, validate the Secret exists and has the
	// requested key. Some providers (e.g. local presidio, custom_guardrail
	// pointing at an internal service) don't need one, so this is optional.
	if g.Spec.APIKeySecretRef != nil {
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{
			Name:      g.Spec.APIKeySecretRef.Name,
			Namespace: g.Namespace,
		}, &secret); err != nil {
			reason := "SecretFetchFailed"
			if apierrors.IsNotFound(err) {
				reason = "SecretNotFound"
			}
			emitEvent(r.Recorder, &g, corev1.EventTypeWarning, EventReasonSecretNotFound,
				"Guardrail %q API key Secret %q: %v", g.Spec.GuardrailName, g.Spec.APIKeySecretRef.Name, err)
			meta.SetStatusCondition(&g.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             reason,
				Message:            fmt.Sprintf("API key Secret %q: %s", g.Spec.APIKeySecretRef.Name, err.Error()),
				ObservedGeneration: g.Generation,
			})
			g.Status.Configured = false
			_ = r.Status().Update(ctx, &g)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if _, ok := secret.Data[g.Spec.APIKeySecretRef.Key]; !ok {
			emitEvent(r.Recorder, &g, corev1.EventTypeWarning, EventReasonValidationFailed,
				"Guardrail %q: key %q not found in Secret %q", g.Spec.GuardrailName, g.Spec.APIKeySecretRef.Key, g.Spec.APIKeySecretRef.Name)
			meta.SetStatusCondition(&g.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             "SecretKeyMissing",
				Message:            fmt.Sprintf("key %q not found in Secret %q", g.Spec.APIKeySecretRef.Key, g.Spec.APIKeySecretRef.Name),
				ObservedGeneration: g.Generation,
			})
			g.Status.Configured = false
			_ = r.Status().Update(ctx, &g)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	wasReady := meta.IsStatusConditionTrue(g.Status.Conditions, ConditionReady)
	g.Status.Configured = true
	now := metav1.Now()
	g.Status.LastSyncTime = &now
	meta.SetStatusCondition(&g.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Validated",
		Message:            "Guardrail spec is valid; waiting for instance controller to render config",
		ObservedGeneration: g.Generation,
	})
	if !wasReady {
		emitEvent(r.Recorder, &g, corev1.EventTypeNormal, EventReasonSynced,
			"Guardrail %q validated", g.Spec.GuardrailName)
	}

	if err := r.Status().Update(ctx, &g); err != nil {
		log.Error(err, "failed to update guardrail status")
	}

	// Requeue periodically to revalidate the referenced Secret.
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMGuardrailReconciler) handleDeletion(ctx context.Context, g *litellmv1alpha1.LiteLLMGuardrail) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(g, FinalizerName) {
		return ctrl.Result{}, nil
	}
	// Nothing to clean up remotely — the `guardrails` config entries live
	// in the instance's ConfigMap, which the instance controller rewrites
	// when this CR is removed (it observes the deletion via its Watch).
	controllerutil.RemoveFinalizer(g, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, g)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMGuardrailReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMGuardrail{}).
		Named("litellmguardrail").
		Complete(r)
}

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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMModelReconciler reconciles a LiteLLMModel object.
type LiteLLMModelReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmmodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmmodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmmodels/finalizers,verbs=update
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcredentials,verbs=get;list;watch

func (r *LiteLLMModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var model litellmv1alpha1.LiteLLMModel
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !model.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &model)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&model, FinalizerName) {
		controllerutil.AddFinalizer(&model, FinalizerName)
		if err := r.Update(ctx, &model); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resolve instance
	resolved, err := resolveInstance(ctx, r.Client, model.Namespace, model.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:    ConditionSynced,
			Status:  metav1.ConditionFalse,
			Reason:  "InstanceNotReady",
			Message: err.Error(),
		})
		_ = r.Status().Update(ctx, &model)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Reconcile model
	result, err := r.reconcileModel(ctx, &model, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: model.Generation,
			})
			_ = r.Status().Update(ctx, &model)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to reconcile model")
		meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:    ConditionSynced,
			Status:  metav1.ConditionFalse,
			Reason:  "SyncFailed",
			Message: err.Error(),
		})
	} else {
		meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:    ConditionSynced,
			Status:  metav1.ConditionTrue,
			Reason:  "Synced",
			Message: "Model synced to LiteLLM",
		})
	}

	if statusErr := r.Status().Update(ctx, &model); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}

	return result, err
}

func (r *LiteLLMModelReconciler) reconcileModel(
	ctx context.Context,
	model *litellmv1alpha1.LiteLLMModel,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)

	params := litellm.ModelParams{
		Model:         model.Spec.LiteLLMParams.Model,
		RPM:           model.Spec.LiteLLMParams.RPM,
		TPM:           model.Spec.LiteLLMParams.TPM,
		Timeout:       model.Spec.LiteLLMParams.Timeout,
		StreamTimeout: model.Spec.LiteLLMParams.StreamTimeout,
		MaxRetries:    model.Spec.LiteLLMParams.MaxRetries,
		Tags:          model.Spec.Tags,
	}

	// credentialRef takes precedence over inline apiBase/apiKeySecretRef.
	// When set, the model is registered against an entry in the proxy's
	// credential_list via litellm_credential_name — api_base/api_key come
	// from the LiteLLMCredential, not from this spec.
	if model.Spec.LiteLLMParams.CredentialRef != nil {
		credName, err := r.resolveCredentialName(ctx, model)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		params.LiteLLMCredentialName = credName
	} else {
		params.APIBase = model.Spec.LiteLLMParams.APIBase
		if model.Spec.LiteLLMParams.APIKeySecretRef != nil {
			apiKey, err := getSecretValue(ctx, r.Client, model.Namespace, model.Spec.LiteLLMParams.APIKeySecretRef)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("resolve API key: %w", err)
			}
			params.APIKey = apiKey
		}
	}

	req := litellm.ModelCreateRequest{
		ModelName:     model.Spec.ModelName,
		LiteLLMParams: params,
	}
	if model.Spec.ModelInfo != nil {
		req.ModelInfo = &litellm.ModelInfoReq{
			MaxTokens:          model.Spec.ModelInfo.MaxTokens,
			InputCostPerToken:  model.Spec.ModelInfo.InputCostPerToken,
			OutputCostPerToken: model.Spec.ModelInfo.OutputCostPerToken,
		}
	}

	if model.Status.LiteLLMModelID == "" {
		resp, err := apiClient.Models().Create(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create model: %w", err)
		}
		model.Status.LiteLLMModelID = resp.ModelID
		model.Status.Synced = true
		// Persist status first so re-queued reconciliations see the model ID
		if err := r.Status().Update(ctx, model); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if model.Annotations == nil {
			model.Annotations = map[string]string{}
		}
		model.Annotations[AnnotationSyncHash] = computeSpecHash(model.Spec)
		if err := r.Update(ctx, model); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created model", "modelId", resp.ModelID)
	} else {
		currentHash := computeSpecHash(model.Spec)
		if model.Annotations[AnnotationSyncHash] != currentHash {
			req.ModelID = model.Status.LiteLLMModelID
			if req.ModelInfo == nil {
				req.ModelInfo = &litellm.ModelInfoReq{}
			}
			req.ModelInfo.ID = model.Status.LiteLLMModelID
			if err := apiClient.Models().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update model: %w", err)
			}
			if model.Annotations == nil {
				model.Annotations = map[string]string{}
			}
			model.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, model); err != nil {
				return ctrl.Result{}, err
			}
			model.Status.Synced = true
			log.Info("updated model", "modelId", model.Status.LiteLLMModelID)
		}
	}

	now := metav1.Now()
	model.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// resolveCredentialName fetches the LiteLLMCredential referenced by the model's
// credentialRef and returns its spec.credentialName (the key that matches an
// entry in the proxy's credential_list). The credential must live in the same
// namespace as the model.
func (r *LiteLLMModelReconciler) resolveCredentialName(ctx context.Context, model *litellmv1alpha1.LiteLLMModel) (string, error) {
	var cred litellmv1alpha1.LiteLLMCredential
	key := client.ObjectKey{Name: model.Spec.LiteLLMParams.CredentialRef.Name, Namespace: model.Namespace}
	if err := r.Get(ctx, key, &cred); err != nil {
		return "", fmt.Errorf("resolve credentialRef %q: %w", model.Spec.LiteLLMParams.CredentialRef.Name, err)
	}
	if cred.Spec.CredentialName == "" {
		return "", fmt.Errorf("credential %q has empty credentialName", cred.Name)
	}
	return cred.Spec.CredentialName, nil
}

func (r *LiteLLMModelReconciler) handleDeletion(
	ctx context.Context,
	model *litellmv1alpha1.LiteLLMModel,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(model, FinalizerName) {
		return ctrl.Result{}, nil
	}

	if model.Status.LiteLLMModelID != "" {
		resolved, err := resolveInstance(ctx, r.Client, model.Namespace, model.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)
			if err := apiClient.Models().Delete(ctx, model.Status.LiteLLMModelID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete model from LiteLLM", "modelId", model.Status.LiteLLMModelID)
			}
		}
	}

	controllerutil.RemoveFinalizer(model, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, model)
}

// findModelsForCredential maps a LiteLLMCredential event to the LiteLLMModel
// CRs that reference it via credentialRef, so rename/params changes on a
// credential re-reconcile dependent models.
func (r *LiteLLMModelReconciler) findModelsForCredential(ctx context.Context, obj client.Object) []reconcile.Request {
	cred, ok := obj.(*litellmv1alpha1.LiteLLMCredential)
	if !ok {
		return nil
	}
	var models litellmv1alpha1.LiteLLMModelList
	if err := r.List(ctx, &models, client.InNamespace(cred.Namespace)); err != nil {
		return nil
	}
	var requests []reconcile.Request
	for _, m := range models.Items {
		if m.Spec.LiteLLMParams.CredentialRef != nil && m.Spec.LiteLLMParams.CredentialRef.Name == cred.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: m.Name, Namespace: m.Namespace},
			})
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMModel{}).
		Watches(
			&litellmv1alpha1.LiteLLMCredential{},
			handler.EnqueueRequestsFromMapFunc(r.findModelsForCredential),
		).
		Named("litellmmodel").
		Complete(r)
}

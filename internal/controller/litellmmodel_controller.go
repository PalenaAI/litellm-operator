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
	Recorder             record.EventRecorder
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
		emitEvent(r.Recorder, &model, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", model.Spec.InstanceRef.Name, err)
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
			emitEvent(r.Recorder, &model, corev1.EventTypeWarning, EventReasonEnterpriseRequired,
				"Model %q requires a LiteLLM Enterprise license", model.Spec.ModelName)
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: model.Generation,
			})
			_ = r.Status().Update(ctx, &model)
			return ctrl.Result{RequeueAfter: enterpriseLicenseRetryInterval}, nil
		}
		log.Error(err, "failed to reconcile model")
		emitEvent(r.Recorder, &model, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile model %q: %v", model.Spec.ModelName, err)
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
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))

	params := litellm.ModelParams{
		Model:          model.Spec.LiteLLMParams.Model,
		RPM:            model.Spec.LiteLLMParams.RPM,
		TPM:            model.Spec.LiteLLMParams.TPM,
		Timeout:        model.Spec.LiteLLMParams.Timeout,
		StreamTimeout:  model.Spec.LiteLLMParams.StreamTimeout,
		MaxRetries:     model.Spec.LiteLLMParams.MaxRetries,
		Tags:           model.Spec.Tags,
		Weight:         model.Spec.LiteLLMParams.Weight,
		Order:          model.Spec.LiteLLMParams.Order,
		MaxInputTokens: model.Spec.LiteLLMParams.MaxInputTokens,
		Temperature:    model.Spec.LiteLLMParams.Temperature,
		TopP:           model.Spec.LiteLLMParams.TopP,
		MaxTokens:      model.Spec.LiteLLMParams.MaxTokens,
		Seed:           model.Spec.LiteLLMParams.Seed,
		Organization:   model.Spec.LiteLLMParams.Organization,
		AWSRegionName:  model.Spec.LiteLLMParams.AWSRegionName,
		ExtraHeaders:   model.Spec.LiteLLMParams.ExtraHeaders,
		DropParams:     model.Spec.LiteLLMParams.DropParams,
		VertexProject:  model.Spec.LiteLLMParams.VertexProject,
		VertexLocation: model.Spec.LiteLLMParams.VertexLocation,
	}

	// credentialRef takes precedence over inline apiBase/apiVersion/apiKeySecretRef.
	//
	// We resolve the credential's values and write api_base/api_version/api_key
	// INLINE onto the model payload (in addition to litellm_credential_name)
	// rather than relying solely on request-time named-credential resolution.
	// LiteLLM hydrates a DB-stored model's litellm_credential_name at router
	// load time, and on a cold start that happens before DB-backed credentials
	// are loaded into the in-memory credential_list — leaving Azure models with
	// no api_base ("Must provide ... azure_endpoint"). Inline params are stored
	// on the deployment and always win over the named credential (LiteLLM only
	// fills fields left None), so this is restart-safe. The credential name is
	// still sent for Admin UI association and best-effort merge of extra params.
	authMode := authModeInline
	if model.Spec.LiteLLMParams.CredentialRef != nil {
		authMode = authModeCredential
		cred, err := r.resolveCredential(ctx, model)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		params.LiteLLMCredentialName = cred.name
		params.APIBase = cred.apiBase
		params.APIVersion = cred.apiVersion
		params.APIKey = cred.apiKey
	} else {
		params.APIBase = model.Spec.LiteLLMParams.APIBase
		params.APIVersion = model.Spec.LiteLLMParams.APIVersion
		if model.Spec.LiteLLMParams.APIKeySecretRef != nil {
			apiKey, err := getSecretValue(ctx, r.Client, model.Namespace, model.Spec.LiteLLMParams.APIKeySecretRef)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("resolve API key: %w", err)
			}
			params.APIKey = apiKey
		}
		if model.Spec.LiteLLMParams.VertexCredentialsSecretRef != nil {
			vc, err := getSecretValue(ctx, r.Client, model.Namespace, model.Spec.LiteLLMParams.VertexCredentialsSecretRef)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("resolve vertex credentials: %w", err)
			}
			params.VertexCredentials = vc
		}
	}

	req := litellm.ModelCreateRequest{
		ModelName:     model.Spec.ModelName,
		LiteLLMParams: params,
	}
	if model.Spec.ModelInfo != nil {
		info := model.Spec.ModelInfo
		req.ModelInfo = &litellm.ModelInfoReq{
			MaxTokens:                   info.MaxTokens,
			InputCostPerToken:           info.InputCostPerToken,
			OutputCostPerToken:          info.OutputCostPerToken,
			Mode:                        info.Mode,
			BaseModel:                   info.BaseModel,
			Tier:                        info.Tier,
			RegionName:                  info.RegionName,
			AccessGroups:                info.AccessGroups,
			SupportedEnvironments:       info.SupportedEnvironments,
			UseInPassThrough:            info.UseInPassThrough,
			InputCostPerPixel:           info.InputCostPerPixel,
			InputCostPerSecond:          info.InputCostPerSecond,
			CacheReadInputTokenCost:     info.CacheReadInputTokenCost,
			CacheCreationInputTokenCost: info.CacheCreationInputTokenCost,
		}
		if hc := info.HealthCheck; hc != nil {
			req.ModelInfo.DisableBackgroundHealthCheck = hc.DisableBackgroundHealthCheck
			req.ModelInfo.HealthCheckTimeout = hc.TimeoutSeconds
			req.ModelInfo.HealthCheckMaxTokens = hc.MaxTokens
			req.ModelInfo.HealthCheckMaxTokensReasoning = hc.MaxTokensReasoning
			req.ModelInfo.HealthCheckMaxTokensNonReasoning = hc.MaxTokensNonReasoning
			req.ModelInfo.HealthCheckReasoningEffort = hc.ReasoningEffort
			req.ModelInfo.HealthCheckVoice = hc.Voice
			req.ModelInfo.HealthCheckModel = hc.Model
		}
	}

	// The sync hash covers the resolved provider params (api_base/api_version/
	// api_key), not just model.Spec — so a Secret rotation or an edit to the
	// referenced LiteLLMCredential re-pushes even though model.Spec is
	// unchanged. (The credential watch already re-enqueues dependent models.)
	currentHash := computeSpecHash(struct {
		Spec   litellmv1alpha1.LiteLLMModelSpec
		Params litellm.ModelParams
	}{model.Spec, params})

	switch {
	case model.Status.LiteLLMModelID == "":
		resp, err := apiClient.Models().Create(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create model: %w", err)
		}
		if err := r.recordSync(ctx, model, resp.ModelID, currentHash, authMode); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created model", "modelId", resp.ModelID)
		emitEvent(r.Recorder, model, corev1.EventTypeNormal, EventReasonCreated,
			"Model %q registered with LiteLLM (id=%s)", model.Spec.ModelName, resp.ModelID)

	case model.Annotations[AnnotationSyncHash] == currentHash && model.Annotations[AnnotationAuthMode] == authMode:
		// In sync; nothing to push.

	case model.Annotations[AnnotationAuthMode] != "" && model.Annotations[AnnotationAuthMode] != authMode:
		// Auth mode flipped (credential <-> inline). /model/update merges and
		// cannot clear the provider fields left by the previous mode, so delete
		// and recreate to guarantee a clean record (no stale api_base / api_key
		// / litellm_credential_name).
		if err := apiClient.Models().Delete(ctx, model.Status.LiteLLMModelID); err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("delete model for auth-mode switch: %w", err)
		}
		resp, err := apiClient.Models().Create(ctx, req)
		if err != nil {
			// Clear the stale ID so the next reconcile recreates rather than
			// trying to update a model that no longer exists.
			model.Status.LiteLLMModelID = ""
			_ = r.Status().Update(ctx, model)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("recreate model for auth-mode switch: %w", err)
		}
		if err := r.recordSync(ctx, model, resp.ModelID, currentHash, authMode); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("recreated model after auth-mode switch", "modelId", resp.ModelID, "authMode", authMode)
		emitEvent(r.Recorder, model, corev1.EventTypeNormal, EventReasonUpdated,
			"Model %q recreated in LiteLLM after auth-mode switch to %q", model.Spec.ModelName, authMode)

	default:
		req.ModelID = model.Status.LiteLLMModelID
		if req.ModelInfo == nil {
			req.ModelInfo = &litellm.ModelInfoReq{}
		}
		req.ModelInfo.ID = model.Status.LiteLLMModelID
		if err := apiClient.Models().Update(ctx, req); err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update model: %w", err)
		}
		if err := r.recordSync(ctx, model, model.Status.LiteLLMModelID, currentHash, authMode); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("updated model", "modelId", model.Status.LiteLLMModelID)
		emitEvent(r.Recorder, model, corev1.EventTypeNormal, EventReasonUpdated,
			"Model %q updated in LiteLLM", model.Spec.ModelName)
	}

	// Poll /health to report operand health into model status — but ONLY when
	// background health checks are enabled on the instance. This is a cost
	// safeguard: with background_health_checks=true, GET /health returns the
	// cached background results (cheap). With it disabled (or unset), GET
	// /health live-probes every deployment on each call — a real, paid
	// completion per model — so polling it here would silently reintroduce the
	// exact cost the operator was asked to avoid. When checks are off we skip
	// the probe entirely and mark health "unknown". Failure is non-fatal.
	if backgroundHealthChecksEnabled(resolved.Instance) {
		r.updateModelHealth(ctx, apiClient, model)
	} else {
		model.Status.Health = healthStatusUnknown
	}

	now := metav1.Now()
	model.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// backgroundHealthChecksEnabled reports whether the instance has opted into
// health checking via spec.generalSettings.backgroundHealthChecks: true. Only
// then does GET /health serve cached (cheap) results; otherwise the operator
// must not call it, because /health live-probes every model (a paid completion
// per model) when background checks are off.
func backgroundHealthChecksEnabled(instance *litellmv1alpha1.LiteLLMInstance) bool {
	if instance == nil || instance.Spec.GeneralSettings == nil {
		return false
	}
	return instance.Spec.GeneralSettings.BackgroundHealthChecks != nil &&
		*instance.Spec.GeneralSettings.BackgroundHealthChecks
}

// updateModelHealth asks the LiteLLM proxy for its /health report and
// records whether this model shows up as healthy, unhealthy, or missing.
// It intentionally never returns an error so health probe flakes do not
// flip the Synced condition or trigger requeue storms.
func (r *LiteLLMModelReconciler) updateModelHealth(ctx context.Context, apiClient litellm.Client, model *litellmv1alpha1.LiteLLMModel) {
	log := logf.FromContext(ctx)
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	report, err := apiClient.Health().Check(probeCtx)
	if err != nil {
		log.V(1).Info("model health probe failed", "error", err)
		model.Status.Health = healthStatusUnknown
		return
	}
	for _, ep := range report.HealthyEndpoints {
		if ep.ModelID == model.Status.LiteLLMModelID || ep.Model == model.Spec.ModelName {
			model.Status.Health = "healthy"
			return
		}
	}
	for _, ep := range report.UnhealthyEndpoints {
		if ep.ModelID == model.Status.LiteLLMModelID || ep.Model == model.Spec.ModelName {
			model.Status.Health = "unhealthy"
			if ep.Error != "" {
				emitEvent(r.Recorder, model, corev1.EventTypeWarning, EventReasonHealthDegraded,
					"Model %q reported unhealthy by LiteLLM: %s", model.Spec.ModelName, ep.Error)
			}
			return
		}
	}
	// Not present in either list — treat as unknown rather than unhealthy
	// so a silently-dropped model doesn't get false-flagged.
	model.Status.Health = healthStatusUnknown
}

const (
	authModeInline     = "inline"
	authModeCredential = "credential"

	// healthStatusUnknown is the status.health value used when the operator has
	// no health signal for a model (not probed, or not present in /health).
	healthStatusUnknown = "unknown"
)

// resolvedCredential holds the LiteLLMCredential values the model controller
// writes inline onto the model payload.
type resolvedCredential struct {
	name       string
	apiBase    string
	apiVersion string
	apiKey     string
}

// resolveCredential fetches the LiteLLMCredential referenced by the model's
// credentialRef and resolves its name plus its api_base / api_version / api_key
// (reading the credential's Secret). These are written inline onto the model so
// LiteLLM does not depend on request-time named-credential resolution, which is
// unreliable for DB-stored models on a cold start. The credential must live in
// the same namespace as the model.
func (r *LiteLLMModelReconciler) resolveCredential(ctx context.Context, model *litellmv1alpha1.LiteLLMModel) (resolvedCredential, error) {
	var cred litellmv1alpha1.LiteLLMCredential
	key := client.ObjectKey{Name: model.Spec.LiteLLMParams.CredentialRef.Name, Namespace: model.Namespace}
	if err := r.Get(ctx, key, &cred); err != nil {
		return resolvedCredential{}, fmt.Errorf("resolve credentialRef %q: %w", model.Spec.LiteLLMParams.CredentialRef.Name, err)
	}
	if cred.Spec.CredentialName == "" {
		return resolvedCredential{}, fmt.Errorf("credential %q has empty credentialName", cred.Name)
	}
	apiKey, err := getSecretValue(ctx, r.Client, cred.Namespace, &cred.Spec.APIKeySecretRef)
	if err != nil {
		return resolvedCredential{}, fmt.Errorf("resolve credential %q API key: %w", cred.Name, err)
	}
	return resolvedCredential{
		name:       cred.Spec.CredentialName,
		apiBase:    cred.Spec.APIBase,
		apiVersion: cred.Spec.APIVersion,
		apiKey:     apiKey,
	}, nil
}

// recordSync persists the model ID, sync hash, and auth mode after a successful
// create/update/recreate. Status is written first so a re-queued reconcile sees
// the model ID before the annotations are committed.
func (r *LiteLLMModelReconciler) recordSync(ctx context.Context, model *litellmv1alpha1.LiteLLMModel, modelID, hash, authMode string) error {
	model.Status.LiteLLMModelID = modelID
	model.Status.Synced = true
	if err := r.Status().Update(ctx, model); err != nil {
		return fmt.Errorf("update status after sync: %w", err)
	}
	if model.Annotations == nil {
		model.Annotations = map[string]string{}
	}
	model.Annotations[AnnotationSyncHash] = hash
	model.Annotations[AnnotationAuthMode] = authMode
	if err := r.Update(ctx, model); err != nil {
		return err
	}
	return nil
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
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))
			if err := apiClient.Models().Delete(ctx, model.Status.LiteLLMModelID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete model from LiteLLM", "modelId", model.Status.LiteLLMModelID)
				emitEvent(r.Recorder, model, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete model %q from LiteLLM: %v", model.Spec.ModelName, err)
			} else {
				emitEvent(r.Recorder, model, corev1.EventTypeNormal, EventReasonDeleted,
					"Model %q deleted from LiteLLM", model.Spec.ModelName)
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

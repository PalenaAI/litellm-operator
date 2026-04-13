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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// ConfigSyncReconciler runs the bidirectional config sync loop.
// It watches LiteLLMInstance resources and, for each instance with
// configSync.enabled, periodically compares CRD state with the LiteLLM
// API state, detects drift, handles unmanaged resources, and reports status.
type ConfigSyncReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmmodels,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmmodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmteams,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmteams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmvirtualkeys,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmvirtualkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmorganizations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmorganizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcustomers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcustomers/status,verbs=get;update;patch

func (r *ConfigSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var instance litellmv1alpha1.LiteLLMInstance
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if config sync is not enabled.
	if instance.Spec.ConfigSync == nil || !instance.Spec.ConfigSync.Enabled {
		// Clear the ConfigSynced condition if it was previously set.
		if meta.FindStatusCondition(instance.Status.Conditions, ConditionConfigSynced) != nil {
			meta.RemoveStatusCondition(&instance.Status.Conditions, ConditionConfigSynced)
			instance.Status.ConfigSync = nil
			_ = r.Status().Update(ctx, &instance)
		}
		return ctrl.Result{}, nil
	}

	// Parse the sync interval.
	syncInterval, err := time.ParseDuration(instance.Spec.ConfigSync.Interval)
	if err != nil || syncInterval < 10*time.Second {
		syncInterval = 30 * time.Second
	}

	// Resolve instance endpoint and master key.
	if !instance.Status.Ready {
		log.V(1).Info("instance not ready, skipping config sync")
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionConfigSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "InstanceNotReady",
			Message:            "Waiting for instance to become ready",
			ObservedGeneration: instance.Generation,
		})
		_ = r.Status().Update(ctx, &instance)
		return ctrl.Result{RequeueAfter: syncInterval}, nil
	}

	masterKey, err := r.resolveMasterKey(ctx, &instance)
	if err != nil {
		log.Error(err, "failed to resolve master key for config sync")
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionConfigSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "MasterKeyError",
			Message:            err.Error(),
			ObservedGeneration: instance.Generation,
		})
		_ = r.Status().Update(ctx, &instance)
		return ctrl.Result{RequeueAfter: syncInterval}, nil
	}

	apiClient := r.LiteLLMClientFactory(instance.Status.Endpoint, masterKey)

	// Run the sync cycle.
	syncer := &configSyncer{
		kClient:   r.Client,
		apiClient: apiClient,
		recorder:  r.Recorder,
		instance:  &instance,
		config:    instance.Spec.ConfigSync,
	}

	result := syncer.sync(ctx)

	// Update instance status.
	now := metav1.Now()
	instance.Status.ConfigSync = &litellmv1alpha1.ConfigSyncStatus{
		LastSyncTime:           &now,
		SyncedModels:           result.SyncedModels,
		SyncedTeams:            result.SyncedTeams,
		SyncedUsers:            result.SyncedUsers,
		SyncedKeys:             result.SyncedKeys,
		SyncedOrganizations:    result.SyncedOrganizations,
		SyncedCustomers:        result.SyncedCustomers,
		UnmanagedModels:        result.UnmanagedModels,
		UnmanagedTeams:         result.UnmanagedTeams,
		UnmanagedUsers:         result.UnmanagedUsers,
		UnmanagedKeys:          result.UnmanagedKeys,
		UnmanagedOrganizations: result.UnmanagedOrganizations,
		UnmanagedCustomers:     result.UnmanagedCustomers,
		SyncErrors:             result.Errors,
	}

	if len(result.Errors) > 0 {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionConfigSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "SyncErrors",
			Message:            fmt.Sprintf("%d error(s) during config sync", len(result.Errors)),
			ObservedGeneration: instance.Generation,
		})
		emitEvent(r.Recorder, &instance, corev1.EventTypeWarning, EventReasonConfigSyncFailed,
			"Config sync completed with %d error(s)", len(result.Errors))
	} else {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionConfigSynced,
			Status:             metav1.ConditionTrue,
			Reason:             "SyncCompleted",
			Message:            r.buildSyncSummary(result),
			ObservedGeneration: instance.Generation,
		})
		emitEvent(r.Recorder, &instance, corev1.EventTypeNormal, EventReasonConfigSyncCompleted,
			"Config sync completed: %s", r.buildSyncSummary(result))
	}

	if err := r.Status().Update(ctx, &instance); err != nil {
		log.Error(err, "failed to update config sync status")
	}

	log.V(1).Info("config sync completed",
		"syncedModels", result.SyncedModels,
		"syncedTeams", result.SyncedTeams,
		"syncedUsers", result.SyncedUsers,
		"syncedKeys", result.SyncedKeys,
		"syncedOrganizations", result.SyncedOrganizations,
		"syncedCustomers", result.SyncedCustomers,
		"unmanagedModels", result.UnmanagedModels,
		"unmanagedTeams", result.UnmanagedTeams,
		"unmanagedUsers", result.UnmanagedUsers,
		"unmanagedKeys", result.UnmanagedKeys,
		"unmanagedOrganizations", result.UnmanagedOrganizations,
		"unmanagedCustomers", result.UnmanagedCustomers,
		"driftDetected", result.DriftDetected,
		"prunedResources", result.PrunedResources,
		"errors", len(result.Errors),
	)

	return ctrl.Result{RequeueAfter: syncInterval}, nil
}

// resolveMasterKey resolves the master key Secret for the instance.
func (r *ConfigSyncReconciler) resolveMasterKey(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) (string, error) {
	ref := instance.Spec.MasterKey.SecretRef
	if ref == nil && instance.Spec.MasterKey.AutoGenerate {
		ref = &litellmv1alpha1.SecretKeyRef{
			Name: instance.Name + "-master-key",
			Key:  "master-key",
		}
	}
	return getSecretValue(ctx, r.Client, instance.Namespace, ref)
}

// buildSyncSummary creates a human-readable one-line summary of sync results.
func (r *ConfigSyncReconciler) buildSyncSummary(result *syncResult) string {
	totalSynced := result.SyncedModels + result.SyncedTeams + result.SyncedUsers +
		result.SyncedKeys + result.SyncedOrganizations + result.SyncedCustomers
	totalUnmanaged := result.UnmanagedModels + result.UnmanagedTeams + result.UnmanagedUsers +
		result.UnmanagedKeys + result.UnmanagedOrganizations + result.UnmanagedCustomers

	summary := fmt.Sprintf("%d managed, %d unmanaged", totalSynced, totalUnmanaged)
	if result.DriftDetected > 0 {
		summary += fmt.Sprintf(", %d drift(s) detected", result.DriftDetected)
	}
	if result.PrunedResources > 0 {
		summary += fmt.Sprintf(", %d pruned", result.PrunedResources)
	}
	return summary
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMInstance{}).
		Named("configsync").
		Complete(r)
}

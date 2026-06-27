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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

const (
	// FinalizerName is the finalizer used by all controllers.
	FinalizerName = "litellm.palena.ai/finalizer"

	// AnnotationManagedBy marks resources managed by the operator.
	AnnotationManagedBy = "litellm.palena.ai/managed-by"

	// AnnotationSyncHash stores the hash of the last synced spec.
	AnnotationSyncHash = "litellm.palena.ai/sync-hash"

	// AnnotationAuthMode records how a model's provider auth was last pushed
	// ("credential" or "inline"). A flip between modes forces a delete+recreate
	// of the LiteLLM model, because /model/update merges and cannot clear the
	// provider fields (api_base/api_key/litellm_credential_name) left by the
	// previous mode.
	AnnotationAuthMode = "litellm.palena.ai/auth-mode"

	// LabelInstanceName labels resources with the instance name.
	LabelInstanceName = "litellm.palena.ai/instance"

	// LabelResourceType labels resources with their type.
	LabelResourceType = "litellm.palena.ai/resource-type"

	// LabelApp is the standard app label.
	LabelApp = "app.kubernetes.io/name"

	// LabelManagedBy is the standard managed-by label.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// Condition types.
	ConditionReady         = "Ready"
	ConditionDatabaseReady = "DatabaseReady"
	ConditionRedisReady    = "RedisReady"
	ConditionConfigSynced  = "ConfigSynced"
	ConditionSynced        = "Synced"

	// Event reasons — kept in one place so operators and alerting tooling
	// can filter on them reliably.
	EventReasonCreated            = "Created"
	EventReasonUpdated            = "Updated"
	EventReasonDeleted            = "Deleted"
	EventReasonSynced             = "Synced"
	EventReasonReconcileFailed    = "ReconcileFailed"
	EventReasonInstanceNotReady   = "InstanceNotReady"
	EventReasonSecretNotFound     = "SecretNotFound"
	EventReasonValidationFailed   = "ValidationFailed"
	EventReasonEnterpriseRequired = "EnterpriseLicenseRequired"
	EventReasonHealthDegraded     = "HealthDegraded"
	EventReasonHealthRestored     = "HealthRestored"
	EventReasonRedisDisconnected  = "RedisDisconnected"
	EventReasonRedisConnected     = "RedisConnected"

	// Config sync event reasons.
	EventReasonConfigSyncCompleted       = "ConfigSyncCompleted"
	EventReasonConfigSyncFailed          = "ConfigSyncFailed"
	EventReasonConfigSyncDriftDetected   = "ConfigSyncDriftDetected"
	EventReasonConfigSyncDriftRemediated = "ConfigSyncDriftRemediated"
	EventReasonConfigSyncPruned          = "ConfigSyncPruned"
	EventReasonConfigSyncRecreated       = "ConfigSyncRecreated"
	EventReasonConfigSyncUnmanaged       = "ConfigSyncUnmanaged"
)

// emitEvent records a Kubernetes Event on an object if the recorder is set.
// Recorder is nil in tests where the reconciler is constructed directly,
// so we guard every call site with this helper rather than passing a fake
// everywhere.
func emitEvent(r record.EventRecorder, obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r == nil || obj == nil {
		return
	}
	r.Eventf(obj, eventType, reason, messageFmt, args...)
}

// ResolvedInstance contains resolved instance information needed by secondary controllers.
type ResolvedInstance struct {
	Endpoint  string
	MasterKey string
	Instance  *litellmv1alpha1.LiteLLMInstance
}

// resolveInstance fetches a LiteLLMInstance and resolves its endpoint and master key.
func resolveInstance(
	ctx context.Context,
	c client.Client,
	namespace string,
	ref litellmv1alpha1.InstanceRef,
) (*ResolvedInstance, error) {
	var instance litellmv1alpha1.LiteLLMInstance
	if err := c.Get(ctx, types.NamespacedName{
		Name: ref.Name, Namespace: namespace,
	}, &instance); err != nil {
		return nil, fmt.Errorf("fetch instance %q: %w", ref.Name, err)
	}

	if !instance.Status.Ready {
		return nil, fmt.Errorf("instance %q is not ready", ref.Name)
	}

	masterKeyRef := instance.Spec.MasterKey.SecretRef
	if masterKeyRef == nil && instance.Spec.MasterKey.AutoGenerate {
		masterKeyRef = &litellmv1alpha1.SecretKeyRef{
			Name: instance.Name + "-master-key",
			Key:  "master-key",
		}
	}
	masterKey, err := getSecretValue(ctx, c, namespace, masterKeyRef)
	if err != nil {
		return nil, fmt.Errorf("get master key: %w", err)
	}

	return &ResolvedInstance{
		Endpoint:  instance.Status.Endpoint,
		MasterKey: masterKey,
		Instance:  &instance,
	}, nil
}

// getSecretValue reads a value from a Kubernetes Secret.
func getSecretValue(
	ctx context.Context,
	c client.Client,
	namespace string,
	ref *litellmv1alpha1.SecretKeyRef,
) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("secret ref is nil")
	}
	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{
		Name: ref.Name, Namespace: namespace,
	}, &secret); err != nil {
		return "", fmt.Errorf("fetch secret %q: %w", ref.Name, err)
	}
	val, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", ref.Key, ref.Name)
	}
	return string(val), nil
}

// computeSpecHash computes a deterministic hash of a spec for change detection.
func computeSpecHash(spec interface{}) string {
	data, _ := json.Marshal(spec)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8])
}

// labelsForInstance returns standard labels for an instance's child resources.
func labelsForInstance(instanceName string) map[string]string {
	return map[string]string{
		LabelApp:          "litellm",
		LabelManagedBy:    "litellm-operator",
		LabelInstanceName: instanceName,
	}
}

// isEnterpriseLicenseError checks if an API error indicates a missing enterprise license.
func isEnterpriseLicenseError(err error) bool {
	var apiErr *litellm.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 403 &&
			strings.Contains(strings.ToLower(apiErr.Message), "enterprise")
	}
	return false
}

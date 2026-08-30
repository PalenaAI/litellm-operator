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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMVirtualKeySpec defines the desired state of LiteLLMVirtualKey.
type LiteLLMVirtualKeySpec struct {
	// Reference to the LiteLLMInstance.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Instance Ref"
	InstanceRef InstanceRef `json:"instanceRef"`

	// Human-readable key alias.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Key Alias"
	KeyAlias string `json:"keyAlias"`

	// Reference to a LiteLLMTeam CR that this key belongs to.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Team Ref"
	TeamRef *InstanceRef `json:"teamRef,omitempty"`

	// Reference to a LiteLLMUser CR that this key belongs to.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="User Ref"
	UserRef *InstanceRef `json:"userRef,omitempty"`

	// Models this key can access.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Models"
	Models []string `json:"models,omitempty"`

	// Maximum budget in USD.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Budget"
	MaxBudget *string `json:"maxBudget,omitempty"`

	// Budget reset duration (e.g., "30d").
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Budget Duration"
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Key expiration time.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Expires At"
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// TPM limit for this key.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TPM Limit"
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// RPM limit for this key.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RPM Limit"
	RPMLimit *int `json:"rpmLimit,omitempty"`

	// SoftBudget is an alert threshold in USD below the hard maxBudget; crossing
	// it triggers a budget alert without blocking requests.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Soft Budget"
	SoftBudget *float64 `json:"softBudget,omitempty"`

	// ModelRPMLimit sets a per-model requests-per-minute cap (model name -> RPM).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model RPM Limit"
	ModelRPMLimit map[string]int `json:"modelRpmLimit,omitempty"`

	// ModelTPMLimit sets a per-model tokens-per-minute cap (model name -> TPM).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model TPM Limit"
	ModelTPMLimit map[string]int `json:"modelTpmLimit,omitempty"`

	// Custom metadata.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metadata"
	Metadata map[string]string `json:"metadata,omitempty"`

	// Blocked disables this key when true, rejecting all requests that use it.
	// Useful for incident response without deleting the key.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Blocked"
	Blocked *bool `json:"blocked,omitempty"`

	// ObjectPermission grants access to MCP servers, vector stores, agents,
	// and access groups.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Object Permission"
	ObjectPermission *ObjectPermission `json:"objectPermission,omitempty"`

	// Per-model spending limits for this key (enterprise).
	// Key: model name, Value: max budget in USD.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Max Budget"
	ModelMaxBudget map[string]string `json:"modelMaxBudget,omitempty"`

	// Maximum concurrent requests for this key.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Parallel Requests"
	MaxParallelRequests *int `json:"maxParallelRequests,omitempty"`

	// Name for the Secret that stores the generated API key.
	// Defaults to "{name}-key". Only honoured before the key is minted; once
	// status.keySecretRef is set the name is pinned, so that editing this field
	// cannot orphan the only copy of the key material.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Key Secret Name"
	KeySecretName string `json:"keySecretName,omitempty"`

	// Metadata to apply to the Secret that stores the generated API key, so that
	// third-party controllers can act on it (e.g. kubernetes-reflector mirroring
	// the Secret into the namespace of the consuming application).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Key Secret Template"
	KeySecretTemplate *KeySecretTemplateSpec `json:"keySecretTemplate,omitempty"`

	// Guardrails to activate for this key. Each entry must match the
	// guardrailName of a LiteLLMGuardrail CR bound to the same instance.
	// Requires a LiteLLM Enterprise license.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Guardrails"
	Guardrails []string `json:"guardrails,omitempty"`
}

// KeySecretTemplateSpec defines metadata applied to the operator-managed Secret
// holding the generated API key. Configured entries are applied and their values
// reconciled; entries added by other controllers are preserved. Because merging
// preserves what is already on the Secret, removing an entry here does not remove
// it from the Secret — delete it from the Secret directly.
type KeySecretTemplateSpec struct {
	// Annotations to apply to the key Secret, for example
	// reflector.v1.k8s.emberstack.com/reflection-allowed: "true" to have
	// kubernetes-reflector mirror the Secret into another namespace.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels to apply to the key Secret. The operator's own labels take
	// precedence on conflict.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// LiteLLMVirtualKeyStatus defines the observed state of LiteLLMVirtualKey.
type LiteLLMVirtualKeyStatus struct {
	// Whether the key is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// Reference to the Secret containing the API key.
	// +optional
	KeySecretRef *SecretKeyRef `json:"keySecretRef,omitempty"`

	// LiteLLM-assigned key token (hashed, for reference).
	LiteLLMKeyToken string `json:"litellmKeyToken,omitempty"`

	// Current spend on this key in USD.
	// +optional
	CurrentSpend *string `json:"currentSpend,omitempty"`

	// Whether the key is active.
	IsActive bool `json:"isActive,omitempty"`

	// Key expiration time.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lk
// +kubebuilder:printcolumn:name="Alias",type="string",JSONPath=".spec.keyAlias"
// +kubebuilder:printcolumn:name="Active",type="boolean",JSONPath=".status.isActive"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Virtual Key",resources={{LiteLLMInstance,v1alpha1,""},{Secret,v1,""}}

// LiteLLMVirtualKey is the Schema for the litellmvirtualkeys API.
type LiteLLMVirtualKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMVirtualKeySpec   `json:"spec,omitempty"`
	Status LiteLLMVirtualKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMVirtualKeyList contains a list of LiteLLMVirtualKey.
type LiteLLMVirtualKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMVirtualKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMVirtualKey{}, &LiteLLMVirtualKeyList{})
}

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

	// Custom metadata.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metadata"
	Metadata map[string]string `json:"metadata,omitempty"`

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
	// Defaults to "{name}-key".
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Key Secret Name"
	KeySecretName string `json:"keySecretName,omitempty"`

	// Guardrails to activate for this key. Each entry must match the
	// guardrailName of a LiteLLMGuardrail CR bound to the same instance.
	// Requires a LiteLLM Enterprise license.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Guardrails"
	Guardrails []string `json:"guardrails,omitempty"`
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

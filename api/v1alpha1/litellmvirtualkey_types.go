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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMVirtualKeySpec defines the desired state of LiteLLMVirtualKey.
type LiteLLMVirtualKeySpec struct {
	// Reference to the LiteLLMInstance.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Human-readable key alias.
	KeyAlias string `json:"keyAlias"`

	// Reference to a LiteLLMTeam CR that this key belongs to.
	// +optional
	TeamRef *InstanceRef `json:"teamRef,omitempty"`

	// Reference to a LiteLLMUser CR that this key belongs to.
	// +optional
	UserRef *InstanceRef `json:"userRef,omitempty"`

	// Models this key can access.
	// +optional
	Models []string `json:"models,omitempty"`

	// Maximum budget in USD.
	// +optional
	MaxBudget *string `json:"maxBudget,omitempty"`

	// Budget reset duration (e.g., "30d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Key expiration time.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// TPM limit for this key.
	// +optional
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// RPM limit for this key.
	// +optional
	RPMLimit *int `json:"rpmLimit,omitempty"`

	// Custom metadata.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Per-model spending limits for this key (enterprise).
	// Key: model name, Value: max budget in USD.
	// +optional
	ModelMaxBudget map[string]string `json:"modelMaxBudget,omitempty"`

	// Maximum concurrent requests for this key.
	// +optional
	MaxParallelRequests *int `json:"maxParallelRequests,omitempty"`

	// Name for the Secret that stores the generated API key.
	// Defaults to "{name}-key".
	// +optional
	KeySecretName string `json:"keySecretName,omitempty"`
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

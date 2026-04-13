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

// LiteLLMModelSpec defines the desired state of LiteLLMModel.
type LiteLLMModelSpec struct {
	// Reference to the LiteLLMInstance.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Model name exposed to clients.
	ModelName string `json:"modelName"`

	// LiteLLM-specific parameters for this model.
	LiteLLMParams LiteLLMModelParams `json:"litellmParams"`

	// Optional model metadata.
	// +optional
	ModelInfo *ModelInfo `json:"modelInfo,omitempty"`

	// Tags for tag-based routing. Requests with matching tags
	// will be routed to this model deployment.
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// LiteLLMModelParams defines provider-specific model parameters.
type LiteLLMModelParams struct {
	// Provider/model string (e.g., "openai/gpt-4", "anthropic/claude-3-opus").
	Model string `json:"model"`

	// API base URL for the provider.
	// +optional
	APIBase string `json:"apiBase,omitempty"`

	// Reference to Secret containing the provider API key.
	// +optional
	APIKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`

	// Reference to a LiteLLMCredential CR for provider authentication.
	// When set, the model is registered with `litellm_credential_name`
	// pointing at the credential, and apiKeySecretRef / apiBase on this
	// model are ignored in favor of the credential's values.
	// +optional
	CredentialRef *CredentialRef `json:"credentialRef,omitempty"`

	// Rate limit: requests per minute.
	// +optional
	RPM *int `json:"rpm,omitempty"`

	// Rate limit: tokens per minute.
	// +optional
	TPM *int `json:"tpm,omitempty"`

	// Request timeout in seconds.
	// +optional
	Timeout *int `json:"timeout,omitempty"`

	// Stream timeout in seconds.
	// +optional
	StreamTimeout *int `json:"streamTimeout,omitempty"`

	// Max retries for failed requests.
	// +optional
	MaxRetries *int `json:"maxRetries,omitempty"`
}

// ModelInfo defines optional model metadata.
type ModelInfo struct {
	// Maximum tokens supported.
	// +optional
	MaxTokens *int `json:"maxTokens,omitempty"`

	// Input cost per token in USD.
	// +optional
	InputCostPerToken *float64 `json:"inputCostPerToken,omitempty"`

	// Output cost per token in USD.
	// +optional
	OutputCostPerToken *float64 `json:"outputCostPerToken,omitempty"`
}

// LiteLLMModelStatus defines the observed state of LiteLLMModel.
type LiteLLMModelStatus struct {
	// Whether the model is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// LiteLLM-assigned model ID.
	LiteLLMModelID string `json:"litellmModelId,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Model health status reported by LiteLLM.
	Health string `json:"health,omitempty"`

	// P50 latency in milliseconds.
	// +optional
	LatencyP50Ms *int `json:"latencyP50Ms,omitempty"`

	// P95 latency in milliseconds.
	// +optional
	LatencyP95Ms *int `json:"latencyP95Ms,omitempty"`

	// Request count in last 24 hours.
	// +optional
	RequestsLast24h *int64 `json:"requestsLast24h,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lm
// +kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.modelName"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.health"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Model"

// LiteLLMModel is the Schema for the litellmmodels API.
type LiteLLMModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMModelSpec   `json:"spec,omitempty"`
	Status LiteLLMModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMModelList contains a list of LiteLLMModel.
type LiteLLMModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMModel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMModel{}, &LiteLLMModelList{})
}

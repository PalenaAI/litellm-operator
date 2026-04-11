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

// LiteLLMCredentialSpec defines the desired state of LiteLLMCredential.
//
// A LiteLLMCredential represents a reusable provider credential written to
// the proxy's `credential_list` config section. Multiple LiteLLMModel CRs
// can reference the same credential by name instead of each embedding its
// own API key, which reduces Secret sprawl and simplifies key rotation.
type LiteLLMCredentialSpec struct {
	// Reference to the LiteLLMInstance this credential belongs to.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Unique name for this credential, referenced by models via
	// litellm_params.litellm_credential_name.
	// +kubebuilder:validation:MinLength=1
	CredentialName string `json:"credentialName"`

	// Reference to a Secret containing the provider API key.
	APIKeySecretRef SecretKeyRef `json:"apiKeySecretRef"`

	// API base URL for the provider (e.g., https://api.openai.com/v1).
	// +optional
	APIBase string `json:"apiBase,omitempty"`

	// API version (e.g., "2024-02-01" for Azure OpenAI).
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Additional provider-specific parameters written verbatim into
	// credential_info alongside api_key / api_base / api_version.
	// +optional
	Params map[string]string `json:"params,omitempty"`
}

// LiteLLMCredentialStatus defines the observed state of LiteLLMCredential.
type LiteLLMCredentialStatus struct {
	// Whether the credential is configured in the referenced instance's proxy config.
	Configured bool `json:"configured,omitempty"`

	// Number of LiteLLMModel CRs referencing this credential via credentialRef.
	// +optional
	ReferencedByModels int `json:"referencedByModels,omitempty"`

	// Last time the credential config was validated and reconciled.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lc
// +kubebuilder:printcolumn:name="Credential",type="string",JSONPath=".spec.credentialName"
// +kubebuilder:printcolumn:name="Instance",type="string",JSONPath=".spec.instanceRef.name"
// +kubebuilder:printcolumn:name="Configured",type="boolean",JSONPath=".status.configured"
// +kubebuilder:printcolumn:name="Models",type="integer",JSONPath=".status.referencedByModels"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LiteLLMCredential is the Schema for the litellmcredentials API.
type LiteLLMCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMCredentialSpec   `json:"spec,omitempty"`
	Status LiteLLMCredentialStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMCredentialList contains a list of LiteLLMCredential.
type LiteLLMCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMCredential `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMCredential{}, &LiteLLMCredentialList{})
}

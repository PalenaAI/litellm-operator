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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMGuardrailSpec defines the desired state of LiteLLMGuardrail.
//
// A LiteLLMGuardrail defines a content moderation / safety integration that
// the proxy should run against requests and/or responses. Guardrails are a
// config-level feature: the operator renders them into the proxy's
// `guardrails` section in proxy_server_config.yaml and injects any required
// API key env vars into the Deployment. The instance controller observes
// LiteLLMGuardrail CRs via a Watch and rewrites the ConfigMap + Deployment
// whenever they change.
type LiteLLMGuardrailSpec struct {
	// Reference to the LiteLLMInstance this guardrail belongs to.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Unique name for this guardrail. Used both as the `guardrail_name` in
	// the proxy config and as the identifier referenced from
	// LiteLLMVirtualKey.spec.guardrails / LiteLLMTeam.spec.guardrails.
	// +kubebuilder:validation:MinLength=1
	GuardrailName string `json:"guardrailName"`

	// Guardrail provider. Must be a LiteLLM-supported integration.
	//
	// "generic_guardrail_api" points the proxy at any HTTP service (e.g. a
	// container you host in-cluster): LiteLLM POSTs request/response content
	// to `{apiBase}/beta/litellm_basic_guardrail_api` and expects a verdict
	// back. No Python class is required. Use `apiBase` for the endpoint,
	// `apiKeySecretRef` for Bearer auth, `unreachableFallback` for failure
	// behavior, and `params` for `additional_provider_specific_params`.
	// +kubebuilder:validation:Enum=aporia;lakera;bedrock;presidio;guardrails_ai;azure;llm_guard;llamaguard;google_text_moderation;custom_guardrail;generic_guardrail_api
	Provider string `json:"provider"`

	// GuardrailClass is the dotted Python import path to a litellm
	// CustomGuardrail subclass. REQUIRED when provider == "custom_guardrail";
	// must be empty for all other providers.
	// +optional
	GuardrailClass string `json:"guardrailClass,omitempty"`

	// UnreachableFallback controls proxy behavior when the guardrail endpoint
	// is unreachable (network error, or upstream 502/503/504). Only applies
	// when provider == "generic_guardrail_api"; must be empty otherwise.
	//   fail_closed — reject the request (LiteLLM default)
	//   fail_open   — log a critical error and let the request proceed
	// +kubebuilder:validation:Enum=fail_closed;fail_open
	// +optional
	UnreachableFallback string `json:"unreachableFallback,omitempty"`

	// Execution mode for the guardrail.
	//   pre_call      — runs before the LLM request is dispatched
	//   post_call     — runs after the LLM response is received
	//   during_call   — runs in parallel with the LLM request
	//   logging_only  — evaluated but never blocks the request
	// +kubebuilder:validation:Enum=pre_call;post_call;during_call;logging_only
	Mode string `json:"mode"`

	// API key for the guardrail provider. The operator injects this as an
	// env var named `GUARDRAIL_{NAME}_API_KEY` (sanitized) and writes
	// `os.environ/GUARDRAIL_{NAME}_API_KEY` into the proxy config.
	// +optional
	APIKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`

	// API base URL for the guardrail provider (e.g. https://api.aporia.com).
	// +optional
	APIBase string `json:"apiBase,omitempty"`

	// Whether this guardrail runs on every request by default even when a
	// key/team does not explicitly opt in. Writes `default_on: true` into
	// the guardrail's litellm_params section.
	// +optional
	DefaultOn *bool `json:"defaultOn,omitempty"`

	// Provider-specific parameters that are merged verbatim into the
	// guardrail's `litellm_params` section. For example, Bedrock guardrails
	// use `guardrailIdentifier` and `guardrailVersion`; Presidio uses
	// `presidio_analyzer_api_base`; Lakera uses `category_thresholds`.
	//
	// For provider == "generic_guardrail_api", these are nested under
	// `additional_provider_specific_params` instead of being merged at the
	// top level, since that is where the generic guardrail API reads them.
	//
	// Values are arbitrary JSON (strings, numbers, bools, or nested
	// objects/arrays), so structured provider config — e.g. Presidio's
	// `pii_entities_config` ({CREDIT_CARD: MASK}) or per-entity thresholds —
	// can be expressed directly, not just as strings.
	// +optional
	Params map[string]apiextensionsv1.JSON `json:"params,omitempty"`

	// Extra environment variables needed by this guardrail beyond the API
	// key (e.g. additional credentials for bedrock, custom endpoints).
	// +optional
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`
}

// LiteLLMGuardrailStatus defines the observed state of LiteLLMGuardrail.
type LiteLLMGuardrailStatus struct {
	// Whether the guardrail has been rendered into the instance's proxy
	// config. Set to true once the instance controller has observed the
	// spec and included it in the latest ConfigMap reconciliation.
	Configured bool `json:"configured,omitempty"`

	// Last time this guardrail's spec was validated and reconciled.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lg
// +kubebuilder:printcolumn:name="Guardrail",type="string",JSONPath=".spec.guardrailName"
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode"
// +kubebuilder:printcolumn:name="Instance",type="string",JSONPath=".spec.instanceRef.name"
// +kubebuilder:printcolumn:name="Configured",type="boolean",JSONPath=".status.configured"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Guardrail"

// LiteLLMGuardrail is the Schema for the litellmguardrails API.
//
// Guardrails are config-level resources: the actual `guardrails` entries are
// materialized by the LiteLLMInstance controller when it rebuilds the
// ConfigMap + Deployment. Per-key and per-team guardrail assignment (via
// LiteLLMVirtualKey.spec.guardrails / LiteLLMTeam.spec.guardrails) requires
// a LiteLLM Enterprise license.
type LiteLLMGuardrail struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMGuardrailSpec   `json:"spec,omitempty"`
	Status LiteLLMGuardrailStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMGuardrailList contains a list of LiteLLMGuardrail.
type LiteLLMGuardrailList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMGuardrail `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMGuardrail{}, &LiteLLMGuardrailList{})
}

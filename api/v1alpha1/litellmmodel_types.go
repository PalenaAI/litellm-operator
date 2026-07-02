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

// LiteLLMModelSpec defines the desired state of LiteLLMModel.
type LiteLLMModelSpec struct {
	// Reference to the LiteLLMInstance.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Instance Ref"
	InstanceRef InstanceRef `json:"instanceRef"`

	// Model name exposed to clients.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Name"
	ModelName string `json:"modelName"`

	// LiteLLM-specific parameters for this model.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="LiteLLM Params"
	LiteLLMParams LiteLLMModelParams `json:"litellmParams"`

	// Optional model metadata.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Info"
	ModelInfo *ModelInfo `json:"modelInfo,omitempty"`

	// Tags for tag-based routing. Requests with matching tags
	// will be routed to this model deployment.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tags"
	Tags []string `json:"tags,omitempty"`
}

// LiteLLMModelParams defines provider-specific model parameters.
type LiteLLMModelParams struct {
	// Provider/model string (e.g., "openai/gpt-4", "anthropic/claude-3-opus").
	Model string `json:"model"`

	// API base URL for the provider.
	// +optional
	APIBase string `json:"apiBase,omitempty"`

	// API version (e.g., "2024-10-21" for Azure OpenAI / Azure AI Foundry).
	// Required by most Azure deployments. When credentialRef is set, the
	// value from the LiteLLMCredential takes precedence over this field.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Reference to Secret containing the provider API key.
	// +optional
	APIKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`

	// Reference to a LiteLLMCredential CR for provider authentication.
	// When set, the credential's resolved api_base / api_version / api_key are
	// written inline onto the model (and `litellm_credential_name` is also sent
	// for Admin UI association). apiBase / apiVersion / apiKeySecretRef on this
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

	// Weight for weighted load balancing across the deployments in this
	// model group. A deployment with weight 2 receives roughly twice the
	// traffic of a deployment with weight 1.
	// +optional
	Weight *int `json:"weight,omitempty"`

	// Order sets the deployment's routing priority within its model group.
	// Lower numbers are preferred; higher-order deployments are only used
	// when lower-order ones are unavailable (fallback-style ordering).
	// +optional
	Order *int `json:"order,omitempty"`

	// MaxInputTokens is the context-window size the router uses for
	// context-window-aware routing and fallbacks (context_window_fallbacks).
	// +optional
	MaxInputTokens *int `json:"maxInputTokens,omitempty"`

	// Temperature is a default temperature applied to every request routed to
	// this deployment unless the caller overrides it.
	// +optional
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP is a default top_p applied to every request routed to this
	// deployment unless the caller overrides it.
	// +optional
	TopP *float64 `json:"topP,omitempty"`

	// MaxTokens is a default max_tokens (max completion tokens) applied to
	// requests to this deployment. This is a request-time default sent to the
	// provider and is distinct from modelInfo.maxTokens, which describes the
	// model's total token capacity for routing decisions.
	// +optional
	MaxTokens *int `json:"maxTokens,omitempty"`

	// Seed is a default seed applied to requests for reproducible outputs
	// (providers that support it).
	// +optional
	Seed *int `json:"seed,omitempty"`

	// Organization is the provider organization identifier (e.g. an OpenAI
	// organization ID) sent with requests to this deployment.
	// +optional
	Organization string `json:"organization,omitempty"`

	// AWSRegionName is the AWS region for AWS-backed providers such as Bedrock
	// and SageMaker (e.g. "us-east-1").
	// +optional
	AWSRegionName string `json:"awsRegionName,omitempty"`

	// ExtraHeaders are additional HTTP headers sent on every upstream request
	// to this deployment (e.g. provider-specific routing or beta headers).
	// +optional
	ExtraHeaders map[string]string `json:"extraHeaders,omitempty"`
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

	// Mode declares the model type so LiteLLM runs the correct health check
	// and routing logic. Common values: "chat", "completion", "embedding",
	// "image_generation", "audio_transcription", "audio_speech", "moderation",
	// "rerank", "responses", "batch", "realtime". Leave empty to let LiteLLM
	// infer from the provider/model string.
	// +optional
	Mode string `json:"mode,omitempty"`

	// BaseModel maps this deployment to a known base model for accurate cost
	// tracking and token accounting. Required for Azure deployments where the
	// deployment name differs from the underlying model (e.g. "azure/gpt-4o").
	// +optional
	BaseModel string `json:"baseModel,omitempty"`

	// Tier classifies the deployment for tier-based routing. Common values are
	// "free" and "paid".
	// +optional
	Tier string `json:"tier,omitempty"`

	// RegionName is the geographic region used for region-based routing
	// (e.g. "us-east-1", "eu").
	// +optional
	RegionName string `json:"regionName,omitempty"`

	// AccessGroups restrict which virtual keys / teams may use this model.
	// A key must be granted one of the listed access groups to route here.
	// +optional
	AccessGroups []string `json:"accessGroups,omitempty"`

	// SupportedEnvironments limits which environments expose this deployment
	// (e.g. "production", "staging", "development").
	// +optional
	SupportedEnvironments []string `json:"supportedEnvironments,omitempty"`

	// UseInPassThrough allows this deployment to be selected by pass-through
	// endpoints in addition to regular routing.
	// +optional
	UseInPassThrough *bool `json:"useInPassThrough,omitempty"`

	// InputCostPerPixel is the cost per pixel in USD for image models.
	// +optional
	InputCostPerPixel *float64 `json:"inputCostPerPixel,omitempty"`

	// InputCostPerSecond is the cost per second in USD for audio / realtime
	// models billed by duration.
	// +optional
	InputCostPerSecond *float64 `json:"inputCostPerSecond,omitempty"`

	// CacheReadInputTokenCost is the cost per token in USD for reads that hit
	// the provider's prompt cache (e.g. Anthropic / OpenAI prompt caching).
	// +optional
	CacheReadInputTokenCost *float64 `json:"cacheReadInputTokenCost,omitempty"`

	// CacheCreationInputTokenCost is the cost per token in USD for writing to
	// the provider's prompt cache.
	// +optional
	CacheCreationInputTokenCost *float64 `json:"cacheCreationInputTokenCost,omitempty"`

	// HealthCheck tunes or disables LiteLLM's per-model health checks.
	// +optional
	HealthCheck *ModelHealthCheck `json:"healthCheck,omitempty"`
}

// ModelHealthCheck controls LiteLLM's health checking for a single model
// deployment. All fields are optional; unset fields fall back to LiteLLM's
// defaults. These map to the health-check keys under model_info in
// proxy_server_config.yaml.
type ModelHealthCheck struct {
	// DisableBackgroundHealthCheck skips background health checks for this
	// model when the proxy runs with background_health_checks enabled. Useful
	// for providers that bill or rate-limit health probes, or models that do
	// not support the probe request shape.
	// +optional
	DisableBackgroundHealthCheck *bool `json:"disableBackgroundHealthCheck,omitempty"`

	// TimeoutSeconds overrides the health-check request timeout for this model
	// (LiteLLM default is 60 seconds).
	// +optional
	TimeoutSeconds *int `json:"timeoutSeconds,omitempty"`

	// MaxTokens overrides the max_tokens used for the health-check request.
	// +optional
	MaxTokens *int `json:"maxTokens,omitempty"`

	// MaxTokensReasoning overrides the health-check max_tokens for reasoning
	// models (which require a larger budget to return a valid response).
	// +optional
	MaxTokensReasoning *int `json:"maxTokensReasoning,omitempty"`

	// MaxTokensNonReasoning overrides the health-check max_tokens for
	// non-reasoning models.
	// +optional
	MaxTokensNonReasoning *int `json:"maxTokensNonReasoning,omitempty"`

	// ReasoningEffort sets the reasoning effort used for health-check requests
	// against reasoning models (e.g. "none", "low", "medium", "high").
	// +optional
	ReasoningEffort string `json:"reasoningEffort,omitempty"`

	// Voice sets the voice used for health-check requests against
	// text-to-speech models (e.g. "alloy").
	// +optional
	Voice string `json:"voice,omitempty"`

	// Model overrides the model used for the health-check request. Primarily
	// used for wildcard routes where the deployment does not map to a single
	// concrete model (e.g. "openai/gpt-4o-mini").
	// +optional
	Model string `json:"model,omitempty"`
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
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Model",resources={{LiteLLMInstance,v1alpha1,""}}

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

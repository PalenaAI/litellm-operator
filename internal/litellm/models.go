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

package litellm

import (
	"context"
	"net/http"
)

// ModelService defines operations on LiteLLM models.
type ModelService interface {
	Create(ctx context.Context, req ModelCreateRequest) (*ModelCreateResponse, error)
	Update(ctx context.Context, req ModelCreateRequest) error
	Delete(ctx context.Context, modelID string) error
	Get(ctx context.Context, modelID string) (*ModelInfoResponse, error)
	List(ctx context.Context) ([]ModelInfoResponse, error)
}

// ModelCreateRequest is the request to create or update a model.
type ModelCreateRequest struct {
	ModelName     string        `json:"model_name"`
	LiteLLMParams ModelParams   `json:"litellm_params"`
	ModelInfo     *ModelInfoReq `json:"model_info,omitempty"`
	ModelID       string        `json:"model_id,omitempty"`
}

// ModelParams defines provider-specific model parameters.
type ModelParams struct {
	Model                 string            `json:"model"`
	LiteLLMCredentialName string            `json:"litellm_credential_name,omitempty"`
	APIBase               string            `json:"api_base,omitempty"`
	APIKey                string            `json:"api_key,omitempty"`
	APIVersion            string            `json:"api_version,omitempty"`
	RPM                   *int              `json:"rpm,omitempty"`
	TPM                   *int              `json:"tpm,omitempty"`
	Timeout               *int              `json:"timeout,omitempty"`
	StreamTimeout         *int              `json:"stream_timeout,omitempty"`
	MaxRetries            *int              `json:"max_retries,omitempty"`
	Tags                  []string          `json:"tags,omitempty"`
	Weight                *int              `json:"weight,omitempty"`
	Order                 *int              `json:"order,omitempty"`
	MaxInputTokens        *int              `json:"max_input_tokens,omitempty"`
	Temperature           *float64          `json:"temperature,omitempty"`
	TopP                  *float64          `json:"top_p,omitempty"`
	MaxTokens             *int              `json:"max_tokens,omitempty"`
	Seed                  *int              `json:"seed,omitempty"`
	Organization          string            `json:"organization,omitempty"`
	AWSRegionName         string            `json:"aws_region_name,omitempty"`
	ExtraHeaders          map[string]string `json:"extra_headers,omitempty"`
}

// ModelInfoReq defines optional model info in requests. Health-check fields are
// flattened here to match LiteLLM's wire format, where they live directly under
// model_info rather than in a nested object.
type ModelInfoReq struct {
	ID                 string   `json:"id,omitempty"`
	MaxTokens          *int     `json:"max_tokens,omitempty"`
	InputCostPerToken  *float64 `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken *float64 `json:"output_cost_per_token,omitempty"`

	Mode                  string   `json:"mode,omitempty"`
	BaseModel             string   `json:"base_model,omitempty"`
	Tier                  string   `json:"tier,omitempty"`
	RegionName            string   `json:"region_name,omitempty"`
	AccessGroups          []string `json:"access_groups,omitempty"`
	SupportedEnvironments []string `json:"supported_environments,omitempty"`
	UseInPassThrough      *bool    `json:"use_in_pass_through,omitempty"`

	InputCostPerPixel           *float64 `json:"input_cost_per_pixel,omitempty"`
	InputCostPerSecond          *float64 `json:"input_cost_per_second,omitempty"`
	CacheReadInputTokenCost     *float64 `json:"cache_read_input_token_cost,omitempty"`
	CacheCreationInputTokenCost *float64 `json:"cache_creation_input_token_cost,omitempty"`

	DisableBackgroundHealthCheck     *bool  `json:"disable_background_health_check,omitempty"`
	HealthCheckTimeout               *int   `json:"health_check_timeout,omitempty"`
	HealthCheckMaxTokens             *int   `json:"health_check_max_tokens,omitempty"`
	HealthCheckMaxTokensReasoning    *int   `json:"health_check_max_tokens_reasoning,omitempty"`
	HealthCheckMaxTokensNonReasoning *int   `json:"health_check_max_tokens_non_reasoning,omitempty"`
	HealthCheckReasoningEffort       string `json:"health_check_reasoning_effort,omitempty"`
	HealthCheckVoice                 string `json:"health_check_voice,omitempty"`
	HealthCheckModel                 string `json:"health_check_model,omitempty"`
}

// ModelCreateResponse is the response from creating a model.
type ModelCreateResponse struct {
	ModelID string `json:"model_id"`
}

// ModelInfoResponse is the response from getting model info.
type ModelInfoResponse struct {
	ModelID   string      `json:"model_id"`
	ModelName string      `json:"model_name"`
	Params    ModelParams `json:"litellm_params"`
}

type modelService struct {
	client *httpClient
}

func (s *modelService) Create(ctx context.Context, req ModelCreateRequest) (*ModelCreateResponse, error) {
	var resp ModelCreateResponse
	err := s.client.do(ctx, http.MethodPost, "/model/new", req, &resp)
	return &resp, err
}

func (s *modelService) Update(ctx context.Context, req ModelCreateRequest) error {
	return s.client.do(ctx, http.MethodPost, "/model/update", req, nil)
}

func (s *modelService) Delete(ctx context.Context, modelID string) error {
	body := map[string]string{"id": modelID}
	return s.client.do(ctx, http.MethodPost, "/model/delete", body, nil)
}

func (s *modelService) Get(ctx context.Context, modelID string) (*ModelInfoResponse, error) {
	var resp ModelInfoResponse
	err := s.client.do(ctx, http.MethodGet, "/model/info?litellm_model_id="+modelID, nil, &resp)
	return &resp, err
}

func (s *modelService) List(ctx context.Context) ([]ModelInfoResponse, error) {
	var resp struct {
		Data []ModelInfoResponse `json:"data"`
	}
	err := s.client.do(ctx, http.MethodGet, "/model/info", nil, &resp)
	return resp.Data, err
}

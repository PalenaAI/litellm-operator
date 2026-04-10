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

package litellm

import (
	"context"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KeyService defines operations on LiteLLM virtual keys.
type KeyService interface {
	Generate(ctx context.Context, req KeyGenerateRequest) (*KeyGenerateResponse, error)
	Update(ctx context.Context, req KeyUpdateRequest) error
	Delete(ctx context.Context, token string) error
	Get(ctx context.Context, token string) (*KeyInfo, error)
	List(ctx context.Context) ([]KeyInfo, error)
}

// KeyGenerateRequest is the request to generate a virtual key.
type KeyGenerateRequest struct {
	KeyAlias            string             `json:"key_alias,omitempty"`
	TeamID              string             `json:"team_id,omitempty"`
	UserID              string             `json:"user_id,omitempty"`
	Models              []string           `json:"models,omitempty"`
	MaxBudget           *string            `json:"max_budget,omitempty"`
	BudgetDuration      string             `json:"budget_duration,omitempty"`
	ExpiresAt           *metav1.Time       `json:"expires,omitempty"`
	TPMLimit            *int               `json:"tpm_limit,omitempty"`
	RPMLimit            *int               `json:"rpm_limit,omitempty"`
	Metadata            map[string]string  `json:"metadata,omitempty"`
	ModelMaxBudget      map[string]float64 `json:"model_max_budget,omitempty"`
	MaxParallelRequests *int               `json:"max_parallel_requests,omitempty"`
}

// KeyGenerateResponse is the response from generating a key.
type KeyGenerateResponse struct {
	Key   string `json:"key"`
	Token string `json:"token"`
}

// KeyUpdateRequest is the request to update a virtual key.
type KeyUpdateRequest struct {
	Token               string             `json:"key"`
	Models              []string           `json:"models,omitempty"`
	MaxBudget           *string            `json:"max_budget,omitempty"`
	BudgetDuration      string             `json:"budget_duration,omitempty"`
	TPMLimit            *int               `json:"tpm_limit,omitempty"`
	RPMLimit            *int               `json:"rpm_limit,omitempty"`
	Metadata            map[string]string  `json:"metadata,omitempty"`
	ModelMaxBudget      map[string]float64 `json:"model_max_budget,omitempty"`
	MaxParallelRequests *int               `json:"max_parallel_requests,omitempty"`
}

// KeyInfo is the response from getting key info.
type KeyInfo struct {
	Token    string   `json:"token"`
	KeyAlias string   `json:"key_alias"`
	Spend    *string  `json:"spend"`
	IsActive bool     `json:"key_status"`
	Models   []string `json:"models"`
}

type keyService struct {
	client *httpClient
}

func (s *keyService) Generate(ctx context.Context, req KeyGenerateRequest) (*KeyGenerateResponse, error) {
	var resp KeyGenerateResponse
	err := s.client.do(ctx, http.MethodPost, "/key/generate", req, &resp)
	return &resp, err
}

func (s *keyService) Update(ctx context.Context, req KeyUpdateRequest) error {
	return s.client.do(ctx, http.MethodPost, "/key/update", req, nil)
}

func (s *keyService) Delete(ctx context.Context, token string) error {
	body := map[string][]string{"keys": {token}}
	return s.client.do(ctx, http.MethodPost, "/key/delete", body, nil)
}

func (s *keyService) Get(ctx context.Context, token string) (*KeyInfo, error) {
	var resp KeyInfo
	err := s.client.do(ctx, http.MethodGet, "/key/info?key="+token, nil, &resp)
	return &resp, err
}

func (s *keyService) List(ctx context.Context) ([]KeyInfo, error) {
	var resp []KeyInfo
	err := s.client.do(ctx, http.MethodGet, "/key/list", nil, &resp)
	return resp, err
}

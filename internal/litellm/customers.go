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

// CustomerService defines operations on LiteLLM end-users (customers).
// The LiteLLM API refers to these as "end_users" but exposes them under
// the /customer/ path.
type CustomerService interface {
	Create(ctx context.Context, req CustomerCreateRequest) (*CustomerInfo, error)
	Update(ctx context.Context, req CustomerUpdateRequest) (*CustomerInfo, error)
	Delete(ctx context.Context, customerID string) error
	Get(ctx context.Context, customerID string) (*CustomerInfo, error)
	List(ctx context.Context) ([]CustomerInfo, error)
}

// ObjectPermission restricts which objects (MCP servers, vector stores, agents,
// access groups) an identity may access. Shared across the customer/team/user/
// key/organization request payloads.
type ObjectPermission struct {
	MCPServers   []string `json:"mcp_servers,omitempty"`
	AccessGroups []string `json:"mcp_access_groups,omitempty"`
	VectorStores []string `json:"vector_stores,omitempty"`
	Agents       []string `json:"agents,omitempty"`
}

// CustomerObjectPermission is retained for backward compatibility as an alias
// for the shared [ObjectPermission] wire type.
type CustomerObjectPermission = ObjectPermission

// CustomerCreateRequest is the request body for POST /customer/new.
type CustomerCreateRequest struct {
	UserID             string                    `json:"user_id"`
	Alias              string                    `json:"alias,omitempty"`
	MaxBudget          *float64                  `json:"max_budget,omitempty"`
	BudgetDuration     string                    `json:"budget_duration,omitempty"`
	BudgetID           string                    `json:"budget_id,omitempty"`
	TpmLimit           *int64                    `json:"tpm_limit,omitempty"`
	RpmLimit           *int64                    `json:"rpm_limit,omitempty"`
	AllowedModelRegion string                    `json:"allowed_model_region,omitempty"`
	DefaultModel       string                    `json:"default_model,omitempty"`
	Models             []string                  `json:"models,omitempty"`
	Blocked            *bool                     `json:"blocked,omitempty"`
	ObjectPermission   *CustomerObjectPermission `json:"object_permission,omitempty"`
	Metadata           map[string]string         `json:"metadata,omitempty"`
}

// CustomerUpdateRequest is the request body for POST /customer/update.
type CustomerUpdateRequest struct {
	UserID             string                    `json:"user_id"`
	Alias              string                    `json:"alias,omitempty"`
	MaxBudget          *float64                  `json:"max_budget,omitempty"`
	BudgetDuration     string                    `json:"budget_duration,omitempty"`
	BudgetID           string                    `json:"budget_id,omitempty"`
	TpmLimit           *int64                    `json:"tpm_limit,omitempty"`
	RpmLimit           *int64                    `json:"rpm_limit,omitempty"`
	AllowedModelRegion string                    `json:"allowed_model_region,omitempty"`
	DefaultModel       string                    `json:"default_model,omitempty"`
	Models             []string                  `json:"models,omitempty"`
	Blocked            *bool                     `json:"blocked,omitempty"`
	ObjectPermission   *CustomerObjectPermission `json:"object_permission,omitempty"`
	Metadata           map[string]string         `json:"metadata,omitempty"`
}

// CustomerInfo is the response body for /customer/info and /customer/new.
type CustomerInfo struct {
	UserID         string   `json:"user_id"`
	Alias          string   `json:"alias"`
	Spend          *float64 `json:"spend"`
	MaxBudget      *float64 `json:"max_budget"`
	BudgetDuration string   `json:"budget_duration"`
	BudgetID       string   `json:"budget_id"`
	TpmLimit       *int64   `json:"tpm_limit"`
	RpmLimit       *int64   `json:"rpm_limit"`
	Models         []string `json:"models"`
	Blocked        bool     `json:"blocked"`
}

type customerService struct {
	client *httpClient
}

func (s *customerService) Create(ctx context.Context, req CustomerCreateRequest) (*CustomerInfo, error) {
	var resp CustomerInfo
	err := s.client.do(ctx, http.MethodPost, "/customer/new", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *customerService) Update(ctx context.Context, req CustomerUpdateRequest) (*CustomerInfo, error) {
	var resp CustomerInfo
	err := s.client.do(ctx, http.MethodPost, "/customer/update", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *customerService) Delete(ctx context.Context, customerID string) error {
	body := map[string][]string{"user_ids": {customerID}}
	return s.client.do(ctx, http.MethodPost, "/customer/delete", body, nil)
}

func (s *customerService) Get(ctx context.Context, customerID string) (*CustomerInfo, error) {
	var resp CustomerInfo
	err := s.client.do(ctx, http.MethodGet, "/customer/info?end_user_id="+customerID, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *customerService) List(ctx context.Context) ([]CustomerInfo, error) {
	var resp []CustomerInfo
	err := s.client.do(ctx, http.MethodGet, "/customer/list", nil, &resp)
	return resp, err
}

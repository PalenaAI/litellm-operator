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

// OrganizationService defines operations on LiteLLM organizations.
type OrganizationService interface {
	Create(ctx context.Context, req OrganizationCreateRequest) (*OrganizationCreateResponse, error)
	Update(ctx context.Context, req OrganizationUpdateRequest) error
	Delete(ctx context.Context, organizationID string) error
	Get(ctx context.Context, organizationID string) (*OrganizationInfo, error)
	List(ctx context.Context) ([]OrganizationInfo, error)
	AddMember(ctx context.Context, req OrgMemberAddRequest) error
	DeleteMember(ctx context.Context, organizationID, userID string) error
}

// OrganizationCreateRequest is the request to create an organization.
type OrganizationCreateRequest struct {
	OrganizationAlias string            `json:"organization_alias"`
	Models            []string          `json:"models,omitempty"`
	MaxBudget         *float64          `json:"max_budget,omitempty"`
	BudgetDuration    string            `json:"budget_duration,omitempty"`
	TpmLimit          *int64            `json:"tpm_limit,omitempty"`
	RpmLimit          *int64            `json:"rpm_limit,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// OrganizationCreateResponse is the response from creating an organization.
type OrganizationCreateResponse struct {
	OrganizationID    string `json:"organization_id"`
	OrganizationAlias string `json:"organization_alias"`
}

// OrganizationUpdateRequest is the request to update an organization.
type OrganizationUpdateRequest struct {
	OrganizationID    string            `json:"organization_id"`
	OrganizationAlias string            `json:"organization_alias,omitempty"`
	Models            []string          `json:"models,omitempty"`
	MaxBudget         *float64          `json:"max_budget,omitempty"`
	BudgetDuration    string            `json:"budget_duration,omitempty"`
	TpmLimit          *int64            `json:"tpm_limit,omitempty"`
	RpmLimit          *int64            `json:"rpm_limit,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// OrganizationInfo is the response from getting organization info.
type OrganizationInfo struct {
	OrganizationID    string          `json:"organization_id"`
	OrganizationAlias string          `json:"organization_alias"`
	Models            []string        `json:"models"`
	Spend             *float64        `json:"spend"`
	Members           []OrgMemberInfo `json:"members"`
}

// OrgMemberInfo describes an organization member from the API.
type OrgMemberInfo struct {
	UserID string `json:"user_id"`
	Email  string `json:"user_email"`
	Role   string `json:"role"`
}

// OrgMemberAddRequest is the request to add a member to an organization.
type OrgMemberAddRequest struct {
	OrganizationID string           `json:"organization_id"`
	Member         OrgMemberRequest `json:"member"`
}

// OrgMemberRequest defines a member to add to an organization.
type OrgMemberRequest struct {
	UserEmail string `json:"user_email"`
	Role      string `json:"role"`
}

type organizationService struct {
	client *httpClient
}

func (s *organizationService) Create(ctx context.Context, req OrganizationCreateRequest) (*OrganizationCreateResponse, error) {
	var resp OrganizationCreateResponse
	err := s.client.do(ctx, http.MethodPost, "/organization/new", req, &resp)
	return &resp, err
}

func (s *organizationService) Update(ctx context.Context, req OrganizationUpdateRequest) error {
	return s.client.do(ctx, http.MethodPost, "/organization/update", req, nil)
}

func (s *organizationService) Delete(ctx context.Context, organizationID string) error {
	body := map[string][]string{"organization_ids": {organizationID}}
	return s.client.do(ctx, http.MethodPost, "/organization/delete", body, nil)
}

func (s *organizationService) Get(ctx context.Context, organizationID string) (*OrganizationInfo, error) {
	var resp OrganizationInfo
	err := s.client.do(ctx, http.MethodGet, "/organization/info?organization_id="+organizationID, nil, &resp)
	return &resp, err
}

func (s *organizationService) List(ctx context.Context) ([]OrganizationInfo, error) {
	var resp []OrganizationInfo
	err := s.client.do(ctx, http.MethodGet, "/organization/list", nil, &resp)
	return resp, err
}

func (s *organizationService) AddMember(ctx context.Context, req OrgMemberAddRequest) error {
	return s.client.do(ctx, http.MethodPost, "/organization/member_add", req, nil)
}

func (s *organizationService) DeleteMember(ctx context.Context, organizationID, userID string) error {
	body := map[string]string{
		"organization_id": organizationID,
		"user_id":         userID,
	}
	return s.client.do(ctx, http.MethodPost, "/organization/member_delete", body, nil)
}

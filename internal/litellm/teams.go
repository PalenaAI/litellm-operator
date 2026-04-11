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
)

// TeamService defines operations on LiteLLM teams.
type TeamService interface {
	Create(ctx context.Context, req TeamCreateRequest) (*TeamCreateResponse, error)
	Update(ctx context.Context, req TeamUpdateRequest) error
	Delete(ctx context.Context, teamID string) error
	Get(ctx context.Context, teamID string) (*TeamInfo, error)
	List(ctx context.Context) ([]TeamInfo, error)
	AddMember(ctx context.Context, teamID, email, role string) error
	RemoveMember(ctx context.Context, teamID, email string) error
	ListMembers(ctx context.Context, teamID string) ([]TeamMemberInfo, error)
}

// TeamCreateRequest is the request to create a team.
type TeamCreateRequest struct {
	TeamAlias           string            `json:"team_alias"`
	OrganizationID      string            `json:"organization_id,omitempty"`
	Models              []string          `json:"models,omitempty"`
	MaxBudget           *float64          `json:"max_budget,omitempty"`
	BudgetDuration      string            `json:"budget_duration,omitempty"`
	TPMLimit            *int              `json:"tpm_limit,omitempty"`
	RPMLimit            *int              `json:"rpm_limit,omitempty"`
	TeamMemberRPMLimit  *int              `json:"team_member_rpm_limit,omitempty"`
	TeamMemberTPMLimit  *int              `json:"team_member_tpm_limit,omitempty"`
	MaxParallelRequests *int              `json:"max_parallel_requests,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Members             []MemberRequest   `json:"members_with_roles,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
	Guardrails          []string          `json:"guardrails,omitempty"`
}

// MemberRequest defines a member to add to a team.
type MemberRequest struct {
	UserEmail string `json:"user_email"`
	Role      string `json:"role"`
}

// TeamCreateResponse is the response from creating a team.
type TeamCreateResponse struct {
	TeamID    string `json:"team_id"`
	TeamAlias string `json:"team_alias"`
}

// TeamUpdateRequest is the request to update a team.
type TeamUpdateRequest struct {
	TeamID              string            `json:"team_id"`
	TeamAlias           string            `json:"team_alias,omitempty"`
	OrganizationID      string            `json:"organization_id,omitempty"`
	Models              []string          `json:"models,omitempty"`
	MaxBudget           *float64          `json:"max_budget,omitempty"`
	BudgetDuration      string            `json:"budget_duration,omitempty"`
	TPMLimit            *int              `json:"tpm_limit,omitempty"`
	RPMLimit            *int              `json:"rpm_limit,omitempty"`
	MaxParallelRequests *int              `json:"max_parallel_requests,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
	Guardrails          []string          `json:"guardrails,omitempty"`
}

// TeamInfo is the response from getting team info.
type TeamInfo struct {
	TeamID    string   `json:"team_id"`
	TeamAlias string   `json:"team_alias"`
	Models    []string `json:"models"`
	Spend     *float64 `json:"spend"`
}

// TeamMemberInfo describes a team member from the API.
type TeamMemberInfo struct {
	Email string `json:"user_email"`
	Role  string `json:"role"`
}

type teamService struct {
	client *httpClient
}

func (s *teamService) Create(ctx context.Context, req TeamCreateRequest) (*TeamCreateResponse, error) {
	var resp TeamCreateResponse
	err := s.client.do(ctx, http.MethodPost, "/team/new", req, &resp)
	return &resp, err
}

func (s *teamService) Update(ctx context.Context, req TeamUpdateRequest) error {
	return s.client.do(ctx, http.MethodPost, "/team/update", req, nil)
}

func (s *teamService) Delete(ctx context.Context, teamID string) error {
	body := map[string][]string{"team_ids": {teamID}}
	return s.client.do(ctx, http.MethodPost, "/team/delete", body, nil)
}

func (s *teamService) Get(ctx context.Context, teamID string) (*TeamInfo, error) {
	var resp TeamInfo
	err := s.client.do(ctx, http.MethodGet, "/team/info?team_id="+teamID, nil, &resp)
	return &resp, err
}

func (s *teamService) List(ctx context.Context) ([]TeamInfo, error) {
	var resp []TeamInfo
	err := s.client.do(ctx, http.MethodGet, "/team/list", nil, &resp)
	return resp, err
}

func (s *teamService) AddMember(ctx context.Context, teamID, email, role string) error {
	body := map[string]interface{}{
		"team_id": teamID,
		"member":  MemberRequest{UserEmail: email, Role: role},
	}
	return s.client.do(ctx, http.MethodPost, "/team/member_add", body, nil)
}

func (s *teamService) RemoveMember(ctx context.Context, teamID, email string) error {
	body := map[string]interface{}{
		"team_id":    teamID,
		"user_email": email,
	}
	return s.client.do(ctx, http.MethodPost, "/team/member_delete", body, nil)
}

func (s *teamService) ListMembers(ctx context.Context, teamID string) ([]TeamMemberInfo, error) {
	var resp struct {
		Members []TeamMemberInfo `json:"members"`
	}
	err := s.client.do(ctx, http.MethodGet, "/team/info?team_id="+teamID, nil, &resp)
	return resp.Members, err
}

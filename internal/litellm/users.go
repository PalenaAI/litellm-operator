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

// UserService defines operations on LiteLLM users.
type UserService interface {
	Create(ctx context.Context, req UserCreateRequest) (*UserCreateResponse, error)
	Update(ctx context.Context, req UserCreateRequest) error
	Delete(ctx context.Context, userID string) error
	Get(ctx context.Context, userID string) (*UserInfo, error)
	List(ctx context.Context) ([]UserInfo, error)
}

// UserCreateRequest is the request to create or update a user.
type UserCreateRequest struct {
	UserID         string            `json:"user_id"`
	UserEmail      string            `json:"user_email,omitempty"`
	UserRole       string            `json:"user_role,omitempty"`
	MaxBudget      *float64          `json:"max_budget,omitempty"`
	BudgetDuration string            `json:"budget_duration,omitempty"`
	Models         []string          `json:"models,omitempty"`
	Teams          []UserTeam        `json:"teams,omitempty"`
	TPMLimit       *int              `json:"tpm_limit,omitempty"`
	RPMLimit       *int              `json:"rpm_limit,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// UserTeam defines team membership for a user.
type UserTeam struct {
	TeamID          string   `json:"team_id"`
	Role            string   `json:"role,omitempty"`
	MaxBudgetInTeam *float64 `json:"max_budget_in_team,omitempty"`
}

// UserCreateResponse is the response from creating a user.
type UserCreateResponse struct {
	UserID string `json:"user_id"`
}

// UserInfo is the response from getting user info.
type UserInfo struct {
	UserID    string   `json:"user_id"`
	UserEmail string   `json:"user_email"`
	UserRole  string   `json:"user_role"`
	Spend     *float64 `json:"spend"`
	Teams     []string `json:"teams"`
}

type userService struct {
	client *httpClient
}

func (s *userService) Create(ctx context.Context, req UserCreateRequest) (*UserCreateResponse, error) {
	var resp UserCreateResponse
	err := s.client.do(ctx, http.MethodPost, "/user/new", req, &resp)
	return &resp, err
}

func (s *userService) Update(ctx context.Context, req UserCreateRequest) error {
	return s.client.do(ctx, http.MethodPost, "/user/update", req, nil)
}

func (s *userService) Delete(ctx context.Context, userID string) error {
	body := map[string][]string{"user_ids": {userID}}
	return s.client.do(ctx, http.MethodPost, "/user/delete", body, nil)
}

func (s *userService) Get(ctx context.Context, userID string) (*UserInfo, error) {
	var resp UserInfo
	err := s.client.do(ctx, http.MethodGet, "/user/info?user_id="+userID, nil, &resp)
	return &resp, err
}

func (s *userService) List(ctx context.Context) ([]UserInfo, error) {
	var resp []UserInfo
	err := s.client.do(ctx, http.MethodGet, "/user/list", nil, &resp)
	return resp, err
}

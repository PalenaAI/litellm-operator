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

// BudgetService defines operations on LiteLLM reusable budget tiers
// (/budget/* endpoints). A budget is referenced by other resources via
// budget_id (e.g. LiteLLMVirtualKey / LiteLLMCustomer budgetId).
type BudgetService interface {
	Create(ctx context.Context, req BudgetRequest) (*BudgetResponse, error)
	Update(ctx context.Context, req BudgetRequest) error
	Delete(ctx context.Context, budgetID string) error
	Get(ctx context.Context, budgetID string) (*BudgetInfo, error)
}

// BudgetRequest is the body for POST /budget/new and /budget/update
// (LiteLLM's BudgetNewRequest).
type BudgetRequest struct {
	BudgetID            string             `json:"budget_id,omitempty"`
	MaxBudget           *float64           `json:"max_budget,omitempty"`
	SoftBudget          *float64           `json:"soft_budget,omitempty"`
	BudgetDuration      string             `json:"budget_duration,omitempty"`
	TPMLimit            *int               `json:"tpm_limit,omitempty"`
	RPMLimit            *int               `json:"rpm_limit,omitempty"`
	MaxParallelRequests *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget      map[string]float64 `json:"model_max_budget,omitempty"`
}

// BudgetResponse is the response from creating/updating a budget.
type BudgetResponse struct {
	BudgetID string `json:"budget_id"`
}

// BudgetInfo is a single budget row returned by /budget/info.
type BudgetInfo struct {
	BudgetID  string   `json:"budget_id"`
	MaxBudget *float64 `json:"max_budget"`
	Spend     *float64 `json:"spend"`
}

type budgetService struct {
	client *httpClient
}

func (s *budgetService) Create(ctx context.Context, req BudgetRequest) (*BudgetResponse, error) {
	var resp BudgetResponse
	err := s.client.do(ctx, http.MethodPost, "/budget/new", req, &resp)
	return &resp, err
}

func (s *budgetService) Update(ctx context.Context, req BudgetRequest) error {
	return s.client.do(ctx, http.MethodPost, "/budget/update", req, nil)
}

func (s *budgetService) Delete(ctx context.Context, budgetID string) error {
	body := map[string]string{"id": budgetID}
	return s.client.do(ctx, http.MethodPost, "/budget/delete", body, nil)
}

// Get fetches a single budget via POST /budget/info, whose body is a list of
// budget IDs (`budgets`) and whose response is a list of budget rows.
func (s *budgetService) Get(ctx context.Context, budgetID string) (*BudgetInfo, error) {
	body := map[string]interface{}{"budgets": []string{budgetID}}
	var rows []BudgetInfo
	if err := s.client.do(ctx, http.MethodPost, "/budget/info", body, &rows); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].BudgetID == budgetID {
			return &rows[i], nil
		}
	}
	return nil, nil
}

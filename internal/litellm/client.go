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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the top-level interface for interacting with a LiteLLM instance.
type Client interface {
	Models() ModelService
	Teams() TeamService
	Users() UserService
	Keys() KeyService
	Organizations() OrganizationService
	Health() HealthService
}

// ClientFactory creates a new LiteLLM API client given an endpoint and master key.
type ClientFactory func(endpoint, masterKey string) Client

// NewClient creates a new LiteLLM API client.
func NewClient(endpoint, masterKey string) Client {
	return &httpClient{
		baseURL:   strings.TrimRight(endpoint, "/"),
		masterKey: masterKey,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

type httpClient struct {
	baseURL   string
	masterKey string
	http      *http.Client
}

func (c *httpClient) Models() ModelService               { return &modelService{c} }
func (c *httpClient) Teams() TeamService                 { return &teamService{c} }
func (c *httpClient) Users() UserService                 { return &userService{c} }
func (c *httpClient) Keys() KeyService                   { return &keyService{c} }
func (c *httpClient) Organizations() OrganizationService { return &organizationService{c} }
func (c *httpClient) Health() HealthService              { return &healthService{c} }

func (c *httpClient) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.masterKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Path:       path,
		}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// APIError represents an error response from the LiteLLM API.
type APIError struct {
	StatusCode int
	Message    string
	Path       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("LiteLLM API error %d on %s: %s", e.StatusCode, e.Path, e.Message)
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsTransient returns true if the error is likely transient (5xx).
func (e *APIError) IsTransient() bool {
	return e.StatusCode >= 500
}

// IsAPIError checks if an error is an APIError and returns it.
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

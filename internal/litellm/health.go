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

// HealthService defines health check operations.
type HealthService interface {
	CheckLiveness(ctx context.Context) error
	CheckReadiness(ctx context.Context) error
	// Readiness returns the structured readiness payload (status + db state).
	Readiness(ctx context.Context) (*ReadinessResponse, error)
	// CachePing actively tests the configured cache backend (e.g. Redis)
	// via GET /cache/ping. It returns an error when no cache is configured
	// (HTTP 503) or when the backend is unreachable.
	CachePing(ctx context.Context) (*CachePingResponse, error)
	// Check returns the per-model health list from GET /health. Used to
	// populate LiteLLMModel.status.health and operand performance metrics.
	Check(ctx context.Context) (*HealthCheckResponse, error)
}

// ReadinessResponse is the structured payload returned by /health/readiness.
//
// Note: this endpoint does NOT report Redis connectivity. As of LiteLLM
// 1.86.x the public /health/readiness was reduced to a minimal payload
// ({"status": ..., "db": ...}); the older `redis` boolean this operator once
// parsed never existed in any released shape, so Redis health must be probed
// separately via CachePing (GET /cache/ping). We tolerate extra fields and
// only extract what we need.
type ReadinessResponse struct {
	Status         string `json:"status,omitempty"`
	DBHealth       string `json:"db,omitempty"`
	LiteLLMVersion string `json:"litellm_version,omitempty"`
}

// CachePingResponse is the payload returned by GET /cache/ping. For a Redis
// backend LiteLLM calls `litellm.cache.ping()` and a test `async_add_cache()`,
// so a 200 response with Status == "healthy" is a genuine connectivity signal.
// When no cache is configured the endpoint returns HTTP 503 instead.
type CachePingResponse struct {
	Status           string `json:"status,omitempty"`
	CacheType        string `json:"cache_type,omitempty"`
	PingResponse     bool   `json:"ping_response,omitempty"`
	SetCacheResponse string `json:"set_cache_response,omitempty"`
}

// HealthCheckResponse is the payload returned by /health.
type HealthCheckResponse struct {
	HealthyEndpoints   []HealthEndpoint `json:"healthy_endpoints,omitempty"`
	UnhealthyEndpoints []HealthEndpoint `json:"unhealthy_endpoints,omitempty"`
	HealthyCount       int              `json:"healthy_count,omitempty"`
	UnhealthyCount     int              `json:"unhealthy_count,omitempty"`
}

// HealthEndpoint identifies a single model deployment in health output.
type HealthEndpoint struct {
	Model    string `json:"model,omitempty"`
	ModelID  string `json:"model_id,omitempty"`
	APIBase  string `json:"api_base,omitempty"`
	Error    string `json:"error,omitempty"`
	Response string `json:"response,omitempty"`
}

type healthService struct {
	client *httpClient
}

func (s *healthService) CheckLiveness(ctx context.Context) error {
	return s.client.do(ctx, http.MethodGet, "/health/liveliness", nil, nil)
}

func (s *healthService) CheckReadiness(ctx context.Context) error {
	return s.client.do(ctx, http.MethodGet, "/health/readiness", nil, nil)
}

func (s *healthService) Readiness(ctx context.Context) (*ReadinessResponse, error) {
	var resp ReadinessResponse
	if err := s.client.do(ctx, http.MethodGet, "/health/readiness", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *healthService) CachePing(ctx context.Context) (*CachePingResponse, error) {
	var resp CachePingResponse
	if err := s.client.do(ctx, http.MethodGet, "/cache/ping", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *healthService) Check(ctx context.Context) (*HealthCheckResponse, error) {
	var resp HealthCheckResponse
	if err := s.client.do(ctx, http.MethodGet, "/health", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

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

// CredentialService manages DB-backed credentials in the LiteLLM proxy.
// These are visible in the Admin UI and merged into models referenced via
// `litellm_credential_name` at request time.
//
// Verified endpoints (LiteLLM 1.86):
//   - POST   /credentials                  create
//   - PATCH  /credentials/{name}           update (both credential_name and credential_values required in body)
//   - DELETE /credentials/{name}           delete (idempotent — returns success even when not found)
//   - GET    /credentials/by_name/{name}   fetch one (404 when missing)
type CredentialService interface {
	Create(ctx context.Context, payload CredentialPayload) error
	Update(ctx context.Context, payload CredentialPayload) error
	Delete(ctx context.Context, credentialName string) error
	Get(ctx context.Context, credentialName string) (*CredentialInfoResponse, error)
}

// CredentialPayload is the request body shared by POST /credentials and
// PATCH /credentials/{name}. LiteLLM rejects partial PATCHes — both
// credential_name and credential_values must be present in either call.
type CredentialPayload struct {
	CredentialName   string                 `json:"credential_name"`
	CredentialValues map[string]interface{} `json:"credential_values"`
	CredentialInfo   map[string]interface{} `json:"credential_info,omitempty"`
}

// CredentialInfoResponse is the response from GET /credentials/by_name/{name}.
// LiteLLM masks api_key values (e.g. "81****oT"), so the response can be used
// for existence checks but not for hash comparison against unencrypted values.
type CredentialInfoResponse struct {
	CredentialName   string                 `json:"credential_name"`
	CredentialValues map[string]interface{} `json:"credential_values,omitempty"`
	CredentialInfo   map[string]interface{} `json:"credential_info,omitempty"`
}

type credentialService struct {
	client *httpClient
}

func (s *credentialService) Create(ctx context.Context, payload CredentialPayload) error {
	return s.client.do(ctx, http.MethodPost, "/credentials", payload, nil)
}

func (s *credentialService) Update(ctx context.Context, payload CredentialPayload) error {
	return s.client.do(ctx, http.MethodPatch, "/credentials/"+payload.CredentialName, payload, nil)
}

func (s *credentialService) Delete(ctx context.Context, credentialName string) error {
	return s.client.do(ctx, http.MethodDelete, "/credentials/"+credentialName, nil, nil)
}

func (s *credentialService) Get(ctx context.Context, credentialName string) (*CredentialInfoResponse, error) {
	var resp CredentialInfoResponse
	if err := s.client.do(ctx, http.MethodGet, "/credentials/by_name/"+credentialName, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

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

package controller

import (
	"testing"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// TestCredentialPlanAction guards the fix for the proxy crash-loop: the
// create-vs-update decision must key off the sync-hash annotation, NOT
// Status.Configured. A credential that has already been pushed (annotation set)
// must never be re-created even when Status.Configured is false — re-POSTing an
// existing credential trips LiteLLM's unique constraint on credential_name.
func TestCredentialPlanAction(t *testing.T) {
	r := &LiteLLMCredentialReconciler{}
	credWith := func(hash string, configured bool) *litellmv1alpha1.LiteLLMCredential {
		c := &litellmv1alpha1.LiteLLMCredential{}
		if hash != "" {
			c.Annotations = map[string]string{AnnotationSyncHash: hash}
		}
		c.Status.Configured = configured
		return c
	}

	cases := []struct {
		name string
		cred *litellmv1alpha1.LiteLLMCredential
		hash string
		want credentialAction
	}{
		{"never synced (no annotation)", credWith("", false), "h1", actionCreate},
		{"synced, hash matches, Status.Configured true", credWith("h1", true), "h1", actionNoop},
		// The regression: annotation set + hash matches, but Status.Configured
		// was lost. Must be a no-op, NOT a re-create.
		{"synced, hash matches, Status.Configured FALSE", credWith("h1", false), "h1", actionNoop},
		{"synced, hash differs", credWith("h1", true), "h2", actionUpdate},
		{"synced, hash differs, Status.Configured false", credWith("h1", false), "h2", actionUpdate},
	}
	for _, tc := range cases {
		if got := r.planAction(tc.cred, tc.hash); got != tc.want {
			t.Errorf("%s: planAction = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestIsCredentialConflict covers the create-conflict detection. LiteLLM
// surfaces "credential already exists" as a 400/409, or — when the Prisma
// unique constraint trips — as a 500 whose body carries the constraint message.
func TestIsCredentialConflict(t *testing.T) {
	uniqueBody := "Unique constraint failed on the fields: (`credential_name`)"
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"400", &litellm.APIError{StatusCode: 400, Message: "bad request"}, true},
		{"409", &litellm.APIError{StatusCode: 409, Message: "conflict"}, true},
		{"500 unique constraint", &litellm.APIError{StatusCode: 500, Message: uniqueBody}, true},
		{"500 already exists text", &litellm.APIError{StatusCode: 500, Message: "credential already exists"}, true},
		{"500 generic", &litellm.APIError{StatusCode: 500, Message: "internal error"}, false},
		{"404", &litellm.APIError{StatusCode: 404, Message: "not found"}, false},
		{"non-api error", errFake("boom"), false},
	}
	for _, tc := range cases {
		if got := isCredentialConflict(tc.err); got != tc.want {
			t.Errorf("%s: isCredentialConflict = %v, want %v", tc.name, got, tc.want)
		}
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

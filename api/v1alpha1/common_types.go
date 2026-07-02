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

package v1alpha1

import (
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// DecodeJSONParams decodes a map of raw-JSON free-form params into a map of Go
// values (bool/float64/string/nested), suitable for merging into an API payload.
// The apiserver validates that each value is well-formed JSON, so a decode error
// here is not expected; such an entry is skipped rather than aborting the merge.
func DecodeJSONParams(m map[string]apiextensionsv1.JSON) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if len(v.Raw) == 0 {
			continue
		}
		var val interface{}
		if err := json.Unmarshal(v.Raw, &val); err != nil {
			continue
		}
		out[k] = val
	}
	return out
}

// ObjectPermission restricts which objects (MCP servers, vector stores, agents,
// access groups) a team, user, key, organization, or customer may access. The
// referenced objects are named by ID/name; this grants access, it does not
// define them.
type ObjectPermission struct {
	// Allowed MCP servers (by name/ID).
	// +optional
	MCPServers []string `json:"mcpServers,omitempty"`

	// Allowed MCP access groups.
	// +optional
	AccessGroups []string `json:"accessGroups,omitempty"`

	// Allowed vector stores (by name/ID).
	// +optional
	VectorStores []string `json:"vectorStores,omitempty"`

	// Allowed agents (by name/ID).
	// +optional
	Agents []string `json:"agents,omitempty"`
}

// SecretKeyRef references a key within a Kubernetes Secret.
type SecretKeyRef struct {
	// Name of the Secret.
	Name string `json:"name"`
	// Key within the Secret.
	Key string `json:"key"`
}

// SecretRef references a Kubernetes Secret by name.
type SecretRef struct {
	// Name of the Secret.
	Name string `json:"name"`
}

// InstanceRef references a LiteLLMInstance in the same namespace.
type InstanceRef struct {
	// Name of the LiteLLMInstance CR.
	Name string `json:"name"`
}

// OrganizationRef references a LiteLLMOrganization CR in the same namespace.
type OrganizationRef struct {
	// Name of the LiteLLMOrganization CR.
	Name string `json:"name"`
}

// CredentialRef references a LiteLLMCredential CR in the same namespace.
type CredentialRef struct {
	// Name of the LiteLLMCredential CR.
	Name string `json:"name"`
}

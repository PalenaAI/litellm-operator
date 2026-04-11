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

package v1alpha1

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

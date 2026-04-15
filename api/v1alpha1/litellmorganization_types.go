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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMOrganizationSpec defines the desired state of LiteLLMOrganization.
type LiteLLMOrganizationSpec struct {
	// Reference to the LiteLLMInstance this organization belongs to.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Human-readable alias for the organization.
	OrganizationAlias string `json:"organizationAlias"`

	// List of models this organization can access.
	// +optional
	Models []string `json:"models,omitempty"`

	// Maximum budget in USD.
	// +optional
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Budget reset interval (e.g., "1d", "7d", "30d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// TPM limit for the organization.
	// +optional
	TpmLimit *int64 `json:"tpmLimit,omitempty"`

	// RPM limit for the organization.
	// +optional
	RpmLimit *int64 `json:"rpmLimit,omitempty"`

	// Organization members.
	// +optional
	Members []OrganizationMember `json:"members,omitempty"`

	// Metadata key-value pairs.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OrganizationMember defines a member of an organization.
type OrganizationMember struct {
	// User email address.
	Email string `json:"email"`

	// Role within the organization: org_admin or internal_user.
	// +kubebuilder:validation:Enum=org_admin;internal_user
	// +kubebuilder:default="internal_user"
	Role string `json:"role,omitempty"`
}

// LiteLLMOrganizationStatus defines the observed state of LiteLLMOrganization.
type LiteLLMOrganizationStatus struct {
	// Whether the organization is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// Organization ID assigned by LiteLLM.
	// +optional
	LiteLLMOrganizationID string `json:"litellmOrganizationId,omitempty"`

	// Current spend in USD.
	// +optional
	CurrentSpend *float64 `json:"currentSpend,omitempty"`

	// Number of members.
	// +optional
	MemberCount int `json:"memberCount,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lo
// +kubebuilder:printcolumn:name="Alias",type="string",JSONPath=".spec.organizationAlias"
// +kubebuilder:printcolumn:name="Members",type="integer",JSONPath=".status.memberCount"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Organization"

// LiteLLMOrganization is the Schema for the litellmorganizations API.
type LiteLLMOrganization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMOrganizationSpec   `json:"spec,omitempty"`
	Status LiteLLMOrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMOrganizationList contains a list of LiteLLMOrganization.
type LiteLLMOrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMOrganization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMOrganization{}, &LiteLLMOrganizationList{})
}

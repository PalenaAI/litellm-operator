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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMTeamSpec defines the desired state of LiteLLMTeam.
type LiteLLMTeamSpec struct {
	// Reference to the LiteLLMInstance.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Reference to the organization this team belongs to.
	// +optional
	OrganizationRef *OrganizationRef `json:"organizationRef,omitempty"`

	// Human-readable team alias.
	TeamAlias string `json:"teamAlias"`

	// Models this team can access.
	// +optional
	Models []string `json:"models,omitempty"`

	// Maximum monthly budget in USD.
	// +optional
	MaxBudgetMonthly *float64 `json:"maxBudgetMonthly,omitempty"`

	// Budget reset duration (e.g., "30d", "7d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// TPM limit for the team.
	// +optional
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// RPM limit for the team.
	// +optional
	RPMLimit *int `json:"rpmLimit,omitempty"`

	// RPM limit per team member.
	// +optional
	TeamMemberRPMLimit *int `json:"teamMemberRpmLimit,omitempty"`

	// TPM limit per team member.
	// +optional
	TeamMemberTPMLimit *int `json:"teamMemberTpmLimit,omitempty"`

	// Custom metadata for the team.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Tags associated with this team. Keys generated for team members
	// will inherit these tags for tag-based routing.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Maximum concurrent requests for this team.
	// +optional
	MaxParallelRequests *int `json:"maxParallelRequests,omitempty"`

	// Controls who owns team membership.
	//   "crd"   — CRD is authoritative. Only listed members exist.
	//   "sso"   — IdP is authoritative. spec.members is ignored.
	//   "mixed" — CRD members are additive. SSO members are preserved.
	// +kubebuilder:validation:Enum=crd;sso;mixed
	// +kubebuilder:default="mixed"
	MemberManagement string `json:"memberManagement,omitempty"`

	// Team members. Behavior depends on memberManagement mode.
	// Ignored when memberManagement is "sso".
	// +optional
	Members []TeamMember `json:"members,omitempty"`

	// Guardrails to activate for this team. Each entry must match the
	// guardrailName of a LiteLLMGuardrail CR bound to the same instance.
	// Requires a LiteLLM Enterprise license.
	// +optional
	Guardrails []string `json:"guardrails,omitempty"`
}

// TeamMember defines a team member.
type TeamMember struct {
	// User email address.
	Email string `json:"email"`

	// Role within the team.
	// +kubebuilder:validation:Enum=admin;user
	// +kubebuilder:default="user"
	Role string `json:"role,omitempty"`
}

// LiteLLMTeamStatus defines the observed state of LiteLLMTeam.
type LiteLLMTeamStatus struct {
	// Whether the team is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// LiteLLM-assigned team ID.
	LiteLLMTeamID string `json:"litellmTeamId,omitempty"`

	// Current spend in USD.
	// +optional
	CurrentSpend *float64 `json:"currentSpend,omitempty"`

	// Total member count (CRD + SSO).
	TotalMemberCount int `json:"totalMemberCount,omitempty"`

	// Members managed by this CRD.
	// +optional
	CRDMembers []TeamMemberStatus `json:"crdMembers,omitempty"`

	// Members provisioned by SSO/SCIM (not managed by CRD).
	// +optional
	SSOMembers []TeamMemberStatus `json:"ssoMembers,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// TeamMemberStatus defines the status of a team member.
type TeamMemberStatus struct {
	// Member email.
	Email string `json:"email"`

	// Member role.
	Role string `json:"role"`

	// Source of the member ("crd", "azure-entra", "okta", etc.).
	// +optional
	Source string `json:"source,omitempty"`

	// Whether the member is synced.
	Synced bool `json:"synced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lt
// +kubebuilder:printcolumn:name="Alias",type="string",JSONPath=".spec.teamAlias"
// +kubebuilder:printcolumn:name="Members",type="integer",JSONPath=".status.totalMemberCount"
// +kubebuilder:printcolumn:name="MemberMgmt",type="string",JSONPath=".spec.memberManagement"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Team"

// LiteLLMTeam is the Schema for the litellmteams API.
type LiteLLMTeam struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMTeamSpec   `json:"spec,omitempty"`
	Status LiteLLMTeamStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMTeamList contains a list of LiteLLMTeam.
type LiteLLMTeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMTeam `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMTeam{}, &LiteLLMTeamList{})
}

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

// LiteLLMTeamSpec defines the desired state of LiteLLMTeam.
type LiteLLMTeamSpec struct {
	// Reference to the LiteLLMInstance.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Instance Ref"
	InstanceRef InstanceRef `json:"instanceRef"`

	// Reference to the organization this team belongs to.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Organization Ref"
	OrganizationRef *OrganizationRef `json:"organizationRef,omitempty"`

	// Human-readable team alias.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Team Alias"
	TeamAlias string `json:"teamAlias"`

	// Models this team can access.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Models"
	Models []string `json:"models,omitempty"`

	// Maximum monthly budget in USD.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Budget Monthly"
	MaxBudgetMonthly *float64 `json:"maxBudgetMonthly,omitempty"`

	// Budget reset duration (e.g., "30d", "7d").
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Budget Duration"
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// TPM limit for the team.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TPM Limit"
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// RPM limit for the team.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RPM Limit"
	RPMLimit *int `json:"rpmLimit,omitempty"`

	// RPM limit per team member.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Team Member RPM Limit"
	TeamMemberRPMLimit *int `json:"teamMemberRpmLimit,omitempty"`

	// TPM limit per team member.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Team Member TPM Limit"
	TeamMemberTPMLimit *int `json:"teamMemberTpmLimit,omitempty"`

	// Custom metadata for the team.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metadata"
	Metadata map[string]string `json:"metadata,omitempty"`

	// Tags associated with this team. Keys generated for team members
	// will inherit these tags for tag-based routing.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tags"
	Tags []string `json:"tags,omitempty"`

	// Maximum concurrent requests for this team.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Parallel Requests"
	MaxParallelRequests *int `json:"maxParallelRequests,omitempty"`

	// Controls who owns team membership.
	//   "crd"   — CRD is authoritative. Only listed members exist.
	//   "sso"   — IdP is authoritative. spec.members is ignored.
	//   "mixed" — CRD members are additive. SSO members are preserved.
	// +kubebuilder:validation:Enum=crd;sso;mixed
	// +kubebuilder:default="mixed"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Member Management"
	MemberManagement string `json:"memberManagement,omitempty"`

	// Team members. Behavior depends on memberManagement mode.
	// Ignored when memberManagement is "sso".
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Members"
	Members []TeamMember `json:"members,omitempty"`

	// Guardrails to activate for this team. Each entry must match the
	// guardrailName of a LiteLLMGuardrail CR bound to the same instance.
	// Requires a LiteLLM Enterprise license.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Guardrails"
	Guardrails []string `json:"guardrails,omitempty"`

	// Logging configuration for this team (enterprise).
	// Enables per-team logging destinations and GDPR logging disable.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Logging"
	Logging *TeamLoggingSpec `json:"logging,omitempty"`
}

// TeamLoggingSpec configures per-team logging behavior.
type TeamLoggingSpec struct {
	// Disable all logging for this team (GDPR compliance).
	// When true, no request/response data is logged for this team's requests.
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// Team-specific logging callbacks.
	// Each callback routes logs to a separate provider instance (e.g., a
	// dedicated Langfuse project for this team).
	// +optional
	Callbacks []TeamCallback `json:"callbacks,omitempty"`
}

// TeamCallback defines a per-team logging callback destination.
type TeamCallback struct {
	// Callback provider name.
	// +kubebuilder:validation:Enum=langfuse;gcs_bucket;langsmith;arize
	Name string `json:"name"`

	// Callback type: when to invoke the callback.
	// +kubebuilder:validation:Enum=success;failure;success_and_failure
	// +kubebuilder:default="success_and_failure"
	// +optional
	Type string `json:"type,omitempty"`

	// Reference to a Secret containing provider credentials.
	// Expected keys depend on the provider:
	//   langfuse:    langfuse_public_key, langfuse_secret, langfuse_host (optional)
	//   gcs_bucket:  gcs_bucket_name, gcs_path_service_account (optional)
	//   langsmith:   langsmith_api_key, langsmith_project
	//   arize:       arize_api_key, arize_space_key
	CredentialsSecretRef SecretRef `json:"credentialsSecretRef"`

	// Provider-specific configuration (e.g., host URL, bucket name).
	// These are merged into callback_vars alongside the Secret data.
	// +optional
	Config map[string]string `json:"config,omitempty"`
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

	// Whether team logging callbacks are synced.
	// +optional
	LoggingSynced bool `json:"loggingSynced,omitempty"`

	// Whether logging is disabled for this team (GDPR).
	// +optional
	LoggingDisabled bool `json:"loggingDisabled,omitempty"`

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
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Team",resources={{LiteLLMInstance,v1alpha1,""},{LiteLLMOrganization,v1alpha1,""}}

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

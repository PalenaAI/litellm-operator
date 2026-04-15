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

// LiteLLMUserSpec defines the desired state of LiteLLMUser.
type LiteLLMUserSpec struct {
	// Reference to the LiteLLMInstance.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Unique user identifier (typically email).
	UserID string `json:"userId"`

	// User email address.
	// +optional
	UserEmail string `json:"userEmail,omitempty"`

	// User role.
	// +kubebuilder:validation:Enum=proxy_admin;proxy_admin_viewer;internal_user;internal_user_viewer
	// +kubebuilder:default="internal_user"
	UserRole string `json:"userRole,omitempty"`

	// Maximum budget in USD.
	// +optional
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Budget reset duration (e.g., "30d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Models this user can access.
	// +optional
	Models []string `json:"models,omitempty"`

	// Teams this user belongs to.
	// +optional
	Teams []UserTeamMembership `json:"teams,omitempty"`

	// TPM limit for this user.
	// +optional
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// RPM limit for this user.
	// +optional
	RPMLimit *int `json:"rpmLimit,omitempty"`

	// Custom metadata.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UserTeamMembership defines a user's team membership.
type UserTeamMembership struct {
	// Reference to a LiteLLMTeam CR.
	// +optional
	TeamRef *InstanceRef `json:"teamRef,omitempty"`

	// Direct team ID (used when team is not managed by a CRD).
	// +optional
	TeamID string `json:"teamId,omitempty"`

	// Role within the team.
	// +kubebuilder:default="user"
	Role string `json:"role,omitempty"`

	// Max budget within this team.
	// +optional
	MaxBudgetInTeam *float64 `json:"maxBudgetInTeam,omitempty"`
}

// LiteLLMUserStatus defines the observed state of LiteLLMUser.
type LiteLLMUserStatus struct {
	// Whether the user is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// LiteLLM-assigned user ID.
	LiteLLMUserID string `json:"litellmUserId,omitempty"`

	// Current spend in USD.
	// +optional
	CurrentSpend *float64 `json:"currentSpend,omitempty"`

	// Teams the user is a member of (resolved).
	// +optional
	ResolvedTeams []ResolvedTeamMembership `json:"resolvedTeams,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResolvedTeamMembership defines a resolved team membership.
type ResolvedTeamMembership struct {
	// Team ID.
	TeamID string `json:"teamId"`

	// Team alias.
	// +optional
	TeamAlias string `json:"teamAlias,omitempty"`

	// Role within the team.
	Role string `json:"role"`

	// Whether the membership is synced.
	Synced bool `json:"synced"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lu
// +kubebuilder:printcolumn:name="UserID",type="string",JSONPath=".spec.userId"
// +kubebuilder:printcolumn:name="Role",type="string",JSONPath=".spec.userRole"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM User"

// LiteLLMUser is the Schema for the litellmusers API.
type LiteLLMUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMUserSpec   `json:"spec,omitempty"`
	Status LiteLLMUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMUserList contains a list of LiteLLMUser.
type LiteLLMUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMUser{}, &LiteLLMUserList{})
}

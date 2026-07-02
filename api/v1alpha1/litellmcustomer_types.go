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

// LiteLLMCustomerSpec defines the desired state of LiteLLMCustomer.
//
// A Customer represents an external end-user of the LiteLLM AI gateway —
// typically the user of an application built on top of LiteLLM. Customers
// are identified by LiteLLM as "end_users" and can have their own budgets,
// rate limits, and model access policies.
type LiteLLMCustomerSpec struct {
	// Reference to the LiteLLMInstance this customer belongs to.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Instance Ref"
	InstanceRef InstanceRef `json:"instanceRef"`

	// Unique customer identifier (typically an external user ID).
	// Maps to LiteLLM's `user_id` / `end_user_id` field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Customer ID"
	CustomerID string `json:"customerId"`

	// Human-readable alias for the customer.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Alias"
	Alias string `json:"alias,omitempty"`

	// Maximum budget in USD.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Budget"
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Budget reset duration (e.g., "1d", "7d", "30d").
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Budget Duration"
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Reference to a predefined budget tier by budget_id.
	// Mutually exclusive with MaxBudget.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Budget ID"
	BudgetID string `json:"budgetId,omitempty"`

	// TPM (tokens-per-minute) limit for this customer.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TPM Limit"
	TpmLimit *int64 `json:"tpmLimit,omitempty"`

	// RPM (requests-per-minute) limit for this customer.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RPM Limit"
	RpmLimit *int64 `json:"rpmLimit,omitempty"`

	// List of models this customer is allowed to access.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Allowed Model Region"
	AllowedModelRegion string `json:"allowedModelRegion,omitempty"`

	// Default model to use for this customer.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Default Model"
	DefaultModel string `json:"defaultModel,omitempty"`

	// List of models this customer is allowed to access.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Models"
	Models []string `json:"models,omitempty"`

	// Whether the customer is blocked from making requests.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Blocked"
	Blocked *bool `json:"blocked,omitempty"`

	// Object permissions (MCP servers, vector stores, agents, access groups).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Object Permission"
	ObjectPermission *CustomerObjectPermission `json:"objectPermission,omitempty"`

	// Metadata key-value pairs stored with the customer.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metadata"
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CustomerObjectPermission is retained for backward compatibility; it is an
// alias for the shared [ObjectPermission] type now used across all identity CRDs.
type CustomerObjectPermission = ObjectPermission

// LiteLLMCustomerStatus defines the observed state of LiteLLMCustomer.
type LiteLLMCustomerStatus struct {
	// Whether the customer is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// Current spend in USD, refreshed from /customer/info.
	// +optional
	CurrentSpend *float64 `json:"currentSpend,omitempty"`

	// Whether the customer is blocked (as reported by LiteLLM).
	// +optional
	Blocked bool `json:"blocked,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lcust
// +kubebuilder:printcolumn:name="CustomerID",type="string",JSONPath=".spec.customerId"
// +kubebuilder:printcolumn:name="MaxBudget",type="string",JSONPath=".spec.maxBudget"
// +kubebuilder:printcolumn:name="Spend",type="string",JSONPath=".status.currentSpend"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Customer",resources={{LiteLLMInstance,v1alpha1,""}}

// LiteLLMCustomer is the Schema for the litellmcustomers API.
// A Customer represents an external end-user of the AI gateway with its own
// spending, rate, and access policies.
type LiteLLMCustomer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMCustomerSpec   `json:"spec,omitempty"`
	Status LiteLLMCustomerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMCustomerList contains a list of LiteLLMCustomer.
type LiteLLMCustomerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMCustomer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMCustomer{}, &LiteLLMCustomerList{})
}

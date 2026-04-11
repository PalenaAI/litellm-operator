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

// LiteLLMCustomerSpec defines the desired state of LiteLLMCustomer.
//
// A Customer represents an external end-user of the LiteLLM AI gateway —
// typically the user of an application built on top of LiteLLM. Customers
// are identified by LiteLLM as "end_users" and can have their own budgets,
// rate limits, and model access policies.
type LiteLLMCustomerSpec struct {
	// Reference to the LiteLLMInstance this customer belongs to.
	InstanceRef InstanceRef `json:"instanceRef"`

	// Unique customer identifier (typically an external user ID).
	// Maps to LiteLLM's `user_id` / `end_user_id` field.
	CustomerID string `json:"customerId"`

	// Human-readable alias for the customer.
	// +optional
	Alias string `json:"alias,omitempty"`

	// Maximum budget in USD.
	// +optional
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Budget reset duration (e.g., "1d", "7d", "30d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Reference to a predefined budget tier by budget_id.
	// Mutually exclusive with MaxBudget.
	// +optional
	BudgetID string `json:"budgetId,omitempty"`

	// TPM (tokens-per-minute) limit for this customer.
	// +optional
	TpmLimit *int64 `json:"tpmLimit,omitempty"`

	// RPM (requests-per-minute) limit for this customer.
	// +optional
	RpmLimit *int64 `json:"rpmLimit,omitempty"`

	// List of models this customer is allowed to access.
	// +optional
	AllowedModelRegion string `json:"allowedModelRegion,omitempty"`

	// Default model to use for this customer.
	// +optional
	DefaultModel string `json:"defaultModel,omitempty"`

	// List of models this customer is allowed to access.
	// +optional
	Models []string `json:"models,omitempty"`

	// Whether the customer is blocked from making requests.
	// +optional
	Blocked *bool `json:"blocked,omitempty"`

	// Object permissions (MCP servers, vector stores, agents, access groups).
	// +optional
	ObjectPermission *CustomerObjectPermission `json:"objectPermission,omitempty"`

	// Metadata key-value pairs stored with the customer.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CustomerObjectPermission restricts which objects a customer may access.
type CustomerObjectPermission struct {
	// Allowed MCP servers.
	// +optional
	MCPServers []string `json:"mcpServers,omitempty"`

	// Allowed access groups.
	// +optional
	AccessGroups []string `json:"accessGroups,omitempty"`

	// Allowed vector stores.
	// +optional
	VectorStores []string `json:"vectorStores,omitempty"`

	// Allowed agents.
	// +optional
	Agents []string `json:"agents,omitempty"`
}

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

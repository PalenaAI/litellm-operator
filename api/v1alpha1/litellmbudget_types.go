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

// LiteLLMBudgetSpec defines the desired state of LiteLLMBudget.
//
// A LiteLLMBudget is a reusable budget / rate-limit tier registered with the
// proxy via /budget/new. Other resources reference it by budget_id — e.g.
// LiteLLMVirtualKey.spec.budgetId or LiteLLMInstance.spec.defaultCustomerBudget
// — instead of repeating inline limits. Managed via the LiteLLM REST API.
type LiteLLMBudgetSpec struct {
	// Reference to the LiteLLMInstance this budget belongs to.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Instance Ref"
	InstanceRef InstanceRef `json:"instanceRef"`

	// Stable budget identifier used by other resources to reference this tier
	// (sent as budget_id to LiteLLM). Defaults to the object's name when empty.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Budget ID"
	BudgetID string `json:"budgetId,omitempty"`

	// Maximum budget in USD for this tier.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Budget"
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Soft budget alert threshold in USD (below maxBudget; does not block).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Soft Budget"
	SoftBudget *float64 `json:"softBudget,omitempty"`

	// Budget reset interval (e.g., "1d", "7d", "30d").
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Budget Duration"
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Tokens-per-minute limit for this tier.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TPM Limit"
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// Requests-per-minute limit for this tier.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RPM Limit"
	RPMLimit *int `json:"rpmLimit,omitempty"`

	// Maximum concurrent requests for this tier.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Max Parallel Requests"
	MaxParallelRequests *int `json:"maxParallelRequests,omitempty"`

	// Per-model budget caps in USD (model name -> max budget).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Max Budget"
	ModelMaxBudget map[string]float64 `json:"modelMaxBudget,omitempty"`
}

// LiteLLMBudgetStatus defines the observed state of LiteLLMBudget.
type LiteLLMBudgetStatus struct {
	// Whether the budget is synced to LiteLLM.
	Synced bool `json:"synced,omitempty"`

	// The budget_id assigned/used in LiteLLM.
	LiteLLMBudgetID string `json:"litellmBudgetId,omitempty"`

	// Current spend against this budget in USD, refreshed from /budget/info.
	// +optional
	CurrentSpend *float64 `json:"currentSpend,omitempty"`

	// Last successful sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lb
// +kubebuilder:printcolumn:name="Budget ID",type="string",JSONPath=".status.litellmBudgetId"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Instance",type="string",JSONPath=".spec.instanceRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Budget",resources={{LiteLLMInstance,v1alpha1,""}}

// LiteLLMBudget is the Schema for the litellmbudgets API.
type LiteLLMBudget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMBudgetSpec   `json:"spec,omitempty"`
	Status LiteLLMBudgetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMBudgetList contains a list of LiteLLMBudget.
type LiteLLMBudgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMBudget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMBudget{}, &LiteLLMBudgetList{})
}

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

package controller

import (
	"testing"
)

func TestParseModelMaxBudget_Nil(t *testing.T) {
	result := parseModelMaxBudget(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseModelMaxBudget_Empty(t *testing.T) {
	result := parseModelMaxBudget(map[string]string{})
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseModelMaxBudget_ValidEntries(t *testing.T) {
	input := map[string]string{
		"gpt-4":         "100.00",
		"claude-3-opus": "50.50",
	}
	result := parseModelMaxBudget(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result["gpt-4"] != 100.00 {
		t.Errorf("expected gpt-4=100.00, got %v", result["gpt-4"])
	}
	if result["claude-3-opus"] != 50.50 {
		t.Errorf("expected claude-3-opus=50.50, got %v", result["claude-3-opus"])
	}
}

func TestParseModelMaxBudget_InvalidEntriesSkipped(t *testing.T) {
	input := map[string]string{
		"gpt-4":   "100.00",
		"invalid": "not-a-number",
	}
	result := parseModelMaxBudget(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(result))
	}
	if result["gpt-4"] != 100.00 {
		t.Errorf("expected gpt-4=100.00, got %v", result["gpt-4"])
	}
}

func TestParseModelMaxBudget_AllInvalid(t *testing.T) {
	input := map[string]string{
		"bad1": "abc",
		"bad2": "",
	}
	result := parseModelMaxBudget(input)
	if result != nil {
		t.Errorf("expected nil when all entries are invalid, got %v", result)
	}
}

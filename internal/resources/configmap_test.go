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

package resources

import (
	"testing"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

func intPtr(v int) *int { return &v }

func TestGenerateProxyConfig_DefaultFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		DefaultFallbacks: []string{"gpt-4-mini", "claude-3-haiku"},
	}

	config := GenerateProxyConfig(instance)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	df, ok := ls["default_fallbacks"].([]string)
	if !ok {
		t.Fatal("expected default_fallbacks to be []string")
	}
	if len(df) != 2 || df[0] != "gpt-4-mini" || df[1] != "claude-3-haiku" {
		t.Errorf("unexpected default_fallbacks: %v", df)
	}
}

func TestGenerateProxyConfig_ModelFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		ModelFallbacks: []litellmv1alpha1.ModelFallbackEntry{
			{Model: "gpt-4", Fallbacks: []string{"gpt-4-mini", "claude-3-haiku"}},
		},
	}

	config := GenerateProxyConfig(instance)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	fallbacks, ok := rs["fallbacks"].([]map[string][]string)
	if !ok {
		t.Fatal("expected fallbacks to be []map[string][]string")
	}
	if len(fallbacks) != 1 {
		t.Fatalf("expected 1 fallback entry, got %d", len(fallbacks))
	}
	models, ok := fallbacks[0]["gpt-4"]
	if !ok {
		t.Fatal("expected fallback entry for gpt-4")
	}
	if len(models) != 2 || models[0] != "gpt-4-mini" || models[1] != "claude-3-haiku" {
		t.Errorf("unexpected fallback models: %v", models)
	}
}

func TestGenerateProxyConfig_ContentPolicyFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		ContentPolicyFallbacks: []litellmv1alpha1.ModelFallbackEntry{
			{Model: "gpt-4", Fallbacks: []string{"claude-3-sonnet"}},
		},
	}

	config := GenerateProxyConfig(instance)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	cpf, ok := ls["content_policy_fallbacks"].([]map[string][]string)
	if !ok {
		t.Fatal("expected content_policy_fallbacks to be []map[string][]string")
	}
	if len(cpf) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cpf))
	}
	if models := cpf[0]["gpt-4"]; len(models) != 1 || models[0] != "claude-3-sonnet" {
		t.Errorf("unexpected content_policy_fallbacks: %v", cpf)
	}
}

func TestGenerateProxyConfig_ContextWindowFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		ContextWindowFallbacks: []litellmv1alpha1.ModelFallbackEntry{
			{Model: "gpt-4", Fallbacks: []string{"gpt-4-32k", "claude-3-sonnet"}},
		},
	}

	config := GenerateProxyConfig(instance)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	cwf, ok := ls["context_window_fallbacks"].([]map[string][]string)
	if !ok {
		t.Fatal("expected context_window_fallbacks to be []map[string][]string")
	}
	if len(cwf) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cwf))
	}
	models := cwf[0]["gpt-4"]
	if len(models) != 2 || models[0] != "gpt-4-32k" || models[1] != "claude-3-sonnet" {
		t.Errorf("unexpected context_window_fallbacks: %v", cwf)
	}
}

func TestGenerateProxyConfig_MaxFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		MaxFallbacks: intPtr(5),
	}

	config := GenerateProxyConfig(instance)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	if mf, ok := rs["max_fallbacks"].(int); !ok || mf != 5 {
		t.Errorf("expected max_fallbacks=5, got %v", rs["max_fallbacks"])
	}
}

func TestGenerateProxyConfig_RetryPolicy(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		RetryPolicy: map[string]int{
			"TimeoutError":   2,
			"RateLimitError": 3,
		},
	}

	config := GenerateProxyConfig(instance)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	rp, ok := rs["retry_policy"].(map[string]int)
	if !ok {
		t.Fatal("expected retry_policy to be map[string]int")
	}
	if rp["TimeoutError"] != 2 {
		t.Errorf("expected TimeoutError=2, got %d", rp["TimeoutError"])
	}
	if rp["RateLimitError"] != 3 {
		t.Errorf("expected RateLimitError=3, got %d", rp["RateLimitError"])
	}
}

func TestGenerateProxyConfig_ModelGroupRetryPolicy(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		ModelGroupRetryPolicy: map[string]map[string]int{
			"gpt-4": {
				"TimeoutError":   1,
				"RateLimitError": 0,
			},
		},
	}

	config := GenerateProxyConfig(instance)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	mgrp, ok := rs["model_group_retry_policy"].(map[string]map[string]int)
	if !ok {
		t.Fatal("expected model_group_retry_policy")
	}
	gpt4 := mgrp["gpt-4"]
	if gpt4["TimeoutError"] != 1 || gpt4["RateLimitError"] != 0 {
		t.Errorf("unexpected model_group_retry_policy for gpt-4: %v", gpt4)
	}
}

func TestGenerateProxyConfig_FallbacksWithExistingSettings(t *testing.T) {
	instance := newTestInstance()
	// Set callbacks (populates litellm_settings) and router settings simultaneously
	instance.Spec.Callbacks = &litellmv1alpha1.CallbacksSpec{
		Types: []string{"langfuse"},
	}
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		RoutingStrategy: "least-busy",
	}
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		DefaultFallbacks: []string{"gpt-4-mini"},
		ModelFallbacks: []litellmv1alpha1.ModelFallbackEntry{
			{Model: "gpt-4", Fallbacks: []string{"gpt-4-mini"}},
		},
		MaxFallbacks: intPtr(2),
	}

	config := GenerateProxyConfig(instance)

	// litellm_settings should have both callbacks and default_fallbacks
	ls := config["litellm_settings"].(map[string]interface{})
	if _, ok := ls["success_callback"]; !ok {
		t.Error("expected success_callback in litellm_settings")
	}
	if _, ok := ls["default_fallbacks"]; !ok {
		t.Error("expected default_fallbacks in litellm_settings")
	}

	// router_settings should have routing_strategy, fallbacks, and max_fallbacks
	rs := config["router_settings"].(map[string]interface{})
	if rs["routing_strategy"] != "least-busy" {
		t.Errorf("expected routing_strategy=least-busy, got %v", rs["routing_strategy"])
	}
	if _, ok := rs["fallbacks"]; !ok {
		t.Error("expected fallbacks in router_settings")
	}
	if rs["max_fallbacks"] != 2 {
		t.Errorf("expected max_fallbacks=2, got %v", rs["max_fallbacks"])
	}
}

func TestGenerateProxyConfig_NoFallbacks(t *testing.T) {
	instance := newTestInstance()
	// No fallbacks set

	config := GenerateProxyConfig(instance)

	if _, ok := config["litellm_settings"]; ok {
		t.Error("litellm_settings should not be present when no fallbacks/callbacks/SSO set")
	}
	if _, ok := config["router_settings"]; ok {
		t.Error("router_settings should not be present when no router settings set")
	}
}

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

package resources

import (
	"encoding/json"
	"testing"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testModelClaude3Haiku = "claude-3-haiku"
	testJWTFieldSub       = "sub"
	testRouteModelNew     = "/model/new"
)

func intPtr(v int) *int { return &v }

// jsonParams wraps plain string values as JSON-encoded params for the
// map[string]apiextensionsv1.JSON fields (guardrail/credential params).
func jsonParams(m map[string]string) map[string]apiextensionsv1.JSON {
	out := make(map[string]apiextensionsv1.JSON, len(m))
	for k, v := range m {
		b, _ := json.Marshal(v)
		out[k] = apiextensionsv1.JSON{Raw: b}
	}
	return out
}

func TestGenerateProxyConfig_DefaultFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		DefaultFallbacks: []string{"gpt-4-mini", testModelClaude3Haiku},
	}

	config := GenerateProxyConfig(instance, nil)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	df, ok := ls["default_fallbacks"].([]string)
	if !ok {
		t.Fatal("expected default_fallbacks to be []string")
	}
	if len(df) != 2 || df[0] != "gpt-4-mini" || df[1] != testModelClaude3Haiku {
		t.Errorf("unexpected default_fallbacks: %v", df)
	}
}

func TestGenerateProxyConfig_ModelFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		ModelFallbacks: []litellmv1alpha1.ModelFallbackEntry{
			{Model: "gpt-4", Fallbacks: []string{"gpt-4-mini", testModelClaude3Haiku}},
		},
	}

	config := GenerateProxyConfig(instance, nil)

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
	if len(models) != 2 || models[0] != "gpt-4-mini" || models[1] != testModelClaude3Haiku {
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

	config := GenerateProxyConfig(instance, nil)

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

	config := GenerateProxyConfig(instance, nil)

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

	config := GenerateProxyConfig(instance, nil)

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

	config := GenerateProxyConfig(instance, nil)

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

// TestGenerateProxyConfig_PreviouslyDeadFields guards against regression of
// three fields that were defined in the CRD but never rendered: they now must
// reach general_settings / router_settings.
func TestGenerateProxyConfig_PreviouslyDeadFields(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		CustomKeyGenerate: "custom_auth.custom_generate_key_fn",
	}
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		RetryAfter: intPtr(15),
	}
	instance.Spec.Database.ConnectionPool = &litellmv1alpha1.ConnectionPoolSpec{
		MaxConnections: 20,
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["custom_key_generate"] != "custom_auth.custom_generate_key_fn" {
		t.Errorf("expected custom_key_generate rendered, got %v", gs["custom_key_generate"])
	}
	if gs["database_connection_pool_limit"] != 20 {
		t.Errorf("expected database_connection_pool_limit=20, got %v", gs["database_connection_pool_limit"])
	}

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	if rs["retry_after"] != 15 {
		t.Errorf("expected retry_after=15, got %v", rs["retry_after"])
	}
}

// TestGenerateProxyConfig_Tier1InstanceKnobs covers the router/general/litellm
// settings added in the Tier 1 audit follow-up.
func TestGenerateProxyConfig_Tier1InstanceKnobs(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		StreamTimeout:       intPtr(45),
		EnablePreCallChecks: boolPtr(true),
		ModelGroupAlias:     map[string]string{"gpt-4": "gpt-4o"},
	}
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		Alerting:                      []string{"slack"},
		AlertingThreshold:             intPtr(300),
		AlertToWebhookURL:             map[string]string{"budget_alerts": "https://hooks.example/x"},
		BackgroundHealthChecks:        boolPtr(true),
		HealthCheckInterval:           intPtr(120),
		HealthCheckDetails:            boolPtr(false),
		HealthCheckSkipDisabledModels: boolPtr(true),
	}
	instance.Spec.LiteLLMSettings = &litellmv1alpha1.LiteLLMSettingsSpec{
		JSONLogs: boolPtr(true),
	}

	config := GenerateProxyConfig(instance, nil)

	rs := config["router_settings"].(map[string]interface{})
	if rs["stream_timeout"] != 45 {
		t.Errorf("expected stream_timeout=45, got %v", rs["stream_timeout"])
	}
	if rs["enable_pre_call_checks"] != true {
		t.Errorf("expected enable_pre_call_checks=true, got %v", rs["enable_pre_call_checks"])
	}
	if alias, _ := rs["model_group_alias"].(map[string]string); alias["gpt-4"] != "gpt-4o" {
		t.Errorf("expected model_group_alias[gpt-4]=gpt-4o, got %v", rs["model_group_alias"])
	}

	gs := config["general_settings"].(map[string]interface{})
	if alerting, _ := gs["alerting"].([]string); len(alerting) != 1 || alerting[0] != "slack" {
		t.Errorf("expected alerting=[slack], got %v", gs["alerting"])
	}
	if gs["alerting_threshold"] != 300 {
		t.Errorf("expected alerting_threshold=300, got %v", gs["alerting_threshold"])
	}
	if gs["background_health_checks"] != true {
		t.Errorf("expected background_health_checks=true, got %v", gs["background_health_checks"])
	}
	if gs["health_check_details"] != false {
		t.Errorf("expected health_check_details=false, got %v", gs["health_check_details"])
	}
	if gs["health_check_skip_disabled_background_models"] != true {
		t.Errorf("expected health_check_skip_disabled_background_models=true, got %v", gs["health_check_skip_disabled_background_models"])
	}

	ls := config["litellm_settings"].(map[string]interface{})
	if ls["json_logs"] != true {
		t.Errorf("expected json_logs=true, got %v", ls["json_logs"])
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

	config := GenerateProxyConfig(instance, nil)

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

	config := GenerateProxyConfig(instance, nil)

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

	config := GenerateProxyConfig(instance, nil)

	if _, ok := config["litellm_settings"]; ok {
		t.Error("litellm_settings should not be present when no fallbacks/callbacks/SSO set")
	}
	if _, ok := config["router_settings"]; ok {
		t.Error("router_settings should not be present when no router settings set")
	}
}

func TestGenerateProxyConfig_CachingDisabled(t *testing.T) {
	instance := newTestInstance()
	// No caching set at all

	config := GenerateProxyConfig(instance, nil)

	if _, ok := config["litellm_settings"]; ok {
		t.Error("litellm_settings should not be present when caching is not configured")
	}
}

func TestGenerateProxyConfig_CachingEnabledFalse(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: false,
		Type:    cacheTypeRedis,
	}

	config := GenerateProxyConfig(instance, nil)

	if _, ok := config["litellm_settings"]; ok {
		t.Error("litellm_settings should not be present when caching.enabled is false")
	}
}

func TestGenerateProxyConfig_CachingRedis(t *testing.T) {
	instance := newTestInstance()
	ttl := 300
	port := 6380
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled:            true,
		Type:               cacheTypeRedis,
		Namespace:          "my-ns",
		TTL:                &ttl,
		SupportedCallTypes: []string{"acompletion", "aembedding"},
		Mode:               cacheModeDefaultOff,
		Redis: &litellmv1alpha1.CacheRedisSpec{
			Host: "redis.example.com",
			Port: &port,
			PasswordSecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "redis-secret",
				Key:  "password",
			},
			SSL: true,
		},
	}

	config := GenerateProxyConfig(instance, nil)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	if ls["cache"] != true {
		t.Error("expected cache=true")
	}
	params, ok := ls["cache_params"].(map[string]interface{})
	if !ok {
		t.Fatal("expected cache_params to be present")
	}
	if params["type"] != cacheTypeRedis {
		t.Errorf("expected type=redis, got %v", params["type"])
	}
	if params["host"] != "redis.example.com" {
		t.Errorf("expected host=redis.example.com, got %v", params["host"])
	}
	if params["port"] != 6380 {
		t.Errorf("expected port=6380, got %v", params["port"])
	}
	if params["password"] != "os.environ/CACHE_REDIS_PASSWORD" {
		t.Errorf("expected password env ref, got %v", params["password"])
	}
	if params["ssl"] != true {
		t.Error("expected ssl=true")
	}
	if params["ttl"] != 300 {
		t.Errorf("expected ttl=300, got %v", params["ttl"])
	}
	if params["namespace"] != "my-ns" {
		t.Errorf("expected namespace=my-ns, got %v", params["namespace"])
	}
	callTypes, ok := params["supported_call_types"].([]string)
	if !ok || len(callTypes) != 2 {
		t.Fatalf("expected 2 supported_call_types, got %v", params["supported_call_types"])
	}
	if callTypes[0] != "acompletion" || callTypes[1] != "aembedding" {
		t.Errorf("unexpected supported_call_types: %v", callTypes)
	}
	if params["mode"] != cacheModeDefaultOff {
		t.Errorf("expected mode=default_off, got %v", params["mode"])
	}
}

func TestGenerateProxyConfig_CachingRedisReusesInstanceRedis(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Redis = &litellmv1alpha1.RedisSpec{
		Enabled: true,
		Host:    "shared-redis.default.svc",
		Port:    6379,
		PasswordSecretRef: &litellmv1alpha1.SecretKeyRef{
			Name: "redis-secret",
			Key:  "password",
		},
	}
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    cacheTypeRedis,
		// No Redis block — should reuse instance Redis
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	params := ls["cache_params"].(map[string]interface{})

	if params["host"] != "shared-redis.default.svc" {
		t.Errorf("expected host from instance Redis, got %v", params["host"])
	}
	if params["port"] != 6379 {
		t.Errorf("expected port from instance Redis, got %v", params["port"])
	}
	if params["password"] != "os.environ/REDIS_PASSWORD" {
		t.Errorf("expected REDIS_PASSWORD env ref for reused Redis, got %v", params["password"])
	}
}

func TestGenerateProxyConfig_CachingS3(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    "s3",
		S3: &litellmv1alpha1.CacheS3Spec{
			BucketName: "my-cache-bucket",
			Region:     "us-east-1",
			CredentialsSecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "aws-creds",
				Key:  "credentials",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	params := ls["cache_params"].(map[string]interface{})

	if params["type"] != "s3" {
		t.Errorf("expected type=s3, got %v", params["type"])
	}
	if params["s3_bucket_name"] != "my-cache-bucket" {
		t.Errorf("expected s3_bucket_name=my-cache-bucket, got %v", params["s3_bucket_name"])
	}
	if params["s3_region_name"] != "us-east-1" {
		t.Errorf("expected s3_region_name=us-east-1, got %v", params["s3_region_name"])
	}
	if params["s3_aws_access_key_id"] != "os.environ/CACHE_S3_ACCESS_KEY_ID" {
		t.Errorf("expected s3 access key env ref, got %v", params["s3_aws_access_key_id"])
	}
}

func TestGenerateProxyConfig_CachingQdrant(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    cacheTypeQdrant,
		Qdrant: &litellmv1alpha1.CacheQdrantSpec{
			URL:            "http://qdrant:6333",
			CollectionName: "llm-cache",
			APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "qdrant-secret",
				Key:  "api-key",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	params := ls["cache_params"].(map[string]interface{})

	if params["type"] != cacheTypeQdrant {
		t.Errorf("expected type=qdrant, got %v", params["type"])
	}
	if params["qdrant_url"] != "http://qdrant:6333" {
		t.Errorf("expected qdrant_url, got %v", params["qdrant_url"])
	}
	if params["qdrant_collection_name"] != "llm-cache" {
		t.Errorf("expected qdrant_collection_name=llm-cache, got %v", params["qdrant_collection_name"])
	}
	if params["qdrant_api_key"] != "os.environ/CACHE_QDRANT_API_KEY" {
		t.Errorf("expected qdrant api key env ref, got %v", params["qdrant_api_key"])
	}
}

func TestGenerateProxyConfig_CachingLocal(t *testing.T) {
	instance := newTestInstance()
	ttl := 120
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    "local",
		TTL:     &ttl,
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	params := ls["cache_params"].(map[string]interface{})

	if params["type"] != "local" {
		t.Errorf("expected type=local, got %v", params["type"])
	}
	if params["ttl"] != 120 {
		t.Errorf("expected ttl=120, got %v", params["ttl"])
	}
}

func TestGenerateProxyConfig_TagFilteringEnabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		EnableTagFiltering: boolPtr(true),
	}

	config := GenerateProxyConfig(instance, nil)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	if rs["enable_tag_filtering"] != true {
		t.Errorf("expected enable_tag_filtering=true, got %v", rs["enable_tag_filtering"])
	}
	if _, ok := rs["tag_filtering_match_any"]; ok {
		t.Error("tag_filtering_match_any should not be present when not set")
	}
}

func TestGenerateProxyConfig_TagFilteringMatchAny(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		EnableTagFiltering:   boolPtr(true),
		TagFilteringMatchAny: boolPtr(true),
	}

	config := GenerateProxyConfig(instance, nil)

	rs := config["router_settings"].(map[string]interface{})
	if rs["enable_tag_filtering"] != true {
		t.Errorf("expected enable_tag_filtering=true, got %v", rs["enable_tag_filtering"])
	}
	if rs["tag_filtering_match_any"] != true {
		t.Errorf("expected tag_filtering_match_any=true, got %v", rs["tag_filtering_match_any"])
	}
}

func TestGenerateProxyConfig_TagFilteringDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		EnableTagFiltering: boolPtr(false),
	}

	config := GenerateProxyConfig(instance, nil)

	// enable_tag_filtering=false should not emit the key
	if rs, ok := config["router_settings"].(map[string]interface{}); ok {
		if _, ok := rs["enable_tag_filtering"]; ok {
			t.Error("enable_tag_filtering should not be present when false")
		}
	}
}

func TestGenerateProxyConfig_IPAllowlist(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{
		IPAllowlist: &litellmv1alpha1.IPAllowlistSpec{
			Enabled:    true,
			AllowedIPs: []string{"192.168.1.0/24", "10.0.0.1"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	ips, ok := gs["allowed_ips"].([]string)
	if !ok {
		t.Fatal("expected allowed_ips to be []string")
	}
	if len(ips) != 2 || ips[0] != "192.168.1.0/24" || ips[1] != "10.0.0.1" {
		t.Errorf("unexpected allowed_ips: %v", ips)
	}
	if _, ok := gs["use_x_forwarded_for"]; ok {
		t.Error("use_x_forwarded_for should not be present when not set")
	}
}

func TestGenerateProxyConfig_IPAllowlistWithXForwardedFor(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{
		IPAllowlist: &litellmv1alpha1.IPAllowlistSpec{
			Enabled:          true,
			AllowedIPs:       []string{"10.0.0.0/8"},
			UseXForwardedFor: boolPtr(true),
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["use_x_forwarded_for"] != true {
		t.Errorf("expected use_x_forwarded_for=true, got %v", gs["use_x_forwarded_for"])
	}
}

func TestGenerateProxyConfig_IPAllowlistWithMaxSizes(t *testing.T) {
	instance := newTestInstance()
	reqSize := 10
	respSize := 25
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{
		IPAllowlist: &litellmv1alpha1.IPAllowlistSpec{
			Enabled:           true,
			AllowedIPs:        []string{"0.0.0.0/0"},
			MaxRequestSizeMB:  &reqSize,
			MaxResponseSizeMB: &respSize,
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["max_request_size_mb"] != 10 {
		t.Errorf("expected max_request_size_mb=10, got %v", gs["max_request_size_mb"])
	}
	if gs["max_response_size_mb"] != 25 {
		t.Errorf("expected max_response_size_mb=25, got %v", gs["max_response_size_mb"])
	}
}

func TestGenerateProxyConfig_IPAllowlistDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{
		IPAllowlist: &litellmv1alpha1.IPAllowlistSpec{
			Enabled:    false,
			AllowedIPs: []string{"10.0.0.1"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	if gs, ok := config["general_settings"].(map[string]interface{}); ok {
		if _, ok := gs["allowed_ips"]; ok {
			t.Error("allowed_ips should not be present when IP allowlist is disabled")
		}
	}
}

func TestGenerateProxyConfig_IPAllowlistWithExistingGeneralSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		ProxyBatchWriteAt: 10,
	}
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{
		IPAllowlist: &litellmv1alpha1.IPAllowlistSpec{
			Enabled:    true,
			AllowedIPs: []string{"10.0.0.1"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["proxy_batch_write_at"] != 10 {
		t.Errorf("expected proxy_batch_write_at=10, got %v", gs["proxy_batch_write_at"])
	}
	ips, ok := gs["allowed_ips"].([]string)
	if !ok || len(ips) != 1 || ips[0] != "10.0.0.1" {
		t.Errorf("expected allowed_ips=[10.0.0.1], got %v", gs["allowed_ips"])
	}
}

func TestGenerateProxyConfig_PassThroughEndpoints_Basic(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:   "/bria",
			Target: "https://engine.prod.bria-api.com",
			Headers: map[string]string{
				"content-type": "application/json",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	endpoints, ok := gs["pass_through_endpoints"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected pass_through_endpoints to be []map[string]interface{}")
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	ep := endpoints[0]
	if ep["path"] != "/bria" {
		t.Errorf("expected path=/bria, got %v", ep["path"])
	}
	if ep["target"] != "https://engine.prod.bria-api.com" {
		t.Errorf("expected target URL, got %v", ep["target"])
	}
	headers, ok := ep["headers"].(map[string]string)
	if !ok {
		t.Fatal("expected headers to be map[string]string")
	}
	if headers["content-type"] != "application/json" {
		t.Errorf("expected content-type header, got %v", headers["content-type"])
	}
}

func TestGenerateProxyConfig_PassThroughEndpoints_AllFields(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:           "/langfuse",
			Target:         "https://us.cloud.langfuse.com",
			Auth:           boolPtr(true),
			ForwardHeaders: boolPtr(true),
			IncludeSubpath: boolPtr(true),
			Methods:        []string{"GET", "POST"},
			DefaultQueryParams: map[string]string{
				"version": "2",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	endpoints := gs["pass_through_endpoints"].([]map[string]interface{})
	ep := endpoints[0]

	if ep["auth"] != true {
		t.Errorf("expected auth=true, got %v", ep["auth"])
	}
	if ep["forward_headers"] != true {
		t.Errorf("expected forward_headers=true, got %v", ep["forward_headers"])
	}
	if ep["include_subpath"] != true {
		t.Errorf("expected include_subpath=true, got %v", ep["include_subpath"])
	}
	methods, ok := ep["methods"].([]string)
	if !ok || len(methods) != 2 || methods[0] != "GET" || methods[1] != "POST" {
		t.Errorf("unexpected methods: %v", ep["methods"])
	}
	qp, ok := ep["default_query_params"].(map[string]string)
	if !ok || qp["version"] != "2" {
		t.Errorf("unexpected default_query_params: %v", ep["default_query_params"])
	}
}

func TestGenerateProxyConfig_PassThroughEndpoints_SecretHeaders(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:   "/bria",
			Target: "https://engine.prod.bria-api.com",
			HeaderSecrets: []litellmv1alpha1.HeaderSecretRef{
				{
					HeaderName: "Authorization",
					Prefix:     "Bearer ",
					SecretRef: litellmv1alpha1.SecretKeyRef{
						Name: "bria-secret",
						Key:  "api-key",
					},
				},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	endpoints := gs["pass_through_endpoints"].([]map[string]interface{})
	headers := endpoints[0]["headers"].(map[string]string)

	expected := "Bearer os.environ/PASSTHROUGH_BRIA_AUTHORIZATION"
	if headers["Authorization"] != expected {
		t.Errorf("expected %q, got %q", expected, headers["Authorization"])
	}
}

func TestGenerateProxyConfig_PassThroughEndpoints_MixedHeaders(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:   "/custom",
			Target: "https://api.example.com",
			Headers: map[string]string{
				"content-type": "application/json",
			},
			HeaderSecrets: []litellmv1alpha1.HeaderSecretRef{
				{
					HeaderName: "X-API-Key",
					SecretRef: litellmv1alpha1.SecretKeyRef{
						Name: "custom-secret",
						Key:  "key",
					},
				},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	endpoints := gs["pass_through_endpoints"].([]map[string]interface{})
	headers := endpoints[0]["headers"].(map[string]string)

	if headers["content-type"] != "application/json" {
		t.Errorf("expected static header, got %v", headers["content-type"])
	}
	if headers["X-API-Key"] != "os.environ/PASSTHROUGH_CUSTOM_X_API_KEY" {
		t.Errorf("expected secret header env ref, got %v", headers["X-API-Key"])
	}
}

func TestGenerateProxyConfig_PassThroughEndpoints_WithExistingGeneralSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		ProxyBatchWriteAt: 10,
	}
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:   "/test",
			Target: "https://example.com",
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["proxy_batch_write_at"] != 10 {
		t.Errorf("expected proxy_batch_write_at=10, got %v", gs["proxy_batch_write_at"])
	}
	endpoints, ok := gs["pass_through_endpoints"].([]map[string]interface{})
	if !ok || len(endpoints) != 1 {
		t.Fatalf("expected 1 pass_through_endpoint, got %v", gs["pass_through_endpoints"])
	}
}

func TestGenerateProxyConfig_PassThroughEndpoints_OmitsOptionalFields(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:   "/minimal",
			Target: "https://example.com",
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	endpoints := gs["pass_through_endpoints"].([]map[string]interface{})
	ep := endpoints[0]

	if _, ok := ep["auth"]; ok {
		t.Error("auth should not be present when not set")
	}
	if _, ok := ep["forward_headers"]; ok {
		t.Error("forward_headers should not be present when not set")
	}
	if _, ok := ep["include_subpath"]; ok {
		t.Error("include_subpath should not be present when not set")
	}
	if _, ok := ep["methods"]; ok {
		t.Error("methods should not be present when empty")
	}
	if _, ok := ep["headers"]; ok {
		t.Error("headers should not be present when empty")
	}
	if _, ok := ep["default_query_params"]; ok {
		t.Error("default_query_params should not be present when empty")
	}
}

func TestPassThroughEnvVarName(t *testing.T) {
	tests := []struct {
		path, header, expected string
	}{
		{"/bria", "Authorization", "PASSTHROUGH_BRIA_AUTHORIZATION"},
		{"/api/v1/custom", "X-API-Key", "PASSTHROUGH_API_V1_CUSTOM_X_API_KEY"},
		{"/langfuse", "content-type", "PASSTHROUGH_LANGFUSE_CONTENT_TYPE"},
		{"custom", "Auth", "PASSTHROUGH_CUSTOM_AUTH"},
	}
	for _, tc := range tests {
		got := PassThroughEnvVarName(tc.path, tc.header)
		if got != tc.expected {
			t.Errorf("PassThroughEnvVarName(%q, %q) = %q, want %q", tc.path, tc.header, got, tc.expected)
		}
	}
}

func TestGenerateProxyConfig_CachingWithExistingSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Callbacks = &litellmv1alpha1.CallbacksSpec{
		Types: []string{"langfuse"},
	}
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    "local",
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	if _, ok := ls["success_callback"]; !ok {
		t.Error("expected success_callback in litellm_settings")
	}
	if ls["cache"] != true {
		t.Error("expected cache=true alongside callbacks")
	}
}

func strPtr(s string) *string { return &s }

func TestGenerateProxyConfig_GlobalBudget(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		MaxBudget:      strPtr("10000.00"),
		BudgetDuration: "30d",
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["max_budget"] != "10000.00" {
		t.Errorf("expected max_budget=10000.00, got %v", gs["max_budget"])
	}
	if gs["budget_duration"] != "30d" {
		t.Errorf("expected budget_duration=30d, got %v", gs["budget_duration"])
	}
}

func TestGenerateProxyConfig_GlobalMaxParallelRequests(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		GlobalMaxParallelRequests: intPtr(100),
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["global_max_parallel_requests"] != 100 {
		t.Errorf("expected global_max_parallel_requests=100, got %v", gs["global_max_parallel_requests"])
	}
}

func TestGenerateProxyConfig_BudgetRescheduler(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		BudgetReschedulerMinTime: intPtr(300),
		BudgetReschedulerMaxTime: intPtr(600),
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["proxy_budget_rescheduler_min_time"] != 300 {
		t.Errorf("expected proxy_budget_rescheduler_min_time=300, got %v", gs["proxy_budget_rescheduler_min_time"])
	}
	if gs["proxy_budget_rescheduler_max_time"] != 600 {
		t.Errorf("expected proxy_budget_rescheduler_max_time=600, got %v", gs["proxy_budget_rescheduler_max_time"])
	}
}

func TestGenerateProxyConfig_DefaultMaxParallelRequests(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		DefaultMaxParallelRequests: intPtr(10),
	}

	config := GenerateProxyConfig(instance, nil)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	if rs["default_max_parallel_requests"] != 10 {
		t.Errorf("expected default_max_parallel_requests=10, got %v", rs["default_max_parallel_requests"])
	}
}

func TestGenerateProxyConfig_ProviderBudgetConfig(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		ProviderBudgetConfig: map[string]litellmv1alpha1.ProviderBudget{
			"openai": {
				BudgetLimit: "500.00",
				TimePeriod:  "1d",
			},
			"anthropic": {
				BudgetLimit: "300.00",
				TimePeriod:  "1d",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	rs, ok := config["router_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected router_settings to be present")
	}
	pbc, ok := rs["provider_budget_config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected provider_budget_config to be present")
	}
	openai, ok := pbc["openai"].(map[string]interface{})
	if !ok {
		t.Fatal("expected openai entry in provider_budget_config")
	}
	if openai["budget_limit"] != "500.00" {
		t.Errorf("expected openai budget_limit=500.00, got %v", openai["budget_limit"])
	}
	if openai["time_period"] != "1d" {
		t.Errorf("expected openai time_period=1d, got %v", openai["time_period"])
	}
	anthropic, ok := pbc["anthropic"].(map[string]interface{})
	if !ok {
		t.Fatal("expected anthropic entry in provider_budget_config")
	}
	if anthropic["budget_limit"] != "300.00" {
		t.Errorf("expected anthropic budget_limit=300.00, got %v", anthropic["budget_limit"])
	}
}

func TestGenerateProxyConfig_AllBudgetSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		MaxBudget:                 strPtr("5000.00"),
		BudgetDuration:            "7d",
		GlobalMaxParallelRequests: intPtr(50),
		BudgetReschedulerMinTime:  intPtr(120),
		BudgetReschedulerMaxTime:  intPtr(300),
	}
	instance.Spec.RouterSettings = &litellmv1alpha1.RouterSettingsSpec{
		DefaultMaxParallelRequests: intPtr(5),
		ProviderBudgetConfig: map[string]litellmv1alpha1.ProviderBudget{
			"openai": {BudgetLimit: "1000.00", TimePeriod: "7d"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["max_budget"] != "5000.00" {
		t.Errorf("expected max_budget=5000.00, got %v", gs["max_budget"])
	}
	if gs["budget_duration"] != "7d" {
		t.Errorf("expected budget_duration=7d, got %v", gs["budget_duration"])
	}
	if gs["global_max_parallel_requests"] != 50 {
		t.Errorf("expected global_max_parallel_requests=50, got %v", gs["global_max_parallel_requests"])
	}
	if gs["proxy_budget_rescheduler_min_time"] != 120 {
		t.Errorf("expected proxy_budget_rescheduler_min_time=120, got %v", gs["proxy_budget_rescheduler_min_time"])
	}
	if gs["proxy_budget_rescheduler_max_time"] != 300 {
		t.Errorf("expected proxy_budget_rescheduler_max_time=300, got %v", gs["proxy_budget_rescheduler_max_time"])
	}

	rs := config["router_settings"].(map[string]interface{})
	if rs["default_max_parallel_requests"] != 5 {
		t.Errorf("expected default_max_parallel_requests=5, got %v", rs["default_max_parallel_requests"])
	}
	pbc := rs["provider_budget_config"].(map[string]interface{})
	if _, ok := pbc["openai"]; !ok {
		t.Error("expected openai in provider_budget_config")
	}
}

func TestGenerateProxyConfig_DefaultCustomerBudget_MaxBudget(t *testing.T) {
	instance := newTestInstance()
	budget := 25.0
	instance.Spec.DefaultCustomerBudget = &litellmv1alpha1.DefaultCustomerBudgetSpec{
		MaxBudget: &budget,
	}

	config := GenerateProxyConfig(instance, nil)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	if ls["max_end_user_budget"] != 25.0 {
		t.Errorf("expected max_end_user_budget=25.0, got %v", ls["max_end_user_budget"])
	}
	if _, present := ls["max_end_user_budget_id"]; present {
		t.Error("expected max_end_user_budget_id to be absent when only MaxBudget is set")
	}
}

func TestGenerateProxyConfig_DefaultCustomerBudget_BudgetID(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.DefaultCustomerBudget = &litellmv1alpha1.DefaultCustomerBudgetSpec{
		BudgetID: "tier-free",
	}

	config := GenerateProxyConfig(instance, nil)

	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	if ls["max_end_user_budget_id"] != "tier-free" {
		t.Errorf("expected max_end_user_budget_id=tier-free, got %v", ls["max_end_user_budget_id"])
	}
}

func TestGenerateProxyConfig_DefaultCustomerBudget_Empty(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.DefaultCustomerBudget = &litellmv1alpha1.DefaultCustomerBudgetSpec{}

	config := GenerateProxyConfig(instance, nil)

	if ls, ok := config["litellm_settings"].(map[string]interface{}); ok {
		if _, present := ls["max_end_user_budget"]; present {
			t.Error("expected no max_end_user_budget when spec is empty")
		}
		if _, present := ls["max_end_user_budget_id"]; present {
			t.Error("expected no max_end_user_budget_id when spec is empty")
		}
	}
}

// NOTE: credentials are no longer rendered into proxy_server_config.yaml.
// They are registered against the LiteLLM proxy's /credentials API by
// internal/controller/litellmcredential_controller.go; see that controller's
// tests for coverage of CRUD, Secret-rotation, and hash-based change detection.

func TestGenerateProxyConfig_GuardrailsNone(t *testing.T) {
	instance := newTestInstance()

	config := GenerateProxyConfig(instance, nil)

	if _, present := config["guardrails"]; present {
		t.Error("guardrails should be absent when no guardrails are provided")
	}
}

func TestGenerateProxyConfig_GuardrailsSingleEntry(t *testing.T) {
	instance := newTestInstance()
	defaultOn := true
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pii-detector", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName: "pii-detector",
				Provider:      "aporia",
				Mode:          "pre_call",
				APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{
					Name: "aporia-secret",
					Key:  "api-key",
				},
				APIBase:   "https://api.aporia.com",
				DefaultOn: &defaultOn,
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)

	entries, ok := config["guardrails"].([]map[string]interface{})
	if !ok {
		t.Fatalf("guardrails should be []map[string]interface{}, got %T", config["guardrails"])
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 guardrail entry, got %d", len(entries))
	}
	if entries[0]["guardrail_name"] != "pii-detector" {
		t.Errorf("expected guardrail_name=pii-detector, got %v", entries[0]["guardrail_name"])
	}
	params, ok := entries[0]["litellm_params"].(map[string]interface{})
	if !ok {
		t.Fatal("litellm_params should be a map")
	}
	if params["guardrail"] != "aporia" {
		t.Errorf("expected guardrail=aporia, got %v", params["guardrail"])
	}
	if params["mode"] != "pre_call" {
		t.Errorf("expected mode=pre_call, got %v", params["mode"])
	}
	if params["api_key"] != "os.environ/GUARDRAIL_PII_DETECTOR_API_KEY" {
		t.Errorf("expected os.environ/GUARDRAIL_PII_DETECTOR_API_KEY, got %v", params["api_key"])
	}
	if params["api_base"] != "https://api.aporia.com" {
		t.Errorf("expected api_base=https://api.aporia.com, got %v", params["api_base"])
	}
	if params["default_on"] != true {
		t.Errorf("expected default_on=true, got %v", params["default_on"])
	}
}

func TestGenerateProxyConfig_GuardrailsNoAPIKey(t *testing.T) {
	// Some providers (local presidio, custom_guardrail pointing at an internal
	// service) don't need an api key. Make sure api_key is omitted when
	// APIKeySecretRef is nil.
	instance := newTestInstance()
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "presidio", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName: "presidio",
				Provider:      "presidio",
				Mode:          "pre_call",
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)

	entries, _ := config["guardrails"].([]map[string]interface{})
	params, _ := entries[0]["litellm_params"].(map[string]interface{})
	if _, present := params["api_key"]; present {
		t.Error("api_key should be absent when APIKeySecretRef is nil")
	}
}

func TestGenerateProxyConfig_GuardrailsWithParamsNotOverridingReserved(t *testing.T) {
	instance := newTestInstance()
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "bedrock-pii", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName:   "bedrock-pii",
				Provider:        "bedrock",
				Mode:            "post_call",
				APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{Name: "aws-secret", Key: "key"},
				Params: jsonParams(map[string]string{
					"guardrailIdentifier": "abc123",
					"guardrailVersion":    "DRAFT",
					// reserved keys — should be rejected
					"guardrail": "lakera",
					"mode":      "during_call",
				}),
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)

	entries, _ := config["guardrails"].([]map[string]interface{})
	params, _ := entries[0]["litellm_params"].(map[string]interface{})

	if params["guardrail"] != "bedrock" {
		t.Errorf("reserved key `guardrail` was overridden by params: %v", params["guardrail"])
	}
	if params["mode"] != "post_call" {
		t.Errorf("reserved key `mode` was overridden by params: %v", params["mode"])
	}
	if params["guardrailIdentifier"] != "abc123" {
		t.Errorf("expected guardrailIdentifier passthrough, got %v", params["guardrailIdentifier"])
	}
	if params["guardrailVersion"] != "DRAFT" {
		t.Errorf("expected guardrailVersion passthrough, got %v", params["guardrailVersion"])
	}
}

// TestGenerateProxyConfig_GuardrailStructuredParams verifies that non-string
// params (nested objects, numbers) survive as structured JSON rather than being
// stringified — e.g. Presidio's pii_entities_config map.
func TestGenerateProxyConfig_GuardrailStructuredParams(t *testing.T) {
	instance := newTestInstance()
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "presidio", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName: "presidio",
				Provider:      "presidio",
				Mode:          "pre_call",
				Params: map[string]apiextensionsv1.JSON{
					"pii_entities_config": {Raw: []byte(`{"CREDIT_CARD":"MASK","EMAIL":"BLOCK"}`)},
					"presidio_threshold":  {Raw: []byte(`0.7`)},
				},
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)
	entries, _ := config["guardrails"].([]map[string]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 guardrail entry, got %d", len(entries))
	}
	params, _ := entries[0]["litellm_params"].(map[string]interface{})

	cfg, ok := params["pii_entities_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected pii_entities_config to be a nested map, got %T (%v)", params["pii_entities_config"], params["pii_entities_config"])
	}
	if cfg["CREDIT_CARD"] != "MASK" || cfg["EMAIL"] != "BLOCK" {
		t.Errorf("unexpected pii_entities_config contents: %v", cfg)
	}
	// JSON numbers decode to float64.
	if params["presidio_threshold"] != float64(0.7) {
		t.Errorf("expected presidio_threshold=0.7 (float64), got %T %v", params["presidio_threshold"], params["presidio_threshold"])
	}
}

func TestGenerateProxyConfig_GuardrailsCustomGuardrailClass(t *testing.T) {
	// For provider=custom_guardrail, litellm_params.guardrail must be the
	// dotted Python import path from spec.guardrailClass, not the provider
	// keyword.
	instance := newTestInstance()
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "my-custom", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:    litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName:  "my-custom",
				Provider:       "custom_guardrail",
				Mode:           "pre_call",
				GuardrailClass: "my_pkg.adapters.MyGuardrail",
				Params: jsonParams(map[string]string{
					"some_setting": "value",
					// reserved key — must not override the class path
					"guardrail": "should_be_ignored",
				}),
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)

	entries, _ := config["guardrails"].([]map[string]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 guardrail entry, got %d", len(entries))
	}
	params, _ := entries[0]["litellm_params"].(map[string]interface{})
	if params["guardrail"] != "my_pkg.adapters.MyGuardrail" {
		t.Errorf("expected guardrail=my_pkg.adapters.MyGuardrail, got %v", params["guardrail"])
	}
	if params["mode"] != "pre_call" {
		t.Errorf("expected mode=pre_call, got %v", params["mode"])
	}
	if params["some_setting"] != "value" {
		t.Errorf("expected some_setting=value passthrough, got %v", params["some_setting"])
	}
}

func TestGenerateProxyConfig_GuardrailsGenericAPI(t *testing.T) {
	// provider=generic_guardrail_api renders the provider literal as the
	// guardrail value, sets unreachable_fallback, and nests spec.params under
	// additional_provider_specific_params (not flat in litellm_params).
	instance := newTestInstance()
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ext-http", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:         litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName:       "ext-http",
				Provider:            "generic_guardrail_api",
				Mode:                "pre_call",
				APIBase:             "http://my-guardrail.guardrails.svc.cluster.local:8080",
				UnreachableFallback: "fail_open",
				APIKeySecretRef:     &litellmv1alpha1.SecretKeyRef{Name: "gr-secret", Key: "api-key"},
				Params: jsonParams(map[string]string{
					"threshold": "0.8",
					"language":  "en",
				}),
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)

	entries, _ := config["guardrails"].([]map[string]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 guardrail entry, got %d", len(entries))
	}
	params, _ := entries[0]["litellm_params"].(map[string]interface{})
	if params["guardrail"] != "generic_guardrail_api" {
		t.Errorf("expected guardrail=generic_guardrail_api, got %v", params["guardrail"])
	}
	if params["api_base"] != "http://my-guardrail.guardrails.svc.cluster.local:8080" {
		t.Errorf("unexpected api_base: %v", params["api_base"])
	}
	if params["api_key"] != "os.environ/GUARDRAIL_EXT_HTTP_API_KEY" {
		t.Errorf("unexpected api_key: %v", params["api_key"])
	}
	if params["unreachable_fallback"] != "fail_open" {
		t.Errorf("expected unreachable_fallback=fail_open, got %v", params["unreachable_fallback"])
	}
	// Custom params must be nested, not flat.
	if _, flat := params["threshold"]; flat {
		t.Error("threshold should be nested under additional_provider_specific_params, not flat")
	}
	extra, ok := params["additional_provider_specific_params"].(map[string]interface{})
	if !ok {
		t.Fatalf("additional_provider_specific_params should be a map, got %T", params["additional_provider_specific_params"])
	}
	if extra["threshold"] != "0.8" {
		t.Errorf("expected nested threshold=0.8, got %v", extra["threshold"])
	}
	if extra["language"] != "en" {
		t.Errorf("expected nested language=en, got %v", extra["language"])
	}
}

func TestGenerateProxyConfig_GuardrailsFilterByInstance(t *testing.T) {
	instance := newTestInstance() // name: test-instance
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "mine", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: "test-instance"},
				GuardrailName: "mine",
				Provider:      "presidio",
				Mode:          "pre_call",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "theirs", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: "other-instance"},
				GuardrailName: "theirs",
				Provider:      "aporia",
				Mode:          "pre_call",
			},
		},
	}

	config := GenerateProxyConfig(instance, guardrails)

	entries, _ := config["guardrails"].([]map[string]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (filtered), got %d", len(entries))
	}
	if entries[0]["guardrail_name"] != "mine" {
		t.Errorf("expected mine, got %v", entries[0]["guardrail_name"])
	}
}

func TestGuardrailEnvVarName_Sanitization(t *testing.T) {
	cases := map[string]string{
		"pii-detector":        "GUARDRAIL_PII_DETECTOR_API_KEY",
		"aporia.prod.1":       "GUARDRAIL_APORIA_PROD_1_API_KEY",
		"guard with spaces":   "GUARDRAIL_GUARD_WITH_SPACES_API_KEY",
		"already_underscored": "GUARDRAIL_ALREADY_UNDERSCORED_API_KEY",
	}
	for in, want := range cases {
		got := GuardrailEnvVarName(in)
		if got != want {
			t.Errorf("GuardrailEnvVarName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateProxyConfig_SecretManagerNone(t *testing.T) {
	instance := newTestInstance()
	config := GenerateProxyConfig(instance, nil)

	gs, _ := config["general_settings"].(map[string]interface{})
	if gs != nil {
		if _, ok := gs["key_management_system"]; ok {
			t.Error("key_management_system should not be present when secretManager is nil")
		}
	}
}

func TestGenerateProxyConfig_SecretManagerAWS(t *testing.T) {
	instance := newTestInstance()
	storeKeys := true
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider:                   "aws_secret_manager",
		HostedKeys:                 []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"},
		StoreVirtualKeys:           &storeKeys,
		PrefixForStoredVirtualKeys: "litellm/",
		AccessMode:                 "read_and_write",
		PrimarySecretName:          "litellm/all-keys",
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings")
	}
	if gs["key_management_system"] != "aws_secret_manager" {
		t.Errorf("expected aws_secret_manager, got %v", gs["key_management_system"])
	}
	kms, ok := gs["key_management_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected key_management_settings")
	}
	hostedKeys, ok := kms["hosted_keys"].([]string)
	if !ok {
		t.Fatal("expected hosted_keys to be []string")
	}
	if len(hostedKeys) != 2 || hostedKeys[0] != "OPENAI_API_KEY" {
		t.Errorf("unexpected hosted_keys: %v", hostedKeys)
	}
	if kms["store_virtual_keys"] != true {
		t.Errorf("expected store_virtual_keys=true, got %v", kms["store_virtual_keys"])
	}
	if kms["prefix_for_stored_virtual_keys"] != "litellm/" {
		t.Errorf("unexpected prefix: %v", kms["prefix_for_stored_virtual_keys"])
	}
	if kms["access_mode"] != "read_and_write" {
		t.Errorf("unexpected access_mode: %v", kms["access_mode"])
	}
	if kms["primary_secret_name"] != "litellm/all-keys" {
		t.Errorf("unexpected primary_secret_name: %v", kms["primary_secret_name"])
	}
}

func TestGenerateProxyConfig_SecretManagerMinimal(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "google_secret_manager",
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings")
	}
	if gs["key_management_system"] != "google_secret_manager" {
		t.Errorf("expected google_secret_manager, got %v", gs["key_management_system"])
	}
	if _, ok := gs["key_management_settings"]; ok {
		t.Error("key_management_settings should be absent when no optional fields are set")
	}
}

func TestGenerateProxyConfig_SecretManagerWithExistingGeneralSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		ProxyBatchWriteAt: 10,
	}
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "hashicorp_vault",
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings")
	}
	// Existing settings preserved
	if gs["proxy_batch_write_at"] != 10 {
		t.Errorf("expected proxy_batch_write_at=10, got %v", gs["proxy_batch_write_at"])
	}
	// Secret manager added
	if gs["key_management_system"] != "hashicorp_vault" {
		t.Errorf("expected hashicorp_vault, got %v", gs["key_management_system"])
	}
}

func TestGenerateProxyConfig_JWTAuthEnabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled:            true,
		AdminJWTScope:      "litellm_proxy_admin",
		AdminAllowedRoutes: []string{"openai_routes", "info_routes"},
		TeamIDJWTField:     "client_id",
		TeamIDsJWTField:    "groups",
		OrgIDJWTField:      "org_id",
		UserIDJWTField:     testJWTFieldSub,
		UserEmailJWTField:  "email",
		UserRoleJWTField:   "role",
		EndUserIDJWTField:  "end_user_id",
		UserIDUpsert:       boolPtr(true),
		PublicKeyTTL:       intPtr(600),
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["enable_jwt_auth"] != true {
		t.Errorf("expected enable_jwt_auth=true, got %v", gs["enable_jwt_auth"])
	}
	jwtauth, ok := gs["litellm_jwtauth"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_jwtauth to be present")
	}
	if jwtauth["admin_jwt_scope"] != "litellm_proxy_admin" {
		t.Errorf("unexpected admin_jwt_scope: %v", jwtauth["admin_jwt_scope"])
	}
	routes, ok := jwtauth["admin_allowed_routes"].([]string)
	if !ok || len(routes) != 2 || routes[0] != "openai_routes" {
		t.Errorf("unexpected admin_allowed_routes: %v", jwtauth["admin_allowed_routes"])
	}
	if jwtauth["team_id_jwt_field"] != "client_id" {
		t.Errorf("unexpected team_id_jwt_field: %v", jwtauth["team_id_jwt_field"])
	}
	if jwtauth["team_ids_jwt_field"] != "groups" {
		t.Errorf("unexpected team_ids_jwt_field: %v", jwtauth["team_ids_jwt_field"])
	}
	if jwtauth["org_id_jwt_field"] != "org_id" {
		t.Errorf("unexpected org_id_jwt_field: %v", jwtauth["org_id_jwt_field"])
	}
	if jwtauth["user_id_jwt_field"] != testJWTFieldSub {
		t.Errorf("unexpected user_id_jwt_field: %v", jwtauth["user_id_jwt_field"])
	}
	if jwtauth["user_email_jwt_field"] != "email" {
		t.Errorf("unexpected user_email_jwt_field: %v", jwtauth["user_email_jwt_field"])
	}
	if jwtauth["user_role_jwt_field"] != "role" {
		t.Errorf("unexpected user_role_jwt_field: %v", jwtauth["user_role_jwt_field"])
	}
	if jwtauth["end_user_id_jwt_field"] != "end_user_id" {
		t.Errorf("unexpected end_user_id_jwt_field: %v", jwtauth["end_user_id_jwt_field"])
	}
	if jwtauth["public_key_ttl"] != 600 {
		t.Errorf("unexpected public_key_ttl: %v", jwtauth["public_key_ttl"])
	}
	if jwtauth["user_id_upsert"] != true {
		t.Errorf("expected user_id_upsert=true, got %v", jwtauth["user_id_upsert"])
	}
}

// TestGenerateProxyConfig_JWTAuthRoleRBAC covers the JWT role-based model-access
// fields: user_roles_jwt_field, user_allowed_roles, enforce_rbac (nested under
// litellm_jwtauth), and object_id_jwt_field.
func TestGenerateProxyConfig_JWTAuthRoleRBAC(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled:           true,
		UserRolesJWTField: "roles",
		UserAllowedRoles:  []string{"basic_user", "power_user"},
		EnforceRBAC:       boolPtr(true),
		ObjectIDJWTField:  "oid",
	}

	config := GenerateProxyConfig(instance, nil)
	gs := config["general_settings"].(map[string]interface{})
	jwtauth, ok := gs["litellm_jwtauth"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_jwtauth to be present")
	}
	if jwtauth["user_roles_jwt_field"] != "roles" {
		t.Errorf("unexpected user_roles_jwt_field: %v", jwtauth["user_roles_jwt_field"])
	}
	roles, ok := jwtauth["user_allowed_roles"].([]string)
	if !ok || len(roles) != 2 || roles[0] != "basic_user" {
		t.Errorf("unexpected user_allowed_roles: %v", jwtauth["user_allowed_roles"])
	}
	if jwtauth["enforce_rbac"] != true {
		t.Errorf("expected enforce_rbac=true under litellm_jwtauth, got %v", jwtauth["enforce_rbac"])
	}
	if jwtauth["object_id_jwt_field"] != "oid" {
		t.Errorf("unexpected object_id_jwt_field: %v", jwtauth["object_id_jwt_field"])
	}
}

func TestGenerateProxyConfig_JWTAuthMinimal(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled: true,
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["enable_jwt_auth"] != true {
		t.Errorf("expected enable_jwt_auth=true, got %v", gs["enable_jwt_auth"])
	}
	// No litellm_jwtauth block when no fields are set
	if _, ok := gs["litellm_jwtauth"]; ok {
		t.Error("litellm_jwtauth should be absent when no optional fields are set")
	}
}

func TestGenerateProxyConfig_JWTAuthScopeModelMappings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled: true,
		ScopeModelMappings: map[string][]string{
			"scope:gpt": {"gpt-4", "gpt-4-mini"},
			"scope:all": {"*"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	jwtauth := gs["litellm_jwtauth"].(map[string]interface{})
	// scopeModelMappings (a map) now renders into scope_mappings (a list), the
	// shape LiteLLM actually reads. Entries are sorted by scope for determinism.
	if _, dead := jwtauth["scope_model_mappings"]; dead {
		t.Error("scope_model_mappings must not be emitted (LiteLLM has no such key)")
	}
	mappings, ok := jwtauth["scope_mappings"].([]map[string]interface{})
	if !ok || len(mappings) != 2 {
		t.Fatalf("expected scope_mappings to be a 2-entry list, got %T %v", jwtauth["scope_mappings"], jwtauth["scope_mappings"])
	}
	// Sorted: "scope:all" < "scope:gpt".
	if mappings[0]["scope"] != "scope:all" {
		t.Errorf("expected first scope=scope:all, got %v", mappings[0]["scope"])
	}
	gptModels, _ := mappings[1]["models"].([]string)
	if mappings[1]["scope"] != "scope:gpt" || len(gptModels) != 2 || gptModels[0] != "gpt-4" {
		t.Errorf("unexpected scope:gpt entry: %v", mappings[1])
	}
}

// TestGenerateProxyConfig_JWTAuthAdvanced covers the full set of advanced
// litellm_jwtauth fields: team/org aliases + upsert + defaults, structured
// scope/role mappings, OIDC UserInfo, virtual-key mapping, routing overrides,
// and the various enforce_* / sync toggles.
func TestGenerateProxyConfig_JWTAuthAdvanced(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled:                     true,
		TeamAliasJWTField:           "team_name",
		OrgAliasJWTField:            "org_name",
		TeamIDDefault:               "default-team",
		TeamAllowedRoutes:           []string{"openai_routes"},
		TeamIDUpsert:                boolPtr(true),
		TeamClaimFallback:           boolPtr(true),
		UserAllowedEmailDomain:      "my-co.com",
		EnforceTeamBasedModelAccess: boolPtr(true),
		EnforceScopeBasedAccess:     boolPtr(true),
		SyncUserRoleAndTeams:        boolPtr(true),
		RolesJWTField:               "resource_access.litellm.roles",
		CustomValidate:              "custom_validate.my_fn",
		VirtualKeyClaimField:        "email",
		VirtualKeyMappingCacheTTL:   intPtr(300),
		OIDCUserInfoEnabled:         boolPtr(true),
		OIDCUserInfoEndpoint:        "https://idp/userinfo",
		OIDCUserInfoCacheTTL:        intPtr(120),
		ScopeMappings: []litellmv1alpha1.JWTScopeMapping{
			{Scope: "litellm.api.consumer", Models: []string{"anthropic-claude"}, Routes: []string{"/v1/chat/completions"}},
		},
		RoleMappings: []litellmv1alpha1.JWTRoleMapping{
			{Role: "litellm.api.consumer", InternalRole: "team"},
		},
		JWTLiteLLMRoleMap: []litellmv1alpha1.JWTLiteLLMRoleMapEntry{
			{JWTRole: "ADMIN", LiteLLMRole: "proxy_admin"},
		},
		RoutingOverrides: []litellmv1alpha1.JWTRoutingOverride{
			{Iss: "issuer.example", ClientID: "MID", Scope: "App:LiteLLM", Aud: "api://litellm", Path: "oauth2"},
		},
	}

	config := GenerateProxyConfig(instance, nil)
	j := config["general_settings"].(map[string]interface{})["litellm_jwtauth"].(map[string]interface{})

	// Scalars / bools.
	checks := map[string]interface{}{
		"team_alias_jwt_field":            "team_name",
		"org_alias_jwt_field":             "org_name",
		"team_id_default":                 "default-team",
		"team_id_upsert":                  true,
		"team_claim_fallback":             true,
		"user_allowed_email_domain":       "my-co.com",
		"enforce_team_based_model_access": true,
		"enforce_scope_based_access":      true,
		"sync_user_role_and_teams":        true,
		"roles_jwt_field":                 "resource_access.litellm.roles",
		"custom_validate":                 "custom_validate.my_fn",
		"virtual_key_claim_field":         "email",
		"virtual_key_mapping_cache_ttl":   300,
		"oidc_userinfo_enabled":           true,
		"oidc_userinfo_endpoint":          "https://idp/userinfo",
		"oidc_userinfo_cache_ttl":         120,
	}
	for k, want := range checks {
		if j[k] != want {
			t.Errorf("jwtauth[%q] = %v, want %v", k, j[k], want)
		}
	}
	if routes, _ := j["team_allowed_routes"].([]string); len(routes) != 1 || routes[0] != "openai_routes" {
		t.Errorf("unexpected team_allowed_routes: %v", j["team_allowed_routes"])
	}

	// Structured lists.
	sm := j["scope_mappings"].([]map[string]interface{})
	if len(sm) != 1 || sm[0]["scope"] != "litellm.api.consumer" {
		t.Fatalf("unexpected scope_mappings: %v", sm)
	}
	if rts, _ := sm[0]["routes"].([]string); len(rts) != 1 || rts[0] != "/v1/chat/completions" {
		t.Errorf("expected scope_mappings routes preserved, got %v", sm[0]["routes"])
	}
	rm := j["role_mappings"].([]map[string]interface{})
	if len(rm) != 1 || rm[0]["role"] != "litellm.api.consumer" || rm[0]["internal_role"] != "team" {
		t.Errorf("unexpected role_mappings: %v", rm)
	}
	jrm := j["jwt_litellm_role_map"].([]map[string]interface{})
	if len(jrm) != 1 || jrm[0]["jwt_role"] != "ADMIN" || jrm[0]["litellm_role"] != "proxy_admin" {
		t.Errorf("unexpected jwt_litellm_role_map: %v", jrm)
	}
	ro := j["routing_overrides"].([]map[string]interface{})
	if len(ro) != 1 || ro[0]["iss"] != "issuer.example" || ro[0]["client_id"] != "MID" || ro[0]["path"] != "oauth2" {
		t.Errorf("unexpected routing_overrides: %v", ro)
	}
}

func TestGenerateProxyConfig_JWTAuthDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled:        false,
		AdminJWTScope:  "admin",
		UserIDJWTField: testJWTFieldSub,
	}

	config := GenerateProxyConfig(instance, nil)

	// When disabled, no JWT auth config should be written
	gs, ok := config["general_settings"].(map[string]interface{})
	if ok {
		if _, found := gs["enable_jwt_auth"]; found {
			t.Error("enable_jwt_auth should not be present when JWT auth is disabled")
		}
	}
}

func TestGenerateProxyConfig_OAuth2AuthEnabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.OAuth2Auth = &litellmv1alpha1.OAuth2AuthSpec{
		Enabled: true,
		ConfigMappings: []litellmv1alpha1.OAuth2Mapping{
			{Name: "clientId", JWTField: "client_id", LiteLLMAttribute: "team_id"},
			{Name: "userId", JWTField: testJWTFieldSub, LiteLLMAttribute: "user_id"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["enable_oauth2_auth"] != true {
		t.Errorf("expected enable_oauth2_auth=true, got %v", gs["enable_oauth2_auth"])
	}
	mappings, ok := gs["oauth2_config_mappings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected oauth2_config_mappings to be present")
	}
	clientId, ok := mappings["clientId"].(map[string]interface{})
	if !ok {
		t.Fatal("expected clientId mapping to be present")
	}
	if clientId["jwt_field"] != "client_id" {
		t.Errorf("unexpected jwt_field: %v", clientId["jwt_field"])
	}
	if clientId["litellm_attribute"] != "team_id" {
		t.Errorf("unexpected litellm_attribute: %v", clientId["litellm_attribute"])
	}
	userId, ok := mappings["userId"].(map[string]interface{})
	if !ok {
		t.Fatal("expected userId mapping to be present")
	}
	if userId["jwt_field"] != testJWTFieldSub {
		t.Errorf("unexpected jwt_field: %v", userId["jwt_field"])
	}
	if userId["litellm_attribute"] != "user_id" {
		t.Errorf("unexpected litellm_attribute: %v", userId["litellm_attribute"])
	}
}

func TestGenerateProxyConfig_OAuth2AuthMinimal(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.OAuth2Auth = &litellmv1alpha1.OAuth2AuthSpec{
		Enabled: true,
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["enable_oauth2_auth"] != true {
		t.Errorf("expected enable_oauth2_auth=true, got %v", gs["enable_oauth2_auth"])
	}
	if _, ok := gs["oauth2_config_mappings"]; ok {
		t.Error("oauth2_config_mappings should be absent when no mappings are configured")
	}
}

func TestGenerateProxyConfig_OAuth2AuthDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.OAuth2Auth = &litellmv1alpha1.OAuth2AuthSpec{
		Enabled: false,
		ConfigMappings: []litellmv1alpha1.OAuth2Mapping{
			{Name: "clientId", JWTField: "client_id", LiteLLMAttribute: "team_id"},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if ok {
		if _, found := gs["enable_oauth2_auth"]; found {
			t.Error("enable_oauth2_auth should not be present when OAuth2 auth is disabled")
		}
	}
}

func TestGenerateProxyConfig_JWTAuthWithExistingGeneralSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		ProxyBatchWriteAt: 10,
	}
	instance.Spec.JWTAuth = &litellmv1alpha1.JWTAuthSpec{
		Enabled:        true,
		UserIDJWTField: testJWTFieldSub,
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	// Existing settings preserved
	if gs["proxy_batch_write_at"] != 10 {
		t.Errorf("expected proxy_batch_write_at=10, got %v", gs["proxy_batch_write_at"])
	}
	// JWT auth added
	if gs["enable_jwt_auth"] != true {
		t.Errorf("expected enable_jwt_auth=true, got %v", gs["enable_jwt_auth"])
	}
	jwtauth := gs["litellm_jwtauth"].(map[string]interface{})
	if jwtauth["user_id_jwt_field"] != testJWTFieldSub {
		t.Errorf("unexpected user_id_jwt_field: %v", jwtauth["user_id_jwt_field"])
	}
}

func TestGenerateProxyConfig_RBACEnabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled: true,
	}

	config := GenerateProxyConfig(instance, nil)

	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["enforce_rbac"] != true {
		t.Errorf("expected enforce_rbac=true, got %v", gs["enforce_rbac"])
	}
}

func TestGenerateProxyConfig_RBACDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled: false,
	}

	config := GenerateProxyConfig(instance, nil)

	if gs, ok := config["general_settings"].(map[string]interface{}); ok {
		if _, ok := gs["enforce_rbac"]; ok {
			t.Error("enforce_rbac should not be present when RBAC is disabled")
		}
	}
}

func TestGenerateProxyConfig_RBACAdminOnlyRoutes(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled:         true,
		AdminOnlyRoutes: []string{testRouteModelNew, "/model/delete", "/organization/new"},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	routes, ok := gs["admin_only_routes"].([]string)
	if !ok {
		t.Fatal("expected admin_only_routes to be []string")
	}
	if len(routes) != 3 || routes[0] != testRouteModelNew || routes[2] != "/organization/new" {
		t.Errorf("unexpected admin_only_routes: %v", routes)
	}
}

func TestGenerateProxyConfig_RBACAllowedRoutes(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled:       true,
		AllowedRoutes: []string{"/chat/completions", "/embeddings", "/key/info"},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	routes, ok := gs["allowed_routes"].([]string)
	if !ok {
		t.Fatal("expected allowed_routes to be []string")
	}
	if len(routes) != 3 || routes[0] != "/chat/completions" {
		t.Errorf("unexpected allowed_routes: %v", routes)
	}
}

func TestGenerateProxyConfig_RBACDefaultTeamDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled:             true,
		DefaultTeamDisabled: boolPtr(true),
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["default_team_disabled"] != true {
		t.Errorf("expected default_team_disabled=true, got %v", gs["default_team_disabled"])
	}
}

func TestGenerateProxyConfig_RBACKeyGeneration(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled: true,
		KeyGeneration: &litellmv1alpha1.KeyGenerationSettings{
			TeamKeyGeneration: &litellmv1alpha1.TeamKeyGenerationSettings{
				AllowedTeamMemberRoles: []string{"admin"},
			},
			PersonalKeyGeneration: &litellmv1alpha1.PersonalKeyGenerationSettings{
				AllowedUserRoles: []string{"proxy_admin"},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	kgs, ok := gs["key_generation_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected key_generation_settings to be present")
	}

	tkg, ok := kgs["team_key_generation"].(map[string]interface{})
	if !ok {
		t.Fatal("expected team_key_generation to be present")
	}
	roles, ok := tkg["allowed_team_member_roles"].([]string)
	if !ok {
		t.Fatal("expected allowed_team_member_roles to be []string")
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("unexpected allowed_team_member_roles: %v", roles)
	}

	pkg, ok := kgs["personal_key_generation"].(map[string]interface{})
	if !ok {
		t.Fatal("expected personal_key_generation to be present")
	}
	userRoles, ok := pkg["allowed_user_roles"].([]string)
	if !ok {
		t.Fatal("expected allowed_user_roles to be []string")
	}
	if len(userRoles) != 1 || userRoles[0] != "proxy_admin" {
		t.Errorf("unexpected allowed_user_roles: %v", userRoles)
	}
}

func TestGenerateProxyConfig_RBACRolePermissions(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled: true,
		RolePermissions: map[string]litellmv1alpha1.RolePermission{
			"internal_user": {
				Routes: []string{"/key/generate", "/key/delete", "/key/info"},
				Models: []string{"gpt-4", testModelClaude3Haiku},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	// LiteLLM requires role_permissions to be a LIST of {role, models, routes};
	// a map crashes proxy startup. Guard against regressing to the map form.
	if _, isMap := gs["role_permissions"].(map[string]interface{}); isMap {
		t.Fatal("role_permissions must be a list, not a map (map form crashes LiteLLM startup)")
	}
	rp, ok := gs["role_permissions"].([]interface{})
	if !ok || len(rp) != 1 {
		t.Fatalf("expected role_permissions to be a 1-entry list, got %T %v", gs["role_permissions"], gs["role_permissions"])
	}
	iu := rp[0].(map[string]interface{})
	if iu["role"] != "internal_user" {
		t.Errorf("expected entry role=internal_user, got %v", iu["role"])
	}
	routes, ok := iu["routes"].([]string)
	if !ok {
		t.Fatal("expected routes to be []string")
	}
	if len(routes) != 3 || routes[0] != "/key/generate" {
		t.Errorf("unexpected routes: %v", routes)
	}
	models, ok := iu["models"].([]string)
	if !ok {
		t.Fatal("expected models to be []string")
	}
	if len(models) != 2 || models[0] != "gpt-4" || models[1] != testModelClaude3Haiku {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestGenerateProxyConfig_RBACFull(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled:             true,
		AdminOnlyRoutes:     []string{testRouteModelNew},
		AllowedRoutes:       []string{"/chat/completions"},
		DefaultTeamDisabled: boolPtr(true),
		KeyGeneration: &litellmv1alpha1.KeyGenerationSettings{
			TeamKeyGeneration: &litellmv1alpha1.TeamKeyGenerationSettings{
				AllowedTeamMemberRoles: []string{"admin"},
			},
		},
		RolePermissions: map[string]litellmv1alpha1.RolePermission{
			"internal_user": {
				Routes: []string{"/key/generate"},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["enforce_rbac"] != true {
		t.Errorf("expected enforce_rbac=true, got %v", gs["enforce_rbac"])
	}
	if gs["default_team_disabled"] != true {
		t.Errorf("expected default_team_disabled=true, got %v", gs["default_team_disabled"])
	}

	adminRoutes := gs["admin_only_routes"].([]string)
	if len(adminRoutes) != 1 || adminRoutes[0] != testRouteModelNew {
		t.Errorf("unexpected admin_only_routes: %v", adminRoutes)
	}

	allowedRoutes := gs["allowed_routes"].([]string)
	if len(allowedRoutes) != 1 || allowedRoutes[0] != "/chat/completions" {
		t.Errorf("unexpected allowed_routes: %v", allowedRoutes)
	}

	kgs := gs["key_generation_settings"].(map[string]interface{})
	if _, ok := kgs["team_key_generation"]; !ok {
		t.Error("expected team_key_generation in key_generation_settings")
	}

	rp := gs["role_permissions"].([]interface{})
	if len(rp) != 1 || rp[0].(map[string]interface{})["role"] != "internal_user" {
		t.Errorf("expected a single internal_user entry in role_permissions list, got %v", rp)
	}
}

func TestGenerateProxyConfig_RBACWithExistingGeneralSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.GeneralSettings = &litellmv1alpha1.GeneralSettingsSpec{
		ProxyBatchWriteAt: 10,
	}
	instance.Spec.RBAC = &litellmv1alpha1.RBACSpec{
		Enabled:         true,
		AdminOnlyRoutes: []string{testRouteModelNew},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["proxy_batch_write_at"] != 10 {
		t.Errorf("expected proxy_batch_write_at=10, got %v", gs["proxy_batch_write_at"])
	}
	if gs["enforce_rbac"] != true {
		t.Errorf("expected enforce_rbac=true, got %v", gs["enforce_rbac"])
	}
	routes := gs["admin_only_routes"].([]string)
	if len(routes) != 1 || routes[0] != testRouteModelNew {
		t.Errorf("unexpected admin_only_routes: %v", routes)
	}
}

func TestGenerateProxyConfig_LoggingAuditLogs(t *testing.T) {
	instance := newTestInstance()
	retDays := 90
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		AuditLogs: &litellmv1alpha1.AuditLogSpec{
			Enabled:       true,
			RetentionDays: &retDays,
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["store_audit_logs"] != true {
		t.Errorf("expected store_audit_logs=true, got %v", gs["store_audit_logs"])
	}
	ls := config["litellm_settings"].(map[string]interface{})
	if ls["audit_log_retention_days"] != 90 {
		t.Errorf("expected audit_log_retention_days=90, got %v", ls["audit_log_retention_days"])
	}
}

func TestGenerateProxyConfig_LoggingAuditLogsNoRetention(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		AuditLogs: &litellmv1alpha1.AuditLogSpec{
			Enabled: true,
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["store_audit_logs"] != true {
		t.Errorf("expected store_audit_logs=true, got %v", gs["store_audit_logs"])
	}
	// No litellm_settings.audit_log_retention_days when not set
	if _, ok := config["litellm_settings"]; ok {
		ls := config["litellm_settings"].(map[string]interface{})
		if _, has := ls["audit_log_retention_days"]; has {
			t.Error("audit_log_retention_days should not be set when RetentionDays is nil")
		}
	}
}

func TestGenerateProxyConfig_LoggingAuditLogsDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		AuditLogs: &litellmv1alpha1.AuditLogSpec{
			Enabled: false,
		},
	}

	config := GenerateProxyConfig(instance, nil)

	if gs, ok := config["general_settings"].(map[string]interface{}); ok {
		if _, has := gs["store_audit_logs"]; has {
			t.Error("store_audit_logs should not be set when audit logs are disabled")
		}
	}
}

func TestGenerateProxyConfig_LoggingTurnOffMessageLogging(t *testing.T) {
	instance := newTestInstance()
	turnOff := true
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		TurnOffMessageLogging: &turnOff,
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	if ls["turn_off_message_logging"] != true {
		t.Errorf("expected turn_off_message_logging=true, got %v", ls["turn_off_message_logging"])
	}
}

func TestGenerateProxyConfig_LoggingRedactUserAPIKeyInfo(t *testing.T) {
	instance := newTestInstance()
	redact := true
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		RedactUserAPIKeyInfo: &redact,
	}

	config := GenerateProxyConfig(instance, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	if ls["redact_user_api_key_info"] != true {
		t.Errorf("expected redact_user_api_key_info=true, got %v", ls["redact_user_api_key_info"])
	}
}

func TestGenerateProxyConfig_LoggingSpendLogRetention(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		SpendLogRetention: &litellmv1alpha1.SpendLogRetentionSpec{
			MaxRetentionPeriod: "90d",
			CleanupInterval:    "1d",
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["maximum_spend_logs_retention_period"] != "90d" {
		t.Errorf("expected maximum_spend_logs_retention_period=90d, got %v", gs["maximum_spend_logs_retention_period"])
	}
	if gs["maximum_spend_logs_retention_interval"] != "1d" {
		t.Errorf("expected maximum_spend_logs_retention_interval=1d, got %v", gs["maximum_spend_logs_retention_interval"])
	}
}

func TestGenerateProxyConfig_LoggingAllSettings(t *testing.T) {
	instance := newTestInstance()
	turnOff := true
	redact := true
	retDays := 30
	instance.Spec.Logging = &litellmv1alpha1.InstanceLoggingSpec{
		AuditLogs: &litellmv1alpha1.AuditLogSpec{
			Enabled:       true,
			RetentionDays: &retDays,
		},
		TurnOffMessageLogging: &turnOff,
		RedactUserAPIKeyInfo:  &redact,
		SpendLogRetention: &litellmv1alpha1.SpendLogRetentionSpec{
			MaxRetentionPeriod: "1y",
			CleanupInterval:    "1h",
		},
	}

	config := GenerateProxyConfig(instance, nil)

	gs := config["general_settings"].(map[string]interface{})
	if gs["store_audit_logs"] != true {
		t.Errorf("expected store_audit_logs=true")
	}
	if gs["maximum_spend_logs_retention_period"] != "1y" {
		t.Errorf("expected retention_period=1y, got %v", gs["maximum_spend_logs_retention_period"])
	}
	if gs["maximum_spend_logs_retention_interval"] != "1h" {
		t.Errorf("expected retention_interval=1h, got %v", gs["maximum_spend_logs_retention_interval"])
	}

	ls := config["litellm_settings"].(map[string]interface{})
	if ls["audit_log_retention_days"] != 30 {
		t.Errorf("expected audit_log_retention_days=30, got %v", ls["audit_log_retention_days"])
	}
	if ls["turn_off_message_logging"] != true {
		t.Errorf("expected turn_off_message_logging=true")
	}
	if ls["redact_user_api_key_info"] != true {
		t.Errorf("expected redact_user_api_key_info=true")
	}
}

func TestGenerateProxyConfig_LoggingNil(t *testing.T) {
	instance := newTestInstance()
	// No Logging spec set — should not produce any logging settings
	config := GenerateProxyConfig(instance, nil)

	if gs, ok := config["general_settings"].(map[string]interface{}); ok {
		if _, has := gs["store_audit_logs"]; has {
			t.Error("store_audit_logs should not be set when Logging is nil")
		}
	}
	if ls, ok := config["litellm_settings"].(map[string]interface{}); ok {
		for _, key := range []string{"turn_off_message_logging", "redact_user_api_key_info", "audit_log_retention_days"} {
			if _, has := ls[key]; has {
				t.Errorf("%s should not be set when Logging is nil", key)
			}
		}
	}
}

func TestGenerateProxyConfig_AdminUIAdminOnly(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.AdminUI = &litellmv1alpha1.AdminUISpec{
		AdminOnly: boolPtr(true),
	}
	config := GenerateProxyConfig(instance, nil)
	gs := config["general_settings"].(map[string]interface{})
	if gs["ui_access_mode"] != uiAccessModeAdminOnly {
		t.Errorf("expected ui_access_mode=admin_only, got %v", gs["ui_access_mode"])
	}
}

func TestGenerateProxyConfig_AdminUIAdminOnlyFalse(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.AdminUI = &litellmv1alpha1.AdminUISpec{
		AdminOnly: boolPtr(false),
	}
	config := GenerateProxyConfig(instance, nil)
	if gs, ok := config["general_settings"].(map[string]interface{}); ok {
		if _, has := gs["ui_access_mode"]; has {
			t.Error("ui_access_mode should not be set when adminOnly is false")
		}
	}
}

func TestGenerateProxyConfig_AdminUIStoreModelInDB(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.AdminUI = &litellmv1alpha1.AdminUISpec{
		StoreModelInDB: boolPtr(true),
	}
	config := GenerateProxyConfig(instance, nil)
	gs := config["general_settings"].(map[string]interface{})
	if gs["store_model_in_db"] != true {
		t.Errorf("expected store_model_in_db=true, got %v", gs["store_model_in_db"])
	}
}

func TestGenerateProxyConfig_AdminUIStoreModelInDBFalse(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.AdminUI = &litellmv1alpha1.AdminUISpec{
		StoreModelInDB: boolPtr(false),
	}
	config := GenerateProxyConfig(instance, nil)
	gs := config["general_settings"].(map[string]interface{})
	if gs["store_model_in_db"] != false {
		t.Errorf("expected store_model_in_db=false, got %v", gs["store_model_in_db"])
	}
}

func TestGenerateProxyConfig_AdminUIDefaultTeamDisabled(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.AdminUI = &litellmv1alpha1.AdminUISpec{
		DefaultTeamDisabled: boolPtr(true),
	}
	config := GenerateProxyConfig(instance, nil)
	gs := config["general_settings"].(map[string]interface{})
	if gs["default_team_disabled"] != true {
		t.Errorf("expected default_team_disabled=true, got %v", gs["default_team_disabled"])
	}
}

func TestGenerateProxyConfig_AdminUIAllSettings(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.AdminUI = &litellmv1alpha1.AdminUISpec{
		AdminOnly:           boolPtr(true),
		StoreModelInDB:      boolPtr(true),
		DefaultTeamDisabled: boolPtr(true),
	}
	config := GenerateProxyConfig(instance, nil)
	gs := config["general_settings"].(map[string]interface{})
	if gs["ui_access_mode"] != uiAccessModeAdminOnly {
		t.Errorf("expected ui_access_mode=admin_only, got %v", gs["ui_access_mode"])
	}
	if gs["store_model_in_db"] != true {
		t.Errorf("expected store_model_in_db=true")
	}
	if gs["default_team_disabled"] != true {
		t.Errorf("expected default_team_disabled=true")
	}
}

func TestGenerateProxyConfig_AdminUINil(t *testing.T) {
	instance := newTestInstance()
	config := GenerateProxyConfig(instance, nil)
	if gs, ok := config["general_settings"].(map[string]interface{}); ok {
		for _, key := range []string{"ui_access_mode", "store_model_in_db", "default_team_disabled"} {
			if _, has := gs[key]; has {
				t.Errorf("%s should not be set when AdminUI is nil", key)
			}
		}
	}
}

func TestGenerateProxyConfig_SSOCustomHandlerModule(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SSO = &litellmv1alpha1.SSOSpec{
		Enabled:  true,
		Provider: "generic-oidc",
		ClientID: litellmv1alpha1.SecretKeyRef{
			Name: "sso", Key: "id",
		},
		ClientSecret: litellmv1alpha1.SecretKeyRef{
			Name: "sso", Key: "secret",
		},
		CustomSSOHandler: &litellmv1alpha1.CustomSSOHandlerSpec{
			Module: "my_package.my_handler",
		},
	}

	config := GenerateProxyConfig(instance, nil)
	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	if gs["custom_sso"] != "my_package.my_handler" {
		t.Errorf("expected custom_sso=my_package.my_handler, got %v", gs["custom_sso"])
	}
}

func TestGenerateProxyConfig_SSOCustomHandlerConfigMap(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SSO = &litellmv1alpha1.SSOSpec{
		Enabled:  true,
		Provider: "generic-oidc",
		ClientID: litellmv1alpha1.SecretKeyRef{
			Name: "sso", Key: "id",
		},
		ClientSecret: litellmv1alpha1.SecretKeyRef{
			Name: "sso", Key: "secret",
		},
		CustomSSOHandler: &litellmv1alpha1.CustomSSOHandlerSpec{
			ConfigMapRef: &litellmv1alpha1.CustomSSOHandlerConfigMapRef{
				Name:         "my-sso-handler",
				FileName:     "handler.py",
				FunctionName: "handle_sso",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)
	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected general_settings to be present")
	}
	want := "custom_sso_handlers.handler.handle_sso"
	if gs["custom_sso"] != want {
		t.Errorf("expected custom_sso=%s, got %v", want, gs["custom_sso"])
	}
}

func TestGenerateProxyConfig_SSODefaultUserParamsTeams(t *testing.T) {
	instance := newTestInstance()
	maxBudget := 50.0
	instance.Spec.SSO = &litellmv1alpha1.SSOSpec{
		Enabled:  true,
		Provider: "generic-oidc",
		ClientID: litellmv1alpha1.SecretKeyRef{
			Name: "sso", Key: "id",
		},
		ClientSecret: litellmv1alpha1.SecretKeyRef{
			Name: "sso", Key: "secret",
		},
		DefaultUserParams: &litellmv1alpha1.DefaultUserParams{
			UserRole: "internal_user",
			Teams: []litellmv1alpha1.DefaultUserTeam{
				{TeamID: "team-a", Role: "user"},
				{TeamID: "team-b", Role: "admin", MaxBudgetInTeam: &maxBudget},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil)
	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected litellm_settings to be present")
	}
	dup, ok := ls["default_internal_user_params"].(map[string]interface{})
	if !ok {
		t.Fatal("expected default_internal_user_params to be present")
	}
	teams, ok := dup["teams"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected teams to be []map[string]interface{}, got %T", dup["teams"])
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	if teams[0]["team_id"] != "team-a" || teams[0]["user_role"] != "user" {
		t.Errorf("unexpected first team entry: %v", teams[0])
	}
	if teams[1]["team_id"] != "team-b" || teams[1]["user_role"] != "admin" {
		t.Errorf("unexpected second team entry: %v", teams[1])
	}
	if teams[1]["max_budget_in_team"] != 50.0 {
		t.Errorf("expected max_budget_in_team=50.0, got %v", teams[1]["max_budget_in_team"])
	}
	if _, has := teams[0]["max_budget_in_team"]; has {
		t.Error("team without budget should not have max_budget_in_team")
	}
}

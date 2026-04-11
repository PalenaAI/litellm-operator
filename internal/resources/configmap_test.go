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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func intPtr(v int) *int { return &v }

func TestGenerateProxyConfig_DefaultFallbacks(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Fallbacks = &litellmv1alpha1.FallbackSpec{
		DefaultFallbacks: []string{"gpt-4-mini", "claude-3-haiku"},
	}

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

	if _, ok := config["litellm_settings"]; ok {
		t.Error("litellm_settings should not be present when caching is not configured")
	}
}

func TestGenerateProxyConfig_CachingEnabledFalse(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: false,
		Type:    "redis",
	}

	config := GenerateProxyConfig(instance, nil, nil)

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
		Type:               "redis",
		Namespace:          "my-ns",
		TTL:                &ttl,
		SupportedCallTypes: []string{"acompletion", "aembedding"},
		Mode:               "default_off",
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

	config := GenerateProxyConfig(instance, nil, nil)

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
	if params["type"] != "redis" {
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
	if params["mode"] != "default_off" {
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
		Type:    "redis",
		// No Redis block — should reuse instance Redis
	}

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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
		Type:    "qdrant",
		Qdrant: &litellmv1alpha1.CacheQdrantSpec{
			URL:            "http://qdrant:6333",
			CollectionName: "llm-cache",
			APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "qdrant-secret",
				Key:  "api-key",
			},
		},
	}

	config := GenerateProxyConfig(instance, nil, nil)

	ls := config["litellm_settings"].(map[string]interface{})
	params := ls["cache_params"].(map[string]interface{})

	if params["type"] != "qdrant" {
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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, nil)

	if ls, ok := config["litellm_settings"].(map[string]interface{}); ok {
		if _, present := ls["max_end_user_budget"]; present {
			t.Error("expected no max_end_user_budget when spec is empty")
		}
		if _, present := ls["max_end_user_budget_id"]; present {
			t.Error("expected no max_end_user_budget_id when spec is empty")
		}
	}
}

func TestGenerateProxyConfig_CredentialListNoCredentials(t *testing.T) {
	instance := newTestInstance()

	config := GenerateProxyConfig(instance, nil, nil)

	if _, present := config["credential_list"]; present {
		t.Error("credential_list should be absent when no credentials are provided")
	}
}

func TestGenerateProxyConfig_CredentialListSingleCredential(t *testing.T) {
	instance := newTestInstance()
	credentials := []litellmv1alpha1.LiteLLMCredential{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "openai-prod", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:    litellmv1alpha1.InstanceRef{Name: "test-instance"},
				CredentialName: "openai-prod",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{
					Name: "openai-secret",
					Key:  "api-key",
				},
			},
		},
	}

	config := GenerateProxyConfig(instance, credentials, nil)

	entries, ok := config["credential_list"].([]map[string]interface{})
	if !ok {
		t.Fatalf("credential_list should be []map[string]interface{}, got %T", config["credential_list"])
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 credential entry, got %d", len(entries))
	}
	if entries[0]["credential_name"] != "openai-prod" {
		t.Errorf("expected credential_name=openai-prod, got %v", entries[0]["credential_name"])
	}
	info, ok := entries[0]["credential_info"].(map[string]interface{})
	if !ok {
		t.Fatal("credential_info should be a map")
	}
	if info["api_key"] != "os.environ/CREDENTIAL_OPENAI_PROD_API_KEY" {
		t.Errorf("expected os.environ/CREDENTIAL_OPENAI_PROD_API_KEY, got %v", info["api_key"])
	}
}

func TestGenerateProxyConfig_CredentialListWithAPIBaseAndParams(t *testing.T) {
	instance := newTestInstance()
	credentials := []litellmv1alpha1.LiteLLMCredential{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "azure-east", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "test-instance"},
				CredentialName:  "azure-east",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "azure-secret", Key: "key"},
				APIBase:         "https://azure-east.openai.azure.com",
				APIVersion:      "2024-02-01",
				Params: map[string]string{
					"azure_ad_token": "token-value",
					// api_base is reserved — should not override
					"api_base": "https://malicious.example.com",
				},
			},
		},
	}

	config := GenerateProxyConfig(instance, credentials, nil)

	entries, _ := config["credential_list"].([]map[string]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	info, _ := entries[0]["credential_info"].(map[string]interface{})

	if info["api_base"] != "https://azure-east.openai.azure.com" {
		t.Errorf("api_base was overridden by params: %v", info["api_base"])
	}
	if info["api_version"] != "2024-02-01" {
		t.Errorf("expected api_version=2024-02-01, got %v", info["api_version"])
	}
	if info["azure_ad_token"] != "token-value" {
		t.Errorf("expected azure_ad_token passthrough, got %v", info["azure_ad_token"])
	}
}

func TestGenerateProxyConfig_CredentialListFiltersByInstance(t *testing.T) {
	instance := newTestInstance() // name: test-instance
	credentials := []litellmv1alpha1.LiteLLMCredential{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "mine", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "test-instance"},
				CredentialName:  "mine",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "theirs", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "other-instance"},
				CredentialName:  "theirs",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
			},
		},
	}

	config := GenerateProxyConfig(instance, credentials, nil)

	entries, _ := config["credential_list"].([]map[string]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (filtered), got %d", len(entries))
	}
	if entries[0]["credential_name"] != "mine" {
		t.Errorf("expected mine, got %v", entries[0]["credential_name"])
	}
}

func TestCredentialEnvVarName_Sanitization(t *testing.T) {
	cases := map[string]string{
		"openai-prod":           "CREDENTIAL_OPENAI_PROD_API_KEY",
		"azure.east.1":          "CREDENTIAL_AZURE_EAST_1_API_KEY",
		"cred with spaces":      "CREDENTIAL_CRED_WITH_SPACES_API_KEY",
		"already_underscored":   "CREDENTIAL_ALREADY_UNDERSCORED_API_KEY",
		"mixed-case_Cred.Value": "CREDENTIAL_MIXED_CASE_CRED_VALUE_API_KEY",
	}
	for in, want := range cases {
		got := CredentialEnvVarName(in)
		if got != want {
			t.Errorf("CredentialEnvVarName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateProxyConfig_GuardrailsNone(t *testing.T) {
	instance := newTestInstance()

	config := GenerateProxyConfig(instance, nil, nil)

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

	config := GenerateProxyConfig(instance, nil, guardrails)

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

	config := GenerateProxyConfig(instance, nil, guardrails)

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
				Params: map[string]string{
					"guardrailIdentifier": "abc123",
					"guardrailVersion":    "DRAFT",
					// reserved keys — should be rejected
					"guardrail": "lakera",
					"mode":      "during_call",
				},
			},
		},
	}

	config := GenerateProxyConfig(instance, nil, guardrails)

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

	config := GenerateProxyConfig(instance, nil, guardrails)

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

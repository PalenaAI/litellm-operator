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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildConfigMap creates the ConfigMap containing proxy_server_config.yaml.
func BuildConfigMap(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) (*corev1.ConfigMap, error) {
	config := GenerateProxyConfig(instance)
	configYAML, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-config",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"proxy_server_config.yaml": string(configYAML),
		},
	}, nil
}

// GenerateProxyConfig generates the proxy_server_config structure from the instance spec.
func GenerateProxyConfig(instance *litellmv1alpha1.LiteLLMInstance) map[string]interface{} {
	config := map[string]interface{}{
		"model_list": []interface{}{},
	}

	// General settings
	if instance.Spec.GeneralSettings != nil {
		gs := map[string]interface{}{}
		if instance.Spec.GeneralSettings.ProxyBatchWriteAt > 0 {
			gs["proxy_batch_write_at"] = instance.Spec.GeneralSettings.ProxyBatchWriteAt
		}
		if instance.Spec.GeneralSettings.MasterKeyRequired != nil {
			gs["master_key_required"] = *instance.Spec.GeneralSettings.MasterKeyRequired
		}
		if len(instance.Spec.GeneralSettings.AlertTypes) > 0 {
			gs["alert_types"] = instance.Spec.GeneralSettings.AlertTypes
		}
		if instance.Spec.GeneralSettings.AllowUserAuth != nil {
			gs["allow_user_auth"] = *instance.Spec.GeneralSettings.AllowUserAuth
		}
		if len(gs) > 0 {
			config["general_settings"] = gs
		}
	}

	// Router settings
	if instance.Spec.RouterSettings != nil {
		rs := map[string]interface{}{}
		if instance.Spec.RouterSettings.RoutingStrategy != "" {
			rs["routing_strategy"] = instance.Spec.RouterSettings.RoutingStrategy
		}
		if instance.Spec.RouterSettings.NumRetries != nil {
			rs["num_retries"] = *instance.Spec.RouterSettings.NumRetries
		}
		if instance.Spec.RouterSettings.Timeout != nil {
			rs["timeout"] = *instance.Spec.RouterSettings.Timeout
		}
		if instance.Spec.RouterSettings.AllowedFails != nil {
			rs["allowed_fails"] = *instance.Spec.RouterSettings.AllowedFails
		}
		if instance.Spec.RouterSettings.CooldownTime != nil {
			rs["cooldown_time"] = *instance.Spec.RouterSettings.CooldownTime
		}
		if len(instance.Spec.RouterSettings.RetryPolicy) > 0 {
			rs["retry_policy"] = instance.Spec.RouterSettings.RetryPolicy
		}
		if len(instance.Spec.RouterSettings.ModelGroupRetryPolicy) > 0 {
			rs["model_group_retry_policy"] = instance.Spec.RouterSettings.ModelGroupRetryPolicy
		}
		if instance.Spec.RouterSettings.EnableTagFiltering != nil && *instance.Spec.RouterSettings.EnableTagFiltering {
			rs["enable_tag_filtering"] = true
		}
		if instance.Spec.RouterSettings.TagFilteringMatchAny != nil {
			rs["tag_filtering_match_any"] = *instance.Spec.RouterSettings.TagFilteringMatchAny
		}
		if len(rs) > 0 {
			config["router_settings"] = rs
		}
	}

	// Fallback configuration
	if instance.Spec.Fallbacks != nil {
		buildFallbackConfig(instance.Spec.Fallbacks, config)
	}

	// SSO litellm_settings
	if instance.Spec.SSO != nil && instance.Spec.SSO.Enabled {
		ls := map[string]interface{}{}
		if instance.Spec.SSO.DefaultUserParams != nil {
			dup := mapDefaultUserParams(instance.Spec.SSO.DefaultUserParams)
			ls["default_internal_user_params"] = dup
		}
		if instance.Spec.SSO.DefaultTeamParams != nil {
			dtp := mapDefaultTeamParams(instance.Spec.SSO.DefaultTeamParams)
			ls["default_team_params"] = dtp
		}
		if len(ls) > 0 {
			config["litellm_settings"] = ls
		}

		if instance.Spec.SSO.TeamIDsJWTField != "" {
			gs, ok := config["general_settings"].(map[string]interface{})
			if !ok {
				gs = map[string]interface{}{}
				config["general_settings"] = gs
			}
			gs["litellm_jwtauth"] = map[string]interface{}{
				"team_ids_jwt_field": instance.Spec.SSO.TeamIDsJWTField,
			}
		}
	}

	// Callbacks
	if instance.Spec.Callbacks != nil && len(instance.Spec.Callbacks.Types) > 0 {
		ls, ok := config["litellm_settings"].(map[string]interface{})
		if !ok {
			ls = map[string]interface{}{}
			config["litellm_settings"] = ls
		}
		ls["success_callback"] = instance.Spec.Callbacks.Types
		ls["failure_callback"] = instance.Spec.Callbacks.Types
	}

	// Caching
	if instance.Spec.Caching != nil && instance.Spec.Caching.Enabled {
		buildCachingConfig(instance, config)
	}

	// IP allowlist (enterprise)
	if instance.Spec.Security != nil && instance.Spec.Security.IPAllowlist != nil && instance.Spec.Security.IPAllowlist.Enabled {
		buildIPAllowlistConfig(instance.Spec.Security.IPAllowlist, config)
	}

	return config
}

// buildIPAllowlistConfig writes IP allowlist settings into general_settings.
func buildIPAllowlistConfig(ipAllowlist *litellmv1alpha1.IPAllowlistSpec, config map[string]interface{}) {
	gs, ok := config["general_settings"].(map[string]interface{})
	if !ok {
		gs = map[string]interface{}{}
		config["general_settings"] = gs
	}

	gs["allowed_ips"] = ipAllowlist.AllowedIPs

	if ipAllowlist.UseXForwardedFor != nil && *ipAllowlist.UseXForwardedFor {
		gs["use_x_forwarded_for"] = true
	}
	if ipAllowlist.MaxRequestSizeMB != nil {
		gs["max_request_size_mb"] = *ipAllowlist.MaxRequestSizeMB
	}
	if ipAllowlist.MaxResponseSizeMB != nil {
		gs["max_response_size_mb"] = *ipAllowlist.MaxResponseSizeMB
	}
}

// buildCachingConfig writes cache settings into litellm_settings.
func buildCachingConfig(instance *litellmv1alpha1.LiteLLMInstance, config map[string]interface{}) {
	ls, ok := config["litellm_settings"].(map[string]interface{})
	if !ok {
		ls = map[string]interface{}{}
		config["litellm_settings"] = ls
	}

	ls["cache"] = true

	caching := instance.Spec.Caching
	params := map[string]interface{}{}

	cacheType := caching.Type
	if cacheType == "" {
		cacheType = "redis"
	}
	params["type"] = cacheType

	// Backend-specific config
	switch cacheType {
	case "redis", "redis-semantic":
		buildRedisCacheParams(instance, params)
	case "s3":
		if caching.S3 != nil {
			params["s3_bucket_name"] = caching.S3.BucketName
			if caching.S3.Region != "" {
				params["s3_region_name"] = caching.S3.Region
			}
			if caching.S3.CredentialsSecretRef != nil {
				params["s3_aws_access_key_id"] = "os.environ/CACHE_S3_ACCESS_KEY_ID"
				params["s3_aws_secret_access_key"] = "os.environ/CACHE_S3_SECRET_ACCESS_KEY"
			}
		}
	case "gcs":
		if caching.GCS != nil {
			params["s3_bucket_name"] = caching.GCS.BucketName // LiteLLM reuses s3_bucket_name for GCS
			if caching.GCS.CredentialsSecretRef != nil {
				params["s3_aws_access_key_id"] = "os.environ/CACHE_GCS_SERVICE_ACCOUNT_JSON"
			}
		}
	case "qdrant":
		if caching.Qdrant != nil {
			params["qdrant_semantic_cache_embedding_model"] = "text-embedding-ada-002"
			params["qdrant_url"] = caching.Qdrant.URL
			if caching.Qdrant.APIKeySecretRef != nil {
				params["qdrant_api_key"] = "os.environ/CACHE_QDRANT_API_KEY"
			}
			if caching.Qdrant.CollectionName != "" {
				params["qdrant_collection_name"] = caching.Qdrant.CollectionName
			}
		}
	}

	if caching.TTL != nil {
		params["ttl"] = *caching.TTL
	}
	if caching.Namespace != "" {
		params["namespace"] = caching.Namespace
	}
	if len(caching.SupportedCallTypes) > 0 {
		params["supported_call_types"] = caching.SupportedCallTypes
	}
	if caching.Mode == "default_off" {
		params["mode"] = "default_off"
	}

	ls["cache_params"] = params
}

// buildRedisCacheParams fills Redis-specific cache_params.
// If caching.redis is empty, falls back to the instance's Redis config.
func buildRedisCacheParams(instance *litellmv1alpha1.LiteLLMInstance, params map[string]interface{}) {
	caching := instance.Spec.Caching

	if caching.Redis != nil {
		// Explicit cache Redis config
		if caching.Redis.Host != "" {
			params["host"] = caching.Redis.Host
		}
		if caching.Redis.Port != nil {
			params["port"] = *caching.Redis.Port
		}
		if caching.Redis.PasswordSecretRef != nil {
			params["password"] = "os.environ/CACHE_REDIS_PASSWORD"
		}
		if caching.Redis.SSL {
			params["ssl"] = true
		}
		return
	}

	// Fall back to instance Redis config
	if instance.Spec.Redis != nil && instance.Spec.Redis.Enabled {
		redis := instance.Spec.Redis
		if redis.Host != "" {
			params["host"] = redis.Host
			port := redis.Port
			if port == 0 {
				port = 6379
			}
			params["port"] = port
			if redis.PasswordSecretRef != nil {
				params["password"] = "os.environ/REDIS_PASSWORD"
			}
		}
		// If using connectionSecretRef, Redis URL is set via env var;
		// LiteLLM picks it up automatically — no need for host/port in cache_params.
	}
}

// buildFallbackConfig writes fallback settings into litellm_settings and router_settings.
func buildFallbackConfig(fb *litellmv1alpha1.FallbackSpec, config map[string]interface{}) {
	// litellm_settings: default_fallbacks, content_policy_fallbacks, context_window_fallbacks
	if len(fb.DefaultFallbacks) > 0 || len(fb.ContentPolicyFallbacks) > 0 || len(fb.ContextWindowFallbacks) > 0 {
		ls, ok := config["litellm_settings"].(map[string]interface{})
		if !ok {
			ls = map[string]interface{}{}
			config["litellm_settings"] = ls
		}
		if len(fb.DefaultFallbacks) > 0 {
			ls["default_fallbacks"] = fb.DefaultFallbacks
		}
		if len(fb.ContentPolicyFallbacks) > 0 {
			ls["content_policy_fallbacks"] = modelFallbackEntriesToMaps(fb.ContentPolicyFallbacks)
		}
		if len(fb.ContextWindowFallbacks) > 0 {
			ls["context_window_fallbacks"] = modelFallbackEntriesToMaps(fb.ContextWindowFallbacks)
		}
	}

	// router_settings: fallbacks, max_fallbacks
	if len(fb.ModelFallbacks) > 0 || fb.MaxFallbacks != nil {
		rs, ok := config["router_settings"].(map[string]interface{})
		if !ok {
			rs = map[string]interface{}{}
			config["router_settings"] = rs
		}
		if len(fb.ModelFallbacks) > 0 {
			rs["fallbacks"] = modelFallbackEntriesToMaps(fb.ModelFallbacks)
		}
		if fb.MaxFallbacks != nil {
			rs["max_fallbacks"] = *fb.MaxFallbacks
		}
	}
}

// modelFallbackEntriesToMaps converts ModelFallbackEntry slices to the list-of-single-key-maps
// format that LiteLLM expects: [{"gpt-4": ["gpt-4-mini", "claude-3-haiku"]}].
func modelFallbackEntriesToMaps(entries []litellmv1alpha1.ModelFallbackEntry) []map[string][]string {
	result := make([]map[string][]string, len(entries))
	for i, e := range entries {
		result[i] = map[string][]string{e.Model: e.Fallbacks}
	}
	return result
}

func mapDefaultUserParams(p *litellmv1alpha1.DefaultUserParams) map[string]interface{} {
	m := map[string]interface{}{}
	if p.MaxBudget != nil {
		m["max_budget"] = *p.MaxBudget
	}
	if p.BudgetDuration != "" {
		m["budget_duration"] = p.BudgetDuration
	}
	if len(p.Models) > 0 {
		m["models"] = p.Models
	}
	if p.UserRole != "" {
		m["user_role"] = p.UserRole
	}
	return m
}

func mapDefaultTeamParams(p *litellmv1alpha1.DefaultTeamParams) map[string]interface{} {
	m := map[string]interface{}{}
	if p.MaxBudget != nil {
		m["max_budget"] = *p.MaxBudget
	}
	if p.BudgetDuration != "" {
		m["budget_duration"] = p.BudgetDuration
	}
	if len(p.Models) > 0 {
		m["models"] = p.Models
	}
	if p.TPMLimit != nil {
		m["tpm_limit"] = *p.TPMLimit
	}
	if p.RPMLimit != nil {
		m["rpm_limit"] = *p.RPMLimit
	}
	return m
}

// MarshalJSON is a helper to serialize the config as JSON for hashing.
func ConfigHash(config map[string]interface{}) string {
	data, _ := json.Marshal(config)
	return string(data)
}

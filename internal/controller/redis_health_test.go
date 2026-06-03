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

package controller

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// Regression test for issue #10: RedisReady was permanently False because the
// readiness parser expected a `redis` boolean that LiteLLM's /health/readiness
// never emits. Redis health is now probed via /cache/ping (when Redis-backed
// caching is on) or reported as configured otherwise.
func TestProbeRedisHealth(t *testing.T) {
	redisCaching := func() *litellmv1alpha1.CachingSpec {
		return &litellmv1alpha1.CachingSpec{Enabled: true, Type: "redis"}
	}

	tests := []struct {
		name          string
		redis         *litellmv1alpha1.RedisSpec
		caching       *litellmv1alpha1.CachingSpec
		cachePing     func(ctx context.Context) (*litellm.CachePingResponse, error)
		wantCondition *metav1.ConditionStatus // nil => condition must be absent
		wantReason    string
		wantConnected bool
	}{
		{
			name:          "redis disabled clears the condition",
			redis:         &litellmv1alpha1.RedisSpec{Enabled: false},
			caching:       redisCaching(),
			wantCondition: nil,
		},
		{
			name:          "redis nil clears the condition",
			redis:         nil,
			caching:       redisCaching(),
			wantCondition: nil,
		},
		{
			name:          "redis for routing only (no redis cache) reports configured",
			redis:         &litellmv1alpha1.RedisSpec{Enabled: true},
			caching:       nil,
			wantCondition: ptrStatus(metav1.ConditionTrue),
			wantReason:    "RedisConfigured",
			wantConnected: true,
		},
		{
			name:          "non-redis cache backend reports configured (no ping available)",
			redis:         &litellmv1alpha1.RedisSpec{Enabled: true},
			caching:       &litellmv1alpha1.CachingSpec{Enabled: true, Type: "s3"},
			wantCondition: ptrStatus(metav1.ConditionTrue),
			wantReason:    "RedisConfigured",
			wantConnected: true,
		},
		{
			name:    "redis-backed caching + healthy ping reports connected",
			redis:   &litellmv1alpha1.RedisSpec{Enabled: true},
			caching: redisCaching(),
			cachePing: func(_ context.Context) (*litellm.CachePingResponse, error) {
				return &litellm.CachePingResponse{Status: "healthy", PingResponse: true}, nil
			},
			wantCondition: ptrStatus(metav1.ConditionTrue),
			wantReason:    "RedisConnected",
			wantConnected: true,
		},
		{
			name:    "redis-backed caching + ping error reports disconnected",
			redis:   &litellmv1alpha1.RedisSpec{Enabled: true},
			caching: redisCaching(),
			cachePing: func(_ context.Context) (*litellm.CachePingResponse, error) {
				return nil, fmt.Errorf("connection refused")
			},
			wantCondition: ptrStatus(metav1.ConditionFalse),
			wantReason:    "RedisDisconnected",
			wantConnected: false,
		},
		{
			name:    "default cache type (empty) is treated as redis-backed",
			redis:   &litellmv1alpha1.RedisSpec{Enabled: true},
			caching: &litellmv1alpha1.CachingSpec{Enabled: true, Type: ""},
			cachePing: func(_ context.Context) (*litellm.CachePingResponse, error) {
				return &litellm.CachePingResponse{Status: "healthy", PingResponse: true}, nil
			},
			wantCondition: ptrStatus(metav1.ConditionTrue),
			wantReason:    "RedisConnected",
			wantConnected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instance := &litellmv1alpha1.LiteLLMInstance{}
			instance.Spec.Redis = tc.redis
			instance.Spec.Caching = tc.caching

			mock := litellm.NewMockClient()
			mock.MockHealth.CachePingFunc = tc.cachePing

			r := &LiteLLMInstanceReconciler{}
			r.probeRedisHealth(context.Background(), instance, mock)

			cond := meta.FindStatusCondition(instance.Status.Conditions, ConditionRedisReady)
			if tc.wantCondition == nil {
				if cond != nil {
					t.Fatalf("expected RedisReady condition to be absent, got %+v", cond)
				}
				if instance.Status.Redis != nil {
					t.Fatalf("expected status.Redis to be nil, got %+v", instance.Status.Redis)
				}
				return
			}

			if cond == nil {
				t.Fatalf("expected RedisReady condition to be set, got nil")
			}
			if cond.Status != *tc.wantCondition {
				t.Errorf("RedisReady status = %q, want %q", cond.Status, *tc.wantCondition)
			}
			if cond.Reason != tc.wantReason {
				t.Errorf("RedisReady reason = %q, want %q", cond.Reason, tc.wantReason)
			}
			if instance.Status.Redis == nil {
				t.Fatalf("expected status.Redis to be set")
			}
			if instance.Status.Redis.Connected != tc.wantConnected {
				t.Errorf("status.Redis.Connected = %v, want %v", instance.Status.Redis.Connected, tc.wantConnected)
			}
		})
	}
}

// stale RedisReady conditions are removed when Redis is later disabled.
func TestProbeRedisHealthClearsStaleCondition(t *testing.T) {
	instance := &litellmv1alpha1.LiteLLMInstance{}
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:   ConditionRedisReady,
		Status: metav1.ConditionTrue,
		Reason: "RedisConnected",
	})
	instance.Status.Redis = &litellmv1alpha1.RedisStatus{Connected: true}

	// Redis no longer configured.
	instance.Spec.Redis = nil

	r := &LiteLLMInstanceReconciler{}
	r.probeRedisHealth(context.Background(), instance, litellm.NewMockClient())

	if cond := meta.FindStatusCondition(instance.Status.Conditions, ConditionRedisReady); cond != nil {
		t.Fatalf("expected stale RedisReady condition to be removed, got %+v", cond)
	}
	if instance.Status.Redis != nil {
		t.Fatalf("expected status.Redis to be cleared, got %+v", instance.Status.Redis)
	}
}

func ptrStatus(s metav1.ConditionStatus) *metav1.ConditionStatus { return &s }

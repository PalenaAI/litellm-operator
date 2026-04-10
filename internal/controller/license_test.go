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
	"context"
	"fmt"
	"testing"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	litellmapi "github.com/PalenaAI/litellm-operator/internal/litellm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileLicense_PerInstanceSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-gateway-license",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"license-key": []byte("ent-key-123"),
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "default"},
	}

	name := r.reconcileLicense(context.Background(), instance)

	if name != "my-gateway-license" {
		t.Errorf("expected 'my-gateway-license', got %q", name)
	}
	if instance.Status.License == nil || !instance.Status.License.Active {
		t.Error("expected license status to be active")
	}
	if instance.Status.License.SecretName != "my-gateway-license" {
		t.Errorf("expected secretName 'my-gateway-license', got %q", instance.Status.License.SecretName)
	}
}

func TestReconcileLicense_NamespaceFallback(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "litellm-license",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"license-key": []byte("ent-key-456"),
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "default"},
	}

	name := r.reconcileLicense(context.Background(), instance)

	if name != "litellm-license" {
		t.Errorf("expected 'litellm-license', got %q", name)
	}
	if instance.Status.License == nil || !instance.Status.License.Active {
		t.Error("expected license status to be active")
	}
}

func TestReconcileLicense_PerInstanceTakesPrecedence(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	perInstance := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway-license", Namespace: "default"},
		Data:       map[string][]byte{"license-key": []byte("per-instance")},
	}
	namespaceWide := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm-license", Namespace: "default"},
		Data:       map[string][]byte{"license-key": []byte("namespace")},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(perInstance, namespaceWide).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "default"},
	}

	name := r.reconcileLicense(context.Background(), instance)

	if name != "my-gateway-license" {
		t.Errorf("expected per-instance secret 'my-gateway-license', got %q", name)
	}
}

func TestReconcileLicense_NoSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "default"},
	}

	name := r.reconcileLicense(context.Background(), instance)

	if name != "" {
		t.Errorf("expected empty string, got %q", name)
	}
	if instance.Status.License == nil || instance.Status.License.Active {
		t.Error("expected license status to be inactive")
	}
}

func TestReconcileLicense_SecretMissingKey(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway-license", Namespace: "default"},
		Data:       map[string][]byte{"wrong-key": []byte("value")},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "default"},
	}

	name := r.reconcileLicense(context.Background(), instance)

	if name != "" {
		t.Errorf("expected empty string when key is missing, got %q", name)
	}
	if instance.Status.License == nil || instance.Status.License.Active {
		t.Error("expected license status to be inactive when key is missing")
	}
}

func TestFindInstanceForLicenseSecret_NamespaceWide(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	inst1 := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-1", Namespace: "ns1"},
	}
	inst2 := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-2", Namespace: "ns1"},
	}
	inst3 := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-3", Namespace: "ns2"},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inst1, inst2, inst3).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm-license", Namespace: "ns1"},
	}

	requests := r.findInstanceForLicenseSecret(context.Background(), secret)

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests for ns1 instances, got %d", len(requests))
	}
	names := map[string]bool{}
	for _, req := range requests {
		names[req.NamespacedName.Name] = true
	}
	if !names["inst-1"] || !names["inst-2"] {
		t.Errorf("expected inst-1 and inst-2, got %v", names)
	}
}

func TestFindInstanceForLicenseSecret_PerInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gateway-license", Namespace: "default"},
	}

	requests := r.findInstanceForLicenseSecret(context.Background(), secret)

	expected := []reconcile.Request{{
		NamespacedName: types.NamespacedName{Name: "my-gateway", Namespace: "default"},
	}}
	if len(requests) != len(expected) {
		t.Fatalf("expected %d request, got %d", len(expected), len(requests))
	}
	if requests[0] != expected[0] {
		t.Errorf("expected %v, got %v", expected[0], requests[0])
	}
}

func TestFindInstanceForLicenseSecret_UnrelatedSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "some-other-secret", Namespace: "default"},
	}

	requests := r.findInstanceForLicenseSecret(context.Background(), secret)

	if len(requests) != 0 {
		t.Errorf("expected no requests for unrelated secret, got %d", len(requests))
	}
}

func TestIsEnterpriseLicenseError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "enterprise 403",
			err:      &litellmapi.APIError{StatusCode: 403, Message: "This feature requires an Enterprise license"},
			expected: true,
		},
		{
			name:     "enterprise 403 lowercase",
			err:      &litellmapi.APIError{StatusCode: 403, Message: "enterprise feature not available"},
			expected: true,
		},
		{
			name:     "wrapped enterprise error",
			err:      fmt.Errorf("create team: %w", &litellmapi.APIError{StatusCode: 403, Message: "Enterprise only"}),
			expected: true,
		},
		{
			name:     "403 but not enterprise",
			err:      &litellmapi.APIError{StatusCode: 403, Message: "forbidden"},
			expected: false,
		},
		{
			name:     "500 with enterprise text",
			err:      &litellmapi.APIError{StatusCode: 500, Message: "enterprise error"},
			expected: false,
		},
		{
			name:     "non-API error",
			err:      fmt.Errorf("network timeout"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				if isEnterpriseLicenseError(nil) {
					t.Error("expected false for nil error")
				}
				return
			}
			got := isEnterpriseLicenseError(tt.err)
			if got != tt.expected {
				t.Errorf("isEnterpriseLicenseError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

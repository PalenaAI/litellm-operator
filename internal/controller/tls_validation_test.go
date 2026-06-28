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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

func tlsTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = litellmv1alpha1.AddToScheme(scheme)
	return scheme
}

// drainEvents collects all events currently buffered in the fake recorder.
func drainEvents(rec *record.FakeRecorder) []string {
	var out []string
	for {
		select {
		case e := <-rec.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}

func containsEvent(events []string, substr string) bool {
	for _, e := range events {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

func TestValidateTLSSecrets_AllPresent(t *testing.T) {
	scheme := tlsTestScheme(t)
	serverTLS := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-tls", Namespace: "default"},
		Data:       map[string][]byte{"tls.crt": []byte("c"), "tls.key": []byte("k")},
	}
	ca := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca-bundle", Namespace: "default"},
		Data:       map[string][]byte{"ca.crt": []byte("ca")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(serverTLS, ca).Build()
	rec := record.NewFakeRecorder(10)
	r := &LiteLLMInstanceReconciler{Client: cl, Scheme: scheme, Recorder: rec}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "default"},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			TLS: &litellmv1alpha1.TLSSpec{
				ServerCertSecretRef: &litellmv1alpha1.SecretRef{Name: "server-tls"},
				TrustedCASecretRef:  &litellmv1alpha1.CASecretRef{Name: "ca-bundle"},
			},
		},
	}

	r.validateTLSSecrets(context.Background(), instance)

	if events := drainEvents(rec); len(events) != 0 {
		t.Errorf("expected no warning events, got %v", events)
	}
}

func TestValidateTLSSecrets_ServerCertMissingKey(t *testing.T) {
	scheme := tlsTestScheme(t)
	// Secret exists but lacks tls.key.
	serverTLS := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-tls", Namespace: "default"},
		Data:       map[string][]byte{"tls.crt": []byte("c")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(serverTLS).Build()
	rec := record.NewFakeRecorder(10)
	r := &LiteLLMInstanceReconciler{Client: cl, Scheme: scheme, Recorder: rec}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "default"},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			TLS: &litellmv1alpha1.TLSSpec{
				ServerCertSecretRef: &litellmv1alpha1.SecretRef{Name: "server-tls"},
			},
		},
	}

	r.validateTLSSecrets(context.Background(), instance)

	events := drainEvents(rec)
	if !containsEvent(events, EventReasonSecretKeyMissing) || !containsEvent(events, "tls.key") {
		t.Errorf("expected SecretKeyMissing event for tls.key, got %v", events)
	}
}

func TestValidateTLSSecrets_DBCASecretNotFound(t *testing.T) {
	scheme := tlsTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	rec := record.NewFakeRecorder(10)
	r := &LiteLLMInstanceReconciler{Client: cl, Scheme: scheme, Recorder: rec}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "default"},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			Database: litellmv1alpha1.DatabaseSpec{
				TLS: &litellmv1alpha1.DatabaseTLSSpec{
					CASecretRef: &litellmv1alpha1.CASecretRef{Name: "pg-ca"},
				},
			},
		},
	}

	r.validateTLSSecrets(context.Background(), instance)

	events := drainEvents(rec)
	if !containsEvent(events, EventReasonSecretNotFound) || !containsEvent(events, "spec.database.tls.caSecretRef") {
		t.Errorf("expected SecretNotFound event for db CA, got %v", events)
	}
}

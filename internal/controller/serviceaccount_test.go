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
	"testing"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// preservedMetadataValue marks external ServiceAccount metadata that the
// reconciler must leave untouched.
const preservedMetadataValue = "preserve"

func TestReconcileServiceAccountMetadata(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := litellmv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "litellm",
			Namespace: "gateway",
			UID:       types.UID("instance-uid"),
		},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			ServiceAccount: &litellmv1alpha1.ServiceAccountSpec{
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/initial",
				},
				Labels: map[string]string{
					"example.com/identity": "irsa",
				},
			},
		},
	}
	existing := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"external.example.com/label": preservedMetadataValue,
			},
			Annotations: map[string]string{
				"external.example.com/annotation": preservedMetadataValue,
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	reconciler := &LiteLLMInstanceReconciler{Client: client, Scheme: scheme}
	labels := map[string]string{"app.kubernetes.io/name": "litellm"}

	if err := reconciler.reconcileServiceAccount(context.Background(), instance, labels); err != nil {
		t.Fatalf("reconcileServiceAccount failed: %v", err)
	}

	key := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	var got corev1.ServiceAccount
	if err := client.Get(context.Background(), key, &got); err != nil {
		t.Fatal(err)
	}
	assertServiceAccountMetadata(t, &got, "arn:aws:iam::123456789012:role/initial")

	instance.Spec.ServiceAccount.Annotations["eks.amazonaws.com/role-arn"] = "arn:aws:iam::123456789012:role/updated"
	if err := reconciler.reconcileServiceAccount(context.Background(), instance, labels); err != nil {
		t.Fatalf("second reconcileServiceAccount failed: %v", err)
	}
	if err := client.Get(context.Background(), key, &got); err != nil {
		t.Fatal(err)
	}
	assertServiceAccountMetadata(t, &got, "arn:aws:iam::123456789012:role/updated")
}

func assertServiceAccountMetadata(t *testing.T, serviceAccount *corev1.ServiceAccount, roleARN string) {
	t.Helper()
	if got := serviceAccount.Annotations["eks.amazonaws.com/role-arn"]; got != roleARN {
		t.Errorf("role annotation = %q, want %q", got, roleARN)
	}
	if got := serviceAccount.Annotations["external.example.com/annotation"]; got != preservedMetadataValue {
		t.Errorf("external annotation was not preserved: %q", got)
	}
	if got := serviceAccount.Labels["example.com/identity"]; got != "irsa" {
		t.Errorf("configured label = %q, want irsa", got)
	}
	if got := serviceAccount.Labels["external.example.com/label"]; got != preservedMetadataValue {
		t.Errorf("external label was not preserved: %q", got)
	}
}

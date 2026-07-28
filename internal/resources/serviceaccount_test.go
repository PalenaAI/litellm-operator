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
	"reflect"
	"testing"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildServiceAccountMetadata(t *testing.T) {
	baseLabels := map[string]string{
		"app.kubernetes.io/name": "litellm",
	}
	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "litellm",
			Namespace: "gateway",
		},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			ServiceAccount: &litellmv1alpha1.ServiceAccountSpec{
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/litellm",
				},
				Labels: map[string]string{
					"example.com/identity": "irsa",
				},
			},
		},
	}

	serviceAccount := BuildServiceAccount(instance, baseLabels)

	wantLabels := map[string]string{
		"app.kubernetes.io/name": "litellm",
		"example.com/identity":   "irsa",
	}
	if !reflect.DeepEqual(serviceAccount.Labels, wantLabels) {
		t.Fatalf("unexpected labels: got %v, want %v", serviceAccount.Labels, wantLabels)
	}
	if got := serviceAccount.Annotations["eks.amazonaws.com/role-arn"]; got != "arn:aws:iam::123456789012:role/litellm" {
		t.Fatalf("unexpected IRSA role annotation: %q", got)
	}

	serviceAccount.Labels["app.kubernetes.io/name"] = "changed"
	if baseLabels["app.kubernetes.io/name"] != "litellm" {
		t.Fatal("BuildServiceAccount mutated the shared labels map")
	}
}

func TestBuildServiceAccountWithoutMetadata(t *testing.T) {
	instance := &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "gateway"},
	}

	serviceAccount := BuildServiceAccount(instance, nil)
	if serviceAccount.Labels != nil || serviceAccount.Annotations != nil {
		t.Fatalf("expected empty metadata, got labels=%v annotations=%v", serviceAccount.Labels, serviceAccount.Annotations)
	}
}

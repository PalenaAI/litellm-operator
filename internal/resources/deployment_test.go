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

func newTestInstance() *litellmv1alpha1.LiteLLMInstance {
	return &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instance",
			Namespace: "default",
		},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			MasterKey: litellmv1alpha1.MasterKeySpec{
				SecretRef: &litellmv1alpha1.SecretKeyRef{
					Name: "master-key-secret",
					Key:  "key",
				},
			},
			Database: litellmv1alpha1.DatabaseSpec{
				External: &litellmv1alpha1.ExternalDBSpec{
					ConnectionSecretRef: litellmv1alpha1.SecretKeyRef{
						Name: "db-secret",
						Key:  "url",
					},
				},
			},
		},
	}
}

func TestBuildDeployment_WithLicenseSecret(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "my-gateway-license")

	found := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "LITELLM_LICENSE" {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("LITELLM_LICENSE env var should use secretKeyRef")
			}
			if env.ValueFrom.SecretKeyRef.Name != "my-gateway-license" {
				t.Errorf("expected secret name 'my-gateway-license', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != "license-key" {
				t.Errorf("expected secret key 'license-key', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
			break
		}
	}
	if !found {
		t.Error("LITELLM_LICENSE env var not found in deployment")
	}
}

func TestBuildDeployment_WithoutLicenseSecret(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "LITELLM_LICENSE" {
			t.Error("LITELLM_LICENSE env var should not be present when no license secret is provided")
		}
	}
}

func TestBuildDeployment_LicenseSecretChangesTemplate(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	depWithout := BuildDeployment(instance, labels, "")
	depWith := BuildDeployment(instance, labels, "my-license")

	envCountWithout := len(depWithout.Spec.Template.Spec.Containers[0].Env)
	envCountWith := len(depWith.Spec.Template.Spec.Containers[0].Env)

	if envCountWith != envCountWithout+1 {
		t.Errorf("expected env var count to differ by 1, got without=%d with=%d", envCountWithout, envCountWith)
	}
}

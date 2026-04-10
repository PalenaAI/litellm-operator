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

func TestBuildDeployment_CachingDisabled(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CACHE_REDIS_PASSWORD" || env.Name == "CACHE_S3_ACCESS_KEY_ID" || env.Name == "CACHE_QDRANT_API_KEY" {
			t.Errorf("cache env var %s should not be present when caching is disabled", env.Name)
		}
	}
}

func TestBuildDeployment_CachingRedisPassword(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    "redis",
		Redis: &litellmv1alpha1.CacheRedisSpec{
			Host: "redis.example.com",
			PasswordSecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "cache-redis-secret",
				Key:  "password",
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	found := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CACHE_REDIS_PASSWORD" {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("CACHE_REDIS_PASSWORD should use secretKeyRef")
			}
			if env.ValueFrom.SecretKeyRef.Name != "cache-redis-secret" {
				t.Errorf("expected secret name 'cache-redis-secret', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != "password" {
				t.Errorf("expected secret key 'password', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
			break
		}
	}
	if !found {
		t.Error("CACHE_REDIS_PASSWORD env var not found")
	}
}

func TestBuildDeployment_CachingS3Credentials(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    "s3",
		S3: &litellmv1alpha1.CacheS3Spec{
			BucketName: "my-bucket",
			CredentialsSecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "aws-creds",
				Key:  "credentials",
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	envMap := map[string]string{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			envMap[env.Name] = env.ValueFrom.SecretKeyRef.Name
		}
	}
	if envMap["CACHE_S3_ACCESS_KEY_ID"] != "aws-creds" {
		t.Error("CACHE_S3_ACCESS_KEY_ID not found or wrong secret")
	}
	if envMap["CACHE_S3_SECRET_ACCESS_KEY"] != "aws-creds" {
		t.Error("CACHE_S3_SECRET_ACCESS_KEY not found or wrong secret")
	}
}

func TestBuildDeployment_PassThroughSecretEnvVars(t *testing.T) {
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
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	found := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "PASSTHROUGH_BRIA_AUTHORIZATION" {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("expected secretKeyRef")
			}
			if env.ValueFrom.SecretKeyRef.Name != "bria-secret" {
				t.Errorf("expected secret name 'bria-secret', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != "api-key" {
				t.Errorf("expected secret key 'api-key', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
			break
		}
	}
	if !found {
		t.Error("PASSTHROUGH_BRIA_AUTHORIZATION env var not found in deployment")
	}
}

func TestBuildDeployment_PassThroughNoSecrets(t *testing.T) {
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
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "PASSTHROUGH_BRIA_CONTENT_TYPE" {
			t.Error("should not inject env vars for static headers")
		}
	}
}

func TestBuildDeployment_PassThroughMultipleEndpoints(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PassThroughEndpoints = []litellmv1alpha1.PassThroughEndpoint{
		{
			Path:   "/bria",
			Target: "https://bria.example.com",
			HeaderSecrets: []litellmv1alpha1.HeaderSecretRef{
				{
					HeaderName: "Authorization",
					SecretRef:  litellmv1alpha1.SecretKeyRef{Name: "bria-secret", Key: "key"},
				},
			},
		},
		{
			Path:   "/langfuse",
			Target: "https://langfuse.example.com",
			HeaderSecrets: []litellmv1alpha1.HeaderSecretRef{
				{
					HeaderName: "X-API-Key",
					SecretRef:  litellmv1alpha1.SecretKeyRef{Name: "langfuse-secret", Key: "api-key"},
				},
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	envMap := map[string]string{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			envMap[env.Name] = env.ValueFrom.SecretKeyRef.Name
		}
	}
	if envMap["PASSTHROUGH_BRIA_AUTHORIZATION"] != "bria-secret" {
		t.Error("PASSTHROUGH_BRIA_AUTHORIZATION not found or wrong secret")
	}
	if envMap["PASSTHROUGH_LANGFUSE_X_API_KEY"] != "langfuse-secret" {
		t.Error("PASSTHROUGH_LANGFUSE_X_API_KEY not found or wrong secret")
	}
}

func TestBuildDeployment_CachingQdrantAPIKey(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Caching = &litellmv1alpha1.CachingSpec{
		Enabled: true,
		Type:    "qdrant",
		Qdrant: &litellmv1alpha1.CacheQdrantSpec{
			URL: "http://qdrant:6333",
			APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{
				Name: "qdrant-secret",
				Key:  "api-key",
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "")

	found := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CACHE_QDRANT_API_KEY" {
			found = true
			if env.ValueFrom.SecretKeyRef.Name != "qdrant-secret" {
				t.Errorf("expected secret name 'qdrant-secret', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			break
		}
	}
	if !found {
		t.Error("CACHE_QDRANT_API_KEY env var not found")
	}
}

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

const testSecretKeyAPIKey = "api-key"

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

	dep := BuildDeployment(instance, labels, "my-gateway-license", nil, nil)

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

	dep := BuildDeployment(instance, labels, "", nil, nil)

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "LITELLM_LICENSE" {
			t.Error("LITELLM_LICENSE env var should not be present when no license secret is provided")
		}
	}
}

func TestBuildDeployment_LicenseSecretChangesTemplate(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	depWithout := BuildDeployment(instance, labels, "", nil, nil)
	depWith := BuildDeployment(instance, labels, "my-license", nil, nil)

	envCountWithout := len(depWithout.Spec.Template.Spec.Containers[0].Env)
	envCountWith := len(depWith.Spec.Template.Spec.Containers[0].Env)

	if envCountWith != envCountWithout+1 {
		t.Errorf("expected env var count to differ by 1, got without=%d with=%d", envCountWithout, envCountWith)
	}
}

func TestBuildDeployment_CachingDisabled(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

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

	dep := BuildDeployment(instance, labels, "", nil, nil)

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

	dep := BuildDeployment(instance, labels, "", nil, nil)

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
						Key:  testSecretKeyAPIKey,
					},
				},
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

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
			if env.ValueFrom.SecretKeyRef.Key != testSecretKeyAPIKey {
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

	dep := BuildDeployment(instance, labels, "", nil, nil)

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
					SecretRef:  litellmv1alpha1.SecretKeyRef{Name: "langfuse-secret", Key: testSecretKeyAPIKey},
				},
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

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
				Key:  testSecretKeyAPIKey,
			},
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

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

func TestBuildDeployment_CredentialEnvVars(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}
	credentials := []litellmv1alpha1.LiteLLMCredential{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "openai-prod", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:    litellmv1alpha1.InstanceRef{Name: "test-instance"},
				CredentialName: "openai-prod",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{
					Name: "openai-secret",
					Key:  testSecretKeyAPIKey,
				},
			},
		},
	}

	dep := BuildDeployment(instance, labels, "", credentials, nil)

	found := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CREDENTIAL_OPENAI_PROD_API_KEY" {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("credential env var should use secretKeyRef")
			}
			if env.ValueFrom.SecretKeyRef.Name != "openai-secret" {
				t.Errorf("expected secret name 'openai-secret', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != testSecretKeyAPIKey {
				t.Errorf("expected secret key 'api-key', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
			break
		}
	}
	if !found {
		t.Error("CREDENTIAL_OPENAI_PROD_API_KEY env var not found")
	}
}

func TestBuildDeployment_CredentialEnvVarsFiltersOtherInstances(t *testing.T) {
	instance := newTestInstance() // name: test-instance
	labels := map[string]string{"app": "litellm"}
	credentials := []litellmv1alpha1.LiteLLMCredential{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "other-instance"},
				CredentialName:  "other",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
			},
		},
	}

	dep := BuildDeployment(instance, labels, "", credentials, nil)

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CREDENTIAL_OTHER_API_KEY" {
			t.Error("credential bound to a different instance should not produce env vars on this deployment")
		}
	}
}

func TestBuildDeployment_CredentialEnvVarsDedup(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}
	// Two credentials with the same credentialName (e.g. duplicate definition)
	// should only produce one env var.
	credentials := []litellmv1alpha1.LiteLLMCredential{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "test-instance"},
				CredentialName:  "shared",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "secret-a", Key: "k"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: "test-instance"},
				CredentialName:  "shared",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "secret-b", Key: "k"},
			},
		},
	}

	dep := BuildDeployment(instance, labels, "", credentials, nil)

	count := 0
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CREDENTIAL_SHARED_API_KEY" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 CREDENTIAL_SHARED_API_KEY env var, got %d", count)
	}
}

func TestBuildDeployment_GuardrailEnvVarsFromSecretRef(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}
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
					Key:  testSecretKeyAPIKey,
				},
			},
		},
	}

	dep := BuildDeployment(instance, labels, "", nil, guardrails)

	found := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "GUARDRAIL_PII_DETECTOR_API_KEY" {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("guardrail env var should use secretKeyRef")
			}
			if env.ValueFrom.SecretKeyRef.Name != "aporia-secret" {
				t.Errorf("expected secret name 'aporia-secret', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != testSecretKeyAPIKey {
				t.Errorf("expected secret key 'api-key', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
			break
		}
	}
	if !found {
		t.Error("GUARDRAIL_PII_DETECTOR_API_KEY env var not found")
	}
}

func TestBuildDeployment_GuardrailEnvVarsFiltersOtherInstances(t *testing.T) {
	instance := newTestInstance() // name: test-instance
	labels := map[string]string{"app": "litellm"}
	guardrails := []litellmv1alpha1.LiteLLMGuardrail{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: "other-instance"},
				GuardrailName: "other",
				Provider:      "aporia",
				Mode:          "pre_call",
				APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{
					Name: "s", Key: "k",
				},
			},
		},
	}

	dep := BuildDeployment(instance, labels, "", nil, guardrails)

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "GUARDRAIL_OTHER_API_KEY" {
			t.Error("guardrail bound to a different instance should not produce env vars on this deployment")
		}
	}
}

func TestBuildDeployment_GuardrailEnvVarsNoAPIKeyOK(t *testing.T) {
	// Guardrails that don't declare an APIKeySecretRef (e.g. local presidio)
	// should not produce any env var and must not crash.
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}
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

	dep := BuildDeployment(instance, labels, "", nil, guardrails)

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "GUARDRAIL_PRESIDIO_API_KEY" {
			t.Error("guardrail without APIKeySecretRef should not produce an env var")
		}
	}
}

func TestBuildDeployment_SecretManagerNone(t *testing.T) {
	instance := newTestInstance()
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "AWS_REGION_NAME" || env.Name == "HCP_VAULT_ADDR" || env.Name == "AZURE_KEY_VAULT_URI" {
			t.Errorf("unexpected secret manager env var %q when secretManager is nil", env.Name)
		}
	}
}

func TestBuildDeployment_SecretManagerAWSEnvVars(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "aws_secret_manager",
		CredentialsSecretRef: &litellmv1alpha1.SecretRef{
			Name: "aws-creds",
		},
		AWS: &litellmv1alpha1.AWSSecretManagerConfig{
			Region:  "us-east-1",
			RoleARN: "arn:aws:iam::123456789012:role/litellm",
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

	// Check env vars
	envMap := map[string]string{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}
	if envMap["AWS_REGION_NAME"] != "us-east-1" {
		t.Errorf("expected AWS_REGION_NAME=us-east-1, got %q", envMap["AWS_REGION_NAME"])
	}
	if envMap["aws_role_name"] != "arn:aws:iam::123456789012:role/litellm" {
		t.Errorf("expected aws_role_name, got %q", envMap["aws_role_name"])
	}

	// Check envFrom for credentials Secret
	container := dep.Spec.Template.Spec.Containers[0]
	found := false
	for _, ef := range container.EnvFrom {
		if ef.SecretRef != nil && ef.SecretRef.Name == "aws-creds" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected envFrom entry for aws-creds Secret")
	}
}

func TestBuildDeployment_SecretManagerVaultEnvVars(t *testing.T) {
	instance := newTestInstance()
	refreshInterval := 60
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "hashicorp_vault",
		CredentialsSecretRef: &litellmv1alpha1.SecretRef{
			Name: "vault-creds",
		},
		Vault: &litellmv1alpha1.VaultConfig{
			Address:         "https://vault.example.com",
			Namespace:       "admin",
			MountName:       "kv",
			PathPrefix:      "litellm/",
			RefreshInterval: &refreshInterval,
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

	envMap := map[string]string{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	checks := map[string]string{
		"HCP_VAULT_ADDR":             "https://vault.example.com",
		"HCP_VAULT_NAMESPACE":        "admin",
		"HCP_VAULT_MOUNT_NAME":       "kv",
		"HCP_VAULT_PATH_PREFIX":      "litellm/",
		"HCP_VAULT_REFRESH_INTERVAL": "60",
	}
	for k, want := range checks {
		if envMap[k] != want {
			t.Errorf("expected %s=%q, got %q", k, want, envMap[k])
		}
	}
}

func TestBuildDeployment_SecretManagerAzureEnvVars(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "azure_key_vault",
		Azure: &litellmv1alpha1.AzureKeyVaultConfig{
			VaultURI: "https://my-vault.vault.azure.net",
			TenantID: "tenant-123",
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

	envMap := map[string]string{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["AZURE_KEY_VAULT_URI"] != "https://my-vault.vault.azure.net" {
		t.Errorf("expected AZURE_KEY_VAULT_URI, got %q", envMap["AZURE_KEY_VAULT_URI"])
	}
	if envMap["AZURE_TENANT_ID"] != "tenant-123" {
		t.Errorf("expected AZURE_TENANT_ID=tenant-123, got %q", envMap["AZURE_TENANT_ID"])
	}
}

func TestBuildDeployment_SecretManagerNoCredentialsSecret(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "google_secret_manager",
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

	// No envFrom should be added for secret manager when no credentials Secret
	container := dep.Spec.Template.Spec.Containers[0]
	for _, ef := range container.EnvFrom {
		if ef.SecretRef != nil {
			t.Errorf("unexpected envFrom secretRef %q when credentialsSecretRef is nil", ef.SecretRef.Name)
		}
	}
}

func TestBuildDeployment_SecretManagerAWSWithIRSA(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.SecretManager = &litellmv1alpha1.SecretManagerSpec{
		Provider: "aws_secret_manager",
		AWS: &litellmv1alpha1.AWSSecretManagerConfig{
			Region:               "us-west-2",
			WebIdentityTokenPath: "/var/run/secrets/eks.amazonaws.com/serviceaccount/token",
		},
	}
	labels := map[string]string{"app": "litellm"}

	dep := BuildDeployment(instance, labels, "", nil, nil)

	envMap := map[string]string{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["aws_web_identity_token"] != "/var/run/secrets/eks.amazonaws.com/serviceaccount/token" {
		t.Errorf("expected aws_web_identity_token path, got %q", envMap["aws_web_identity_token"])
	}
	// Should not have a credentials Secret envFrom
	container := dep.Spec.Template.Spec.Containers[0]
	for _, ef := range container.EnvFrom {
		if ef.SecretRef != nil {
			t.Errorf("unexpected envFrom when using IRSA (no credentialsSecretRef)")
		}
	}
}

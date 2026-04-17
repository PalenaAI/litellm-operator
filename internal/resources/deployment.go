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
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

const (
	// defaultImageRepo is the default LiteLLM image (runs as root).
	defaultImageRepo = "ghcr.io/berriai/litellm"
	// nonRootImageRepo is the official non-root LiteLLM image (runs as nobody:65534).
	nonRootImageRepo = "ghcr.io/berriai/litellm-non_root"
	// volumeNameColorTheme is the volume name for the Admin UI color theme ConfigMap.
	volumeNameColorTheme = "color-theme"
	// volumeNameCustomSSO is the volume name for the custom SSO handler ConfigMap.
	volumeNameCustomSSO = "custom-sso-handler"
	// customSSOMountDir is the directory inside the LiteLLM pod where the
	// custom SSO handler ConfigMap is mounted. The containing package name
	// is the last path segment, so LiteLLM imports handlers as
	// "custom_sso_handlers.<stem>.<function>".
	customSSOMountDir = "/app/custom_sso_handlers"
)

func podSecurityContext(nonRoot bool) *corev1.PodSecurityContext {
	if nonRoot {
		return &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(65534),
			FSGroup:      int64Ptr(65534),
		}
	}
	return &corev1.PodSecurityContext{}
}

// BuildDeployment creates the LiteLLM Deployment.
// licenseSecretName is the name of the Secret containing the enterprise license key.
// Pass empty string when no license Secret is detected.
// credentials are the LiteLLMCredential CRs bound to this instance whose
// API keys need to be injected as env vars for the proxy to resolve them
// via os.environ/… references in credential_list. Pass nil if none.
// guardrails are the LiteLLMGuardrail CRs bound to this instance whose API
// keys and extra env vars need to be injected. Pass nil if none.
func BuildDeployment(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string, licenseSecretName string, credentials []litellmv1alpha1.LiteLLMCredential, guardrails []litellmv1alpha1.LiteLLMGuardrail) *appsv1.Deployment {
	replicas := instance.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	nonRoot := instance.Spec.Security != nil && instance.Spec.Security.RunAsNonRoot != nil && *instance.Spec.Security.RunAsNonRoot

	repo := instance.Spec.Image.Repository
	if repo == "" {
		if nonRoot {
			repo = nonRootImageRepo
		} else {
			repo = defaultImageRepo
		}
	}
	tag := instance.Spec.Image.Tag
	if tag == "" {
		tag = "main-latest"
	}
	pullPolicy := instance.Spec.Image.PullPolicy
	if pullPolicy == "" {
		pullPolicy = corev1.PullIfNotPresent
	}

	envVars := buildEnvVars(instance, licenseSecretName)
	envVars = append(envVars, credentialEnvVars(instance, credentials)...)
	envVars = append(envVars, guardrailEnvVars(instance, guardrails)...)
	envVars = mergeEnvVars(envVars, instance.Spec.ExtraEnvVars)

	envFrom := append(secretManagerEnvFrom(instance), instance.Spec.ExtraEnvFrom...)

	container := corev1.Container{
		Name:            "litellm",
		Image:           fmt.Sprintf("%s:%s", repo, tag),
		ImagePullPolicy: pullPolicy,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 4000, Protocol: corev1.ProtocolTCP},
		},
		Env:          envVars,
		EnvFrom:      envFrom,
		VolumeMounts: buildVolumeMounts(instance),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health/liveliness",
					Port: intstr.FromInt(4000),
				},
			},
			InitialDelaySeconds: healthCheckInitialDelay(instance, "liveness"),
			PeriodSeconds:       15,
			TimeoutSeconds:      5,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health/readiness",
					Port: intstr.FromInt(4000),
				},
			},
			InitialDelaySeconds: healthCheckInitialDelay(instance, "readiness"),
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health/liveliness",
					Port: intstr.FromInt(4000),
				},
			},
			PeriodSeconds:    5,
			FailureThreshold: startupFailureThreshold(instance),
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
		},
	}

	if instance.Spec.Resources != nil {
		container.Resources = *instance.Spec.Resources
	} else {
		container.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		}
	}

	imagePullSecrets := make([]corev1.LocalObjectReference, 0, len(instance.Spec.Image.PullSecrets))
	for _, s := range instance.Spec.Image.PullSecrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: s.Name})
	}

	strategy := appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
	}
	if instance.Spec.Upgrade != nil && instance.Spec.Upgrade.Strategy == "recreate" {
		strategy.Type = appsv1.RecreateDeploymentStrategyType
	} else {
		ru := &appsv1.RollingUpdateDeployment{}
		if instance.Spec.Upgrade != nil {
			if instance.Spec.Upgrade.MaxUnavailable != nil {
				val := intstr.FromInt(int(*instance.Spec.Upgrade.MaxUnavailable))
				ru.MaxUnavailable = &val
			}
			if instance.Spec.Upgrade.MaxSurge != nil {
				val := intstr.FromInt(int(*instance.Spec.Upgrade.MaxSurge))
				ru.MaxSurge = &val
			}
		}
		strategy.RollingUpdate = ru
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: strategy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.Name,
					ImagePullSecrets:   imagePullSecrets,
					Containers:         []corev1.Container{container},
					Volumes:            buildVolumes(instance),
					SecurityContext:    podSecurityContext(nonRoot),
				},
			},
		},
	}

	if len(instance.Spec.TopologySpreadConstraints) > 0 {
		dep.Spec.Template.Spec.TopologySpreadConstraints = instance.Spec.TopologySpreadConstraints
	}

	return dep
}

func buildEnvVars(instance *litellmv1alpha1.LiteLLMInstance, licenseSecretName string) []corev1.EnvVar {
	vars := []corev1.EnvVar{
		{Name: "LITELLM_CONFIG_DIR", Value: "/app/config"},
		// Required for the /model/new API endpoint used by the LiteLLMModel controller
		{Name: "STORE_MODEL_IN_DB", Value: "True"},
	}

	// Master key
	if instance.Spec.MasterKey.SecretRef != nil {
		vars = append(vars, corev1.EnvVar{
			Name: "LITELLM_MASTER_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.MasterKey.SecretRef.Name},
					Key:                  instance.Spec.MasterKey.SecretRef.Key,
				},
			},
		})
	} else if instance.Spec.MasterKey.AutoGenerate {
		vars = append(vars, corev1.EnvVar{
			Name: "LITELLM_MASTER_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Name + "-master-key"},
					Key:                  "master-key",
				},
			},
		})
	}

	// Salt key
	if instance.Spec.SaltKey != nil {
		if instance.Spec.SaltKey.SecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "LITELLM_SALT_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.SaltKey.SecretRef.Name},
						Key:                  instance.Spec.SaltKey.SecretRef.Key,
					},
				},
			})
		} else if instance.Spec.SaltKey.AutoGenerate {
			vars = append(vars, corev1.EnvVar{
				Name: "LITELLM_SALT_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: instance.Name + "-salt-key"},
						Key:                  "salt-key",
					},
				},
			})
		}
	}

	// Database URL
	if instance.Spec.Database.External != nil {
		vars = append(vars, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.Database.External.ConnectionSecretRef.Name},
					Key:                  instance.Spec.Database.External.ConnectionSecretRef.Key,
				},
			},
		})
	} else if instance.Spec.Database.CloudNativePG != nil {
		vars = append(vars, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.Database.CloudNativePG.ClusterName + "-app"},
					Key:                  "uri",
				},
			},
		})
	} else if instance.Spec.Database.Managed != nil && instance.Spec.Database.Managed.Enabled {
		vars = append(vars, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Name + "-db"},
					Key:                  "database-url",
				},
			},
		})
	}

	// Redis
	if instance.Spec.Redis != nil && instance.Spec.Redis.Enabled {
		if instance.Spec.Redis.ConnectionSecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "REDIS_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.Redis.ConnectionSecretRef.Name},
						Key:                  instance.Spec.Redis.ConnectionSecretRef.Key,
					},
				},
			})
		} else if instance.Spec.Redis.Host != "" {
			port := instance.Spec.Redis.Port
			if port == 0 {
				port = 6379
			}
			redisURL := fmt.Sprintf("redis://%s:%d", instance.Spec.Redis.Host, port)
			vars = append(vars, corev1.EnvVar{Name: "REDIS_HOST", Value: instance.Spec.Redis.Host})
			vars = append(vars, corev1.EnvVar{Name: "REDIS_PORT", Value: fmt.Sprintf("%d", port)})
			vars = append(vars, corev1.EnvVar{Name: "REDIS_URL", Value: redisURL})
			if instance.Spec.Redis.PasswordSecretRef != nil {
				vars = append(vars, corev1.EnvVar{
					Name: "REDIS_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.Redis.PasswordSecretRef.Name},
							Key:                  instance.Spec.Redis.PasswordSecretRef.Key,
						},
					},
				})
			}
		}
	}

	// SSO environment variables
	vars = append(vars, ssoEnvVars(instance)...)

	// SCIM environment variables
	if instance.Spec.SCIM != nil && instance.Spec.SCIM.Enabled {
		vars = append(vars, corev1.EnvVar{Name: "SCIM_ENABLED", Value: "true"})
		tokenSecretName := instance.Spec.SCIM.GeneratedTokenSecretName
		if tokenSecretName == "" {
			tokenSecretName = "litellm-scim-token"
		}
		if instance.Spec.SCIM.TokenSecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "SCIM_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.SCIM.TokenSecretRef.Name},
						Key:                  instance.Spec.SCIM.TokenSecretRef.Key,
					},
				},
			})
		} else {
			vars = append(vars, corev1.EnvVar{
				Name: "SCIM_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: tokenSecretName},
						Key:                  "token",
					},
				},
			})
		}
	}

	// Secret manager env vars
	vars = append(vars, secretManagerEnvVars(instance)...)

	// Caching env vars
	vars = append(vars, cachingEnvVars(instance)...)

	// Pass-through endpoint header secret env vars
	vars = append(vars, passThroughEnvVars(instance)...)

	// Callbacks env vars
	if instance.Spec.Callbacks != nil {
		vars = append(vars, instance.Spec.Callbacks.EnvVars...)
	}

	// Admin UI env vars
	vars = append(vars, adminUIEnvVars(instance)...)

	// Enterprise license
	if licenseSecretName != "" {
		vars = append(vars, corev1.EnvVar{
			Name: "LITELLM_LICENSE",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: licenseSecretName,
					},
					Key: "license-key",
				},
			},
		})
	}

	return vars
}

func ssoEnvVars(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvVar {
	sso := instance.Spec.SSO
	if sso == nil || !sso.Enabled {
		return nil
	}

	vars := []corev1.EnvVar{
		envFromSecret("GENERIC_CLIENT_ID", sso.ClientID),
		envFromSecret("GENERIC_CLIENT_SECRET", sso.ClientSecret),
		{Name: "PROXY_BASE_URL", Value: proxyBaseURL(instance)},
	}

	if sso.LogoutURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "PROXY_LOGOUT_URL", Value: sso.LogoutURL})
	}

	switch sso.Provider {
	case "azure-entra":
		vars = append(vars,
			envFromSecret("MICROSOFT_CLIENT_ID", sso.ClientID),
			envFromSecret("MICROSOFT_CLIENT_SECRET", sso.ClientSecret),
		)
		if sso.TenantID != "" {
			vars = append(vars, corev1.EnvVar{Name: "MICROSOFT_TENANT", Value: sso.TenantID})
		}
	case "okta":
		if sso.AuthorizationEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_AUTHORIZATION_ENDPOINT", Value: sso.AuthorizationEndpoint})
		}
		if sso.TokenEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_TOKEN_ENDPOINT", Value: sso.TokenEndpoint})
		}
		if sso.UserinfoEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USERINFO_ENDPOINT", Value: sso.UserinfoEndpoint})
		}
	case "google":
		vars = append(vars,
			envFromSecret("GOOGLE_CLIENT_ID", sso.ClientID),
			envFromSecret("GOOGLE_CLIENT_SECRET", sso.ClientSecret),
		)
	case "generic-oidc":
		if sso.AuthorizationEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_AUTHORIZATION_ENDPOINT", Value: sso.AuthorizationEndpoint})
		}
		if sso.TokenEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_TOKEN_ENDPOINT", Value: sso.TokenEndpoint})
		}
		if sso.UserinfoEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USERINFO_ENDPOINT", Value: sso.UserinfoEndpoint})
		}
		if len(sso.Scopes) > 0 {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_SCOPE", Value: strings.Join(sso.Scopes, " ")})
		}
	}

	if m := sso.UserAttributeMappings; m != nil {
		if m.UserID != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USER_ID_ATTRIBUTE", Value: m.UserID})
		}
		if m.Email != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USER_EMAIL_ATTRIBUTE", Value: m.Email})
		}
		if m.DisplayName != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USER_DISPLAY_NAME_ATTRIBUTE", Value: m.DisplayName})
		}
		if m.FirstName != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USER_FIRST_NAME_ATTRIBUTE", Value: m.FirstName})
		}
		if m.LastName != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USER_LAST_NAME_ATTRIBUTE", Value: m.LastName})
		}
		if m.Role != "" {
			vars = append(vars, corev1.EnvVar{Name: "GENERIC_USER_ROLE_ATTRIBUTE", Value: m.Role})
		}
	}

	return vars
}

func envFromSecret(envName string, ref litellmv1alpha1.SecretKeyRef) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: ref.Name},
				Key:                  ref.Key,
			},
		},
	}
}

// mergeEnvVars returns operator envs with any entries in user overriding by
// name. Operator ordering is preserved for overridden entries; user-only
// entries are appended in their original order.
func mergeEnvVars(operator, user []corev1.EnvVar) []corev1.EnvVar {
	if len(user) == 0 {
		return operator
	}
	byName := make(map[string]corev1.EnvVar, len(user))
	for _, e := range user {
		byName[e.Name] = e
	}
	merged := make([]corev1.EnvVar, 0, len(operator)+len(user))
	seen := make(map[string]bool, len(user))
	for _, e := range operator {
		if override, ok := byName[e.Name]; ok {
			merged = append(merged, override)
			seen[e.Name] = true
			continue
		}
		merged = append(merged, e)
	}
	for _, e := range user {
		if seen[e.Name] {
			continue
		}
		merged = append(merged, e)
		seen[e.Name] = true
	}
	return merged
}

func proxyBaseURL(instance *litellmv1alpha1.LiteLLMInstance) string {
	if instance.Spec.Ingress != nil && instance.Spec.Ingress.Enabled && instance.Spec.Ingress.Host != "" {
		scheme := "http"
		if instance.Spec.Ingress.TLS != nil && instance.Spec.Ingress.TLS.Enabled {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s", scheme, instance.Spec.Ingress.Host)
	}
	port := instance.Spec.Service.Port
	if port == 0 {
		port = 4000
	}
	return fmt.Sprintf("http://%s.%s.svc:%d", instance.Name, instance.Namespace, port)
}

func buildVolumeMounts(instance *litellmv1alpha1.LiteLLMInstance) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/app/config",
			ReadOnly:  true,
		},
		{
			Name:      "tmp",
			MountPath: "/tmp",
		},
	}
	if instance.Spec.AdminUI != nil && instance.Spec.AdminUI.ColorThemeConfigMapRef != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volumeNameColorTheme,
			MountPath: "/app/enterprise/enterprise_ui/enterprise_colors.json",
			SubPath:   "enterprise_colors.json",
			ReadOnly:  true,
		})
	}
	if customSSOConfigMapName(instance) != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volumeNameCustomSSO,
			MountPath: customSSOMountDir,
			ReadOnly:  true,
		})
	}
	return mounts
}

// customSSOConfigMapName returns the ConfigMap name backing the custom SSO
// handler mount, or "" if no ConfigMap-backed handler is configured.
func customSSOConfigMapName(instance *litellmv1alpha1.LiteLLMInstance) string {
	sso := instance.Spec.SSO
	if sso == nil || !sso.Enabled || sso.CustomSSOHandler == nil || sso.CustomSSOHandler.ConfigMapRef == nil {
		return ""
	}
	return sso.CustomSSOHandler.ConfigMapRef.Name
}

func buildVolumes(instance *litellmv1alpha1.LiteLLMInstance) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Name + "-config",
					},
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	if instance.Spec.AdminUI != nil && instance.Spec.AdminUI.ColorThemeConfigMapRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameColorTheme,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.AdminUI.ColorThemeConfigMapRef.Name,
					},
				},
			},
		})
	}
	if name := customSSOConfigMapName(instance); name != "" {
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameCustomSSO,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		})
	}
	return volumes
}

func healthCheckInitialDelay(instance *litellmv1alpha1.LiteLLMInstance, probe string) int32 {
	if instance.Spec.HealthCheck != nil {
		switch probe {
		case "liveness":
			if instance.Spec.HealthCheck.LivenessInitialDelay > 0 {
				return instance.Spec.HealthCheck.LivenessInitialDelay
			}
		case "readiness":
			if instance.Spec.HealthCheck.ReadinessInitialDelay > 0 {
				return instance.Spec.HealthCheck.ReadinessInitialDelay
			}
		}
	}
	if probe == "liveness" {
		return 15
	}
	return 10
}

func startupFailureThreshold(instance *litellmv1alpha1.LiteLLMInstance) int32 {
	if instance.Spec.HealthCheck != nil && instance.Spec.HealthCheck.StartupFailureThreshold > 0 {
		return instance.Spec.HealthCheck.StartupFailureThreshold
	}
	return 30
}

// guardrailEnvVars injects API keys (and any additional EnvVars) from
// LiteLLMGuardrail CRs bound to this instance so the proxy can resolve the
// `os.environ/GUARDRAIL_{NAME}_API_KEY` references in the guardrails
// config section. Guardrails whose InstanceRef points elsewhere are skipped.
func guardrailEnvVars(instance *litellmv1alpha1.LiteLLMInstance, guardrails []litellmv1alpha1.LiteLLMGuardrail) []corev1.EnvVar {
	if len(guardrails) == 0 {
		return nil
	}
	var vars []corev1.EnvVar
	seen := make(map[string]struct{})
	for _, g := range guardrails {
		if g.Spec.InstanceRef.Name != instance.Name {
			continue
		}
		if g.Spec.APIKeySecretRef != nil {
			envName := GuardrailEnvVarName(g.Spec.GuardrailName)
			if _, dup := seen[envName]; !dup {
				seen[envName] = struct{}{}
				vars = append(vars, corev1.EnvVar{
					Name: envName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: g.Spec.APIKeySecretRef.Name},
							Key:                  g.Spec.APIKeySecretRef.Key,
						},
					},
				})
			}
		}
		for _, ev := range g.Spec.EnvVars {
			if _, dup := seen[ev.Name]; dup {
				continue
			}
			seen[ev.Name] = struct{}{}
			vars = append(vars, ev)
		}
	}
	return vars
}

// credentialEnvVars injects API keys from LiteLLMCredential CRs as env vars
// so the proxy can resolve them via the `os.environ/CREDENTIAL_…_API_KEY`
// references in credential_list. Credentials whose InstanceRef points
// elsewhere are skipped.
func credentialEnvVars(instance *litellmv1alpha1.LiteLLMInstance, credentials []litellmv1alpha1.LiteLLMCredential) []corev1.EnvVar {
	if len(credentials) == 0 {
		return nil
	}
	vars := make([]corev1.EnvVar, 0, len(credentials))
	seen := make(map[string]struct{}, len(credentials))
	for _, c := range credentials {
		if c.Spec.InstanceRef.Name != instance.Name {
			continue
		}
		envName := CredentialEnvVarName(c.Spec.CredentialName)
		if _, dup := seen[envName]; dup {
			continue
		}
		seen[envName] = struct{}{}
		vars = append(vars, corev1.EnvVar{
			Name: envName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: c.Spec.APIKeySecretRef.Name},
					Key:                  c.Spec.APIKeySecretRef.Key,
				},
			},
		})
	}
	return vars
}

func passThroughEnvVars(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvVar {
	var vars []corev1.EnvVar
	for _, ep := range instance.Spec.PassThroughEndpoints {
		for _, hs := range ep.HeaderSecrets {
			envName := PassThroughEnvVarName(ep.Path, hs.HeaderName)
			vars = append(vars, corev1.EnvVar{
				Name: envName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: hs.SecretRef.Name},
						Key:                  hs.SecretRef.Key,
					},
				},
			})
		}
	}
	return vars
}

func cachingEnvVars(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvVar {
	caching := instance.Spec.Caching
	if caching == nil || !caching.Enabled {
		return nil
	}

	var vars []corev1.EnvVar

	cacheType := caching.Type
	if cacheType == "" {
		cacheType = "redis"
	}

	switch cacheType {
	case "redis", "redis-semantic":
		if caching.Redis != nil && caching.Redis.PasswordSecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "CACHE_REDIS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: caching.Redis.PasswordSecretRef.Name},
						Key:                  caching.Redis.PasswordSecretRef.Key,
					},
				},
			})
		}
	case "s3":
		if caching.S3 != nil && caching.S3.CredentialsSecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "CACHE_S3_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: caching.S3.CredentialsSecretRef.Name},
						Key:                  "aws_access_key_id",
					},
				},
			})
			vars = append(vars, corev1.EnvVar{
				Name: "CACHE_S3_SECRET_ACCESS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: caching.S3.CredentialsSecretRef.Name},
						Key:                  "aws_secret_access_key",
					},
				},
			})
		}
	case "gcs":
		if caching.GCS != nil && caching.GCS.CredentialsSecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "CACHE_GCS_SERVICE_ACCOUNT_JSON",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: caching.GCS.CredentialsSecretRef.Name},
						Key:                  caching.GCS.CredentialsSecretRef.Key,
					},
				},
			})
		}
	case "qdrant":
		if caching.Qdrant != nil && caching.Qdrant.APIKeySecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "CACHE_QDRANT_API_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: caching.Qdrant.APIKeySecretRef.Name},
						Key:                  caching.Qdrant.APIKeySecretRef.Key,
					},
				},
			})
		}
	}

	return vars
}

// secretManagerEnvVars returns provider-specific config env vars for the
// secret manager integration. Credential secrets are injected via envFrom
// (see secretManagerEnvFrom); this function handles the non-secret config
// values like region, vault address, tenant ID, etc.
func secretManagerEnvVars(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvVar {
	sm := instance.Spec.SecretManager
	if sm == nil {
		return nil
	}

	var vars []corev1.EnvVar

	// AWS providers
	if sm.AWS != nil {
		vars = append(vars, corev1.EnvVar{Name: "AWS_REGION_NAME", Value: sm.AWS.Region})
		if sm.AWS.RoleARN != "" {
			vars = append(vars, corev1.EnvVar{Name: "aws_role_name", Value: sm.AWS.RoleARN})
		}
		if sm.AWS.SessionName != "" {
			vars = append(vars, corev1.EnvVar{Name: "aws_session_name", Value: sm.AWS.SessionName})
		}
		if sm.AWS.WebIdentityTokenPath != "" {
			vars = append(vars, corev1.EnvVar{Name: "aws_web_identity_token", Value: sm.AWS.WebIdentityTokenPath})
		}
		if sm.AWS.STSEndpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "AWS_STS_ENDPOINT", Value: sm.AWS.STSEndpoint})
		}
	}

	// Azure Key Vault
	if sm.Azure != nil {
		vars = append(vars, corev1.EnvVar{Name: "AZURE_KEY_VAULT_URI", Value: sm.Azure.VaultURI})
		vars = append(vars, corev1.EnvVar{Name: "AZURE_TENANT_ID", Value: sm.Azure.TenantID})
	}

	// HashiCorp Vault
	if sm.Vault != nil {
		vars = append(vars, corev1.EnvVar{Name: "HCP_VAULT_ADDR", Value: sm.Vault.Address})
		if sm.Vault.Namespace != "" {
			vars = append(vars, corev1.EnvVar{Name: "HCP_VAULT_NAMESPACE", Value: sm.Vault.Namespace})
		}
		if sm.Vault.MountName != "" {
			vars = append(vars, corev1.EnvVar{Name: "HCP_VAULT_MOUNT_NAME", Value: sm.Vault.MountName})
		}
		if sm.Vault.PathPrefix != "" {
			vars = append(vars, corev1.EnvVar{Name: "HCP_VAULT_PATH_PREFIX", Value: sm.Vault.PathPrefix})
		}
		if sm.Vault.RefreshInterval != nil {
			vars = append(vars, corev1.EnvVar{Name: "HCP_VAULT_REFRESH_INTERVAL", Value: fmt.Sprintf("%d", *sm.Vault.RefreshInterval)})
		}
	}

	return vars
}

// secretManagerEnvFrom returns an EnvFromSource that injects all keys from
// the secret manager credentials Secret into the pod environment.
func secretManagerEnvFrom(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvFromSource {
	sm := instance.Spec.SecretManager
	if sm == nil || sm.CredentialsSecretRef == nil {
		return nil
	}
	return []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: sm.CredentialsSecretRef.Name,
				},
			},
		},
	}
}

// adminUIEnvVars injects environment variables for Admin UI configuration.
func adminUIEnvVars(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvVar {
	ui := instance.Spec.AdminUI
	if ui == nil {
		return nil
	}
	var vars []corev1.EnvVar
	if ui.Disabled != nil && *ui.Disabled {
		vars = append(vars, corev1.EnvVar{Name: "DISABLE_ADMIN_UI", Value: "True"})
	}
	if ui.APIDocBaseURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "LITELLM_UI_API_DOC_BASE_URL", Value: ui.APIDocBaseURL})
	}
	if ui.DocsURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "DOCS_URL", Value: ui.DocsURL})
	}
	if ui.RootRedirectURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "ROOT_REDIRECT_URL", Value: ui.RootRedirectURL})
	}
	if ui.LogoURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "UI_LOGO_PATH", Value: ui.LogoURL})
	}
	if ui.EmailLogoURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "EMAIL_LOGO_URL", Value: ui.EmailLogoURL})
	}
	if ui.EmailSupportContact != "" {
		vars = append(vars, corev1.EnvVar{Name: "EMAIL_SUPPORT_CONTACT", Value: ui.EmailSupportContact})
	}
	return vars
}

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }

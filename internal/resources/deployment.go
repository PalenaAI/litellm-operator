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
func BuildDeployment(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *appsv1.Deployment {
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

	envVars := buildEnvVars(instance)
	envVars = append(envVars, instance.Spec.ExtraEnvVars...)

	container := corev1.Container{
		Name:            "litellm",
		Image:           fmt.Sprintf("%s:%s", repo, tag),
		ImagePullPolicy: pullPolicy,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 4000, Protocol: corev1.ProtocolTCP},
		},
		Env:          envVars,
		EnvFrom:      instance.Spec.ExtraEnvFrom,
		VolumeMounts: buildVolumeMounts(),
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

func buildEnvVars(instance *litellmv1alpha1.LiteLLMInstance) []corev1.EnvVar {
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

	// Callbacks env vars
	if instance.Spec.Callbacks != nil {
		vars = append(vars, instance.Spec.Callbacks.EnvVars...)
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

func buildVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
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
}

func buildVolumes(instance *litellmv1alpha1.LiteLLMInstance) []corev1.Volume {
	return []corev1.Volume{
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

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }

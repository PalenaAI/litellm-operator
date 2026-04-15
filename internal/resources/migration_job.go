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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildMigrationJob creates a Job that runs database migrations.
func BuildMigrationJob(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *batchv1.Job {
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

	// Generate a unique job name based on the image to allow re-running on upgrades
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", repo, tag)))
	jobName := fmt.Sprintf("%s-migrate-%s", instance.Name, hex.EncodeToString(hash[:4]))

	var backoffLimit int32 = 3
	var ttl int32 = 600

	var dbEnv []corev1.EnvVar
	if instance.Spec.Database.External != nil {
		dbEnv = append(dbEnv, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.Database.External.ConnectionSecretRef.Name},
					Key:                  instance.Spec.Database.External.ConnectionSecretRef.Key,
				},
			},
		})
	} else if instance.Spec.Database.CloudNativePG != nil {
		dbEnv = append(dbEnv, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Spec.Database.CloudNativePG.ClusterName + "-app"},
					Key:                  "uri",
				},
			},
		})
	} else if instance.Spec.Database.Managed != nil && instance.Spec.Database.Managed.Enabled {
		dbEnv = append(dbEnv, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Name + "-db"},
					Key:                  "database-url",
				},
			},
		})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "migrate",
							Image: fmt.Sprintf("%s:%s", repo, tag),
							Command: []string{"sh", "-c",
								"prisma db push --schema=/app/schema.prisma --accept-data-loss --skip-generate",
							},
							Env: dbEnv,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
							},
						},
					},
					SecurityContext: podSecurityContext(nonRoot),
				},
			},
		},
	}
}

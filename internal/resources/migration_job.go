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

// migrationPodSecurityContext returns the pod-level security context for the
// migration Job pod. The default `podSecurityContext(true)` helper used by
// the gateway Deployment hardcodes UID 65534 (the user baked into the
// official `litellm-non_root` gateway image) — but the dedicated
// `litellm-migrations` image runs as wolfi-base's `nonroot` user (UID 65532),
// and Prisma's pre-warmed engine cache at `/home/nonroot/.cache` is owned by
// that UID. Running it as 65534 makes prisma exit non-zero. So for the
// dbimage path we pin UID 65532 explicitly.
func migrationPodSecurityContext(useDBImage, runAsNonRoot bool) *corev1.PodSecurityContext {
	if useDBImage {
		return &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(65532),
			FSGroup:      int64Ptr(65532),
		}
	}
	return podSecurityContext(runAsNonRoot)
}

const (
	// defaultDatabaseImageRepo is LiteLLM's componentized migrations image
	// (introduced in v1.86.0, PR #27557). Its entrypoint is
	// `python3 /app/run.py`, which runs `prisma migrate deploy` via
	// ProxyExtrasDBManager.setup_database with P3005/P3009/P3018 recovery
	// and the v2 migration resolver, then exits.
	//
	// As of June 2026 the LiteLLM team only publishes tags for release
	// candidates (e.g. v1.87.0-rc.1, v1.88.0-rc.1) — there is no v1.86.x
	// or v1.87.0 stable tag yet. If your gateway tag isn't published,
	// override via spec.database.migration.databaseImage.tag or stay on
	// the gateway-image path. Note: `ghcr.io/berriai/litellm-database`
	// is an OLDER full-proxy image (not the migrations image) — do NOT
	// confuse the two; using it here would start the proxy server and
	// the Job would never complete.
	defaultDatabaseImageRepo = "ghcr.io/berriai/litellm-migrations"

	// migrateDeployCommand applies LiteLLM's versioned migration files via
	// `ProxyExtrasDBManager.setup_database` — the same recovery-aware entry
	// the new ghcr.io/berriai/litellm-database image runs from its
	// `migrations/run.py`. It locates the 100+ versioned migrations inside
	// the litellm_proxy_extras Python package (NOT at /app/migrations, which
	// only contains build sources) and applies them with P3005/P3009/P3018
	// recovery and the v2 migration resolver.
	//
	// Replaces the pre-v0.12.x `prisma db push --accept-data-loss` (which
	// ignored versioned migrations entirely and could drop columns on schema
	// divergence) AND a naive `prisma migrate deploy --schema=/app/schema.prisma`
	// (which finds the wrong directory and applies 0–3 migrations instead
	// of the full set). Required for LiteLLM v1.86+ which disabled schema
	// updates in app pods.
	// Must pass use_migrate=True (default False!) to actually run
	// `prisma migrate deploy` instead of falling back to db push, and
	// use_v2_resolver=True to skip the legacy diff-and-force recovery that
	// caused schema thrashing during rolling deploys. These are the same
	// defaults migrations/run.py uses in the dedicated database image.
	migrateDeployCommand = `python3 -c "import sys; from litellm_proxy_extras.utils import ProxyExtrasDBManager; sys.exit(0 if ProxyExtrasDBManager.setup_database(use_migrate=True, use_v2_resolver=True) else 1)"`
)

// BuildMigrationJob creates a Job that runs database migrations.
//
// Two modes, selected by spec.migration.useDatabaseImage:
//
//  1. (default) Run `prisma migrate deploy` inside the gateway image. The
//     gateway image ships prisma and the schema at /app/schema.prisma so no
//     extra image pull is required.
//
//  2. (useDatabaseImage=true) Run LiteLLM's dedicated `litellm-database`
//     image alone — its entrypoint (`python3 /app/run.py`) wraps
//     `prisma migrate deploy` with retry/recovery for P3005 (baseline),
//     P3009/P3018 (idempotent re-runs), and the v2 migration resolver that
//     avoids schema thrashing during rolling deploys. The operator only
//     injects DATABASE_URL.
func BuildMigrationJob(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *batchv1.Job {
	nonRoot := instance.Spec.Security != nil && instance.Spec.Security.RunAsNonRoot != nil && *instance.Spec.Security.RunAsNonRoot

	useDBImage := instance.Spec.Database.Migration != nil && instance.Spec.Database.Migration.UseDatabaseImage

	gatewayTag := instance.Spec.Image.Tag
	if gatewayTag == "" {
		gatewayTag = "latest"
	}

	var (
		image           string
		command         []string
		jobHashSeed     string
		runAsNonRoot    = nonRoot
		pullPolicy      = instance.Spec.Image.PullPolicy
		gatewayPullSecs = instance.Spec.Image.PullSecrets
	)

	if useDBImage {
		// Componentized path: the dedicated migrations image runs as
		// unprivileged user 65532 by default (wolfi-base nonroot), already
		// has prisma + schema baked in, and its ENTRYPOINT does the work.
		// We deliberately do NOT set Command — the image entrypoint runs
		// `python3 /app/run.py` which wraps `prisma migrate deploy` with
		// recovery logic.
		repo := defaultDatabaseImageRepo
		tag := gatewayTag
		if instance.Spec.Database.Migration != nil && instance.Spec.Database.Migration.DatabaseImage != nil {
			if instance.Spec.Database.Migration.DatabaseImage.Repository != "" {
				repo = instance.Spec.Database.Migration.DatabaseImage.Repository
			}
			if instance.Spec.Database.Migration.DatabaseImage.Tag != "" {
				tag = instance.Spec.Database.Migration.DatabaseImage.Tag
			}
			if instance.Spec.Database.Migration.DatabaseImage.PullPolicy != "" {
				pullPolicy = instance.Spec.Database.Migration.DatabaseImage.PullPolicy
			}
		}
		image = fmt.Sprintf("%s:%s", repo, tag)
		command = nil
		// The componentized image is always non-root, regardless of the
		// instance-level runAsNonRoot setting.
		runAsNonRoot = true
		jobHashSeed = "db-image|" + image
	} else {
		// Gateway-image path: `prisma migrate deploy` inside the same image
		// the proxy runs. Respects the instance's root vs non_root variant.
		repo := instance.Spec.Image.Repository
		if repo == "" {
			if nonRoot {
				repo = nonRootImageRepo
			} else {
				repo = defaultImageRepo
			}
		}
		image = fmt.Sprintf("%s:%s", repo, gatewayTag)
		command = []string{"sh", "-c", migrateDeployCommand}
		jobHashSeed = "gateway|" + image + "|" + migrateDeployCommand
	}

	// Include the command/image-selection in the hash so toggling
	// useDatabaseImage (or upgrading from the legacy db-push command on a
	// pre-v0.12 operator) triggers a fresh Job rather than colliding with
	// the previous one.
	hash := sha256.Sum256([]byte(jobHashSeed))
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

	imagePullSecrets := make([]corev1.LocalObjectReference, 0, len(gatewayPullSecs))
	for _, s := range gatewayPullSecs {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: s.Name})
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
					RestartPolicy:    corev1.RestartPolicyOnFailure,
					ImagePullSecrets: imagePullSecrets,
					Containers: []corev1.Container{
						{
							Name:            "migrate",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Command:         command,
							Env:             dbEnv,
							VolumeMounts:    dbTLSVolumeMounts(instance),
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
							},
						},
					},
					Volumes:         dbTLSVolumes(instance),
					SecurityContext: migrationPodSecurityContext(useDBImage, runAsNonRoot),
				},
			},
		},
	}
}

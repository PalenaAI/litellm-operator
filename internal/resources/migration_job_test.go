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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// Pinning a single tag across tests keeps the test matrix focused on the
// migration code path (image selection, command, security context) rather
// than tag plumbing.
const testGatewayTag = "v1.86.1"

func TestBuildMigrationJob_DefaultUsesMigrateDeployOnGatewayImage(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag

	job := BuildMigrationJob(instance, map[string]string{"app": "litellm"})
	if job == nil {
		t.Fatal("BuildMigrationJob returned nil")
	}

	c := job.Spec.Template.Spec.Containers[0]
	if !strings.HasPrefix(c.Image, "ghcr.io/berriai/litellm:") {
		t.Errorf("expected default gateway image, got %q", c.Image)
	}
	if !strings.HasSuffix(c.Image, ":"+testGatewayTag) {
		t.Errorf("expected gateway image tag to match spec.image.tag, got %q", c.Image)
	}
	if len(c.Command) != 3 || c.Command[0] != "sh" || c.Command[1] != "-c" {
		t.Fatalf("expected sh -c wrapper command, got %#v", c.Command)
	}
	// The gateway-image path must invoke ProxyExtrasDBManager.setup_database
	// (which locates the 100+ versioned migrations inside the
	// litellm_proxy_extras Python package). A naive `prisma migrate deploy
	// --schema=/app/schema.prisma` finds the wrong directory and silently
	// applies almost nothing — verified end-to-end with the migration matrix.
	if !strings.Contains(c.Command[2], "ProxyExtrasDBManager.setup_database") {
		t.Errorf("expected ProxyExtrasDBManager.setup_database invocation; got %q", c.Command[2])
	}
	// setup_database defaults to use_migrate=False (which silently runs
	// `prisma db push` and never applies versioned migrations) — verified
	// E2E. Must pass both kwargs explicitly.
	if !strings.Contains(c.Command[2], "use_migrate=True") {
		t.Errorf("setup_database must be called with use_migrate=True; got %q", c.Command[2])
	}
	if !strings.Contains(c.Command[2], "use_v2_resolver=True") {
		t.Errorf("setup_database must be called with use_v2_resolver=True; got %q", c.Command[2])
	}
	if strings.Contains(c.Command[2], "db push") {
		t.Errorf("legacy `prisma db push` must no longer be used; got %q", c.Command[2])
	}
	if strings.Contains(c.Command[2], "--accept-data-loss") {
		t.Errorf("`--accept-data-loss` must no longer be used; got %q", c.Command[2])
	}
}

func TestBuildMigrationJob_NonRootUsesNonRootImage(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag
	yes := true
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{RunAsNonRoot: &yes}

	job := BuildMigrationJob(instance, map[string]string{"app": "litellm"})
	c := job.Spec.Template.Spec.Containers[0]

	if !strings.HasPrefix(c.Image, "ghcr.io/berriai/litellm-non_root:") {
		t.Errorf("expected non_root gateway image, got %q", c.Image)
	}
	psc := job.Spec.Template.Spec.SecurityContext
	if psc == nil || psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Error("expected pod-level runAsNonRoot=true for non-root deployments")
	}
}

func TestBuildMigrationJob_UseDatabaseImageRunsDedicatedImageOnly(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag
	instance.Spec.Database.Migration = &litellmv1alpha1.MigrationSpec{
		Enabled:          true,
		UseDatabaseImage: true,
	}

	job := BuildMigrationJob(instance, map[string]string{"app": "litellm"})
	c := job.Spec.Template.Spec.Containers[0]

	if c.Image != "ghcr.io/berriai/litellm-migrations:"+testGatewayTag {
		t.Errorf("expected dedicated migrations image (NOT litellm-database, which is the old full-proxy image) with gateway tag, got %q", c.Image)
	}
	// Crucially: no Command override. The image entrypoint
	// (python3 /app/run.py) runs prisma migrate deploy with recovery logic.
	if c.Command != nil {
		t.Errorf("expected no Command override (image entrypoint owns the migration); got %#v", c.Command)
	}
	// The componentized image always runs as a non-root user — and
	// specifically UID 65532 (wolfi-base nonroot). UID 65534 (nobody, used
	// by the legacy non_root gateway image) breaks prisma because the
	// engine cache at /home/nonroot/.cache is owned by 65532. Verified
	// against `litellm-migrations:v1.87.0-rc.1`.
	psc := job.Spec.Template.Spec.SecurityContext
	if psc == nil || psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Error("expected pod-level runAsNonRoot=true for the database image")
	}
	if psc.RunAsUser == nil || *psc.RunAsUser != 65532 {
		t.Errorf("expected RunAsUser=65532 for the database image, got %v", psc.RunAsUser)
	}
	if psc.FSGroup == nil || *psc.FSGroup != 65532 {
		t.Errorf("expected FSGroup=65532 for the database image, got %v", psc.FSGroup)
	}
}

func TestBuildMigrationJob_UseDatabaseImageRespectsOverride(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag
	instance.Spec.Database.Migration = &litellmv1alpha1.MigrationSpec{
		Enabled:          true,
		UseDatabaseImage: true,
		DatabaseImage: &litellmv1alpha1.DatabaseImageSpec{
			Repository: "registry.example.com/mirror/litellm-migrations",
			Tag:        "v1.87.0",
		},
	}

	job := BuildMigrationJob(instance, map[string]string{"app": "litellm"})
	if got := job.Spec.Template.Spec.Containers[0].Image; got != "registry.example.com/mirror/litellm-migrations:v1.87.0" {
		t.Errorf("override repo+tag not honored, got %q", got)
	}
}

func TestBuildMigrationJob_UseDatabaseImageNonRootStillRunsDatabaseImage(t *testing.T) {
	// Toggling runAsNonRoot on the instance must NOT switch the migration
	// image away from litellm-migrations — that image is always non-root.
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag
	yes := true
	instance.Spec.Security = &litellmv1alpha1.SecuritySpec{RunAsNonRoot: &yes}
	instance.Spec.Database.Migration = &litellmv1alpha1.MigrationSpec{
		Enabled:          true,
		UseDatabaseImage: true,
	}

	job := BuildMigrationJob(instance, map[string]string{"app": "litellm"})
	if !strings.HasPrefix(job.Spec.Template.Spec.Containers[0].Image, "ghcr.io/berriai/litellm-migrations:") {
		t.Errorf("expected litellm-migrations image regardless of runAsNonRoot, got %q",
			job.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestBuildMigrationJob_ToggleProducesDistinctJobNames(t *testing.T) {
	// Switching between the two migration modes must produce different Job
	// names so the new Job doesn't collide with the previous one.
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag

	gatewayJob := BuildMigrationJob(instance, nil)

	instance.Spec.Database.Migration = &litellmv1alpha1.MigrationSpec{
		Enabled:          true,
		UseDatabaseImage: true,
	}
	dbImageJob := BuildMigrationJob(instance, nil)

	if gatewayJob.Name == dbImageJob.Name {
		t.Errorf("job names must differ across migration modes; both got %q", gatewayJob.Name)
	}
}

func TestBuildMigrationJob_PodScheduling(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag
	withoutScheduling := BuildMigrationJob(instance, nil)
	instance.Spec.PodScheduling = &litellmv1alpha1.PodSchedulingSpec{
		NodeSelector: map[string]string{
			"cloud.google.com/gke-nodepool": "general-arm64",
			"kubernetes.io/arch":            "arm64",
		},
		Tolerations: []corev1.Toleration{{
			Key:      "kubernetes.io/arch",
			Operator: corev1.TolerationOpEqual,
			Value:    "arm64",
			Effect:   corev1.TaintEffectNoSchedule,
		}},
	}

	job := BuildMigrationJob(instance, nil)
	podSpec := job.Spec.Template.Spec
	if !reflect.DeepEqual(podSpec.NodeSelector, instance.Spec.PodScheduling.NodeSelector) {
		t.Errorf("migration Job node selector = %#v, want %#v", podSpec.NodeSelector, instance.Spec.PodScheduling.NodeSelector)
	}
	if !reflect.DeepEqual(podSpec.Tolerations, instance.Spec.PodScheduling.Tolerations) {
		t.Errorf("migration Job tolerations = %#v, want %#v", podSpec.Tolerations, instance.Spec.PodScheduling.Tolerations)
	}
	if job.Name == withoutScheduling.Name {
		t.Errorf("scheduling change must produce a distinct Job name; both got %q", job.Name)
	}
}

func TestBuildMigrationJob_InjectsDatabaseURLFromExternalSecret(t *testing.T) {
	instance := newTestInstance() // uses External DB
	job := BuildMigrationJob(instance, nil)
	env := job.Spec.Template.Spec.Containers[0].Env

	var found bool
	for _, e := range env {
		if e.Name == "DATABASE_URL" {
			found = true
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				t.Fatal("DATABASE_URL should be sourced from a Secret")
			}
			if e.ValueFrom.SecretKeyRef.Name != "db-secret" || e.ValueFrom.SecretKeyRef.Key != "url" {
				t.Errorf("unexpected DATABASE_URL secret ref: %#v", e.ValueFrom.SecretKeyRef)
			}
		}
	}
	if !found {
		t.Error("DATABASE_URL env var not injected into migration Job")
	}
}

// An omitted podScheduling, an explicit empty podScheduling{} and empty maps all
// mean the same thing, so they must produce the same Job name. Gating the name
// seed on the pointer alone made a rendered-but-empty block — which a Helm
// template or GitOps overlay easily produces — hash differently from an absent
// one, re-running the database migration for no reason.
func TestBuildMigrationJob_EmptyPodSchedulingKeepsJobName(t *testing.T) {
	nameWith := func(scheduling *litellmv1alpha1.PodSchedulingSpec) string {
		instance := newTestInstance()
		instance.Spec.Image.Tag = testGatewayTag
		instance.Spec.PodScheduling = scheduling
		return BuildMigrationJob(instance, nil).Name
	}

	omitted := nameWith(nil)
	cases := map[string]*litellmv1alpha1.PodSchedulingSpec{
		"empty struct":     {},
		"empty maps":       {NodeSelector: map[string]string{}, Tolerations: []corev1.Toleration{}},
		"nil affinity too": {NodeSelector: nil, Tolerations: nil, Affinity: nil},
	}
	for name, scheduling := range cases {
		t.Run(name, func(t *testing.T) {
			if got := nameWith(scheduling); got != omitted {
				t.Errorf("Job name = %q, want %q (same as omitting podScheduling)", got, omitted)
			}
		})
	}
}

// Affinity is placement like any other, so it must reach the Job Pod and change
// the Job's name — the template is immutable, so a pending Job pinned to the old
// affinity would otherwise never be replaced.
func TestBuildMigrationJob_Affinity(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Image.Tag = testGatewayTag
	withoutAffinity := BuildMigrationJob(instance, nil)

	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "node.kubernetes.io/instance-type",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"m6i.large", "m6i.xlarge"},
					}},
				}},
			},
		},
	}
	instance.Spec.PodScheduling = &litellmv1alpha1.PodSchedulingSpec{Affinity: affinity}

	job := BuildMigrationJob(instance, nil)
	if !reflect.DeepEqual(job.Spec.Template.Spec.Affinity, affinity) {
		t.Errorf("migration Job affinity = %#v, want %#v", job.Spec.Template.Spec.Affinity, affinity)
	}
	if job.Name == withoutAffinity.Name {
		t.Errorf("an affinity change must produce a distinct Job name; both got %q", job.Name)
	}
}

// The Job must not alias the CR's Affinity, or mutating the built Job would
// write back into the instance spec held in the informer cache.
func TestBuildMigrationJob_AffinityIsDeepCopied(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.PodScheduling = &litellmv1alpha1.PodSchedulingSpec{
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "pool",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"spot"},
						}},
					},
				}},
			},
		},
	}

	job := BuildMigrationJob(instance, nil)
	job.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight = 99

	if got := instance.Spec.PodScheduling.Affinity.NodeAffinity.
		PreferredDuringSchedulingIgnoredDuringExecution[0].Weight; got != 1 {
		t.Errorf("mutating the Job wrote back into the instance spec: weight = %d, want 1", got)
	}
}

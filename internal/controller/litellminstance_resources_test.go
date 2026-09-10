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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/resources"
)

type updateCountingClient struct {
	client.Client
	updates int
}

func (c *updateCountingClient) Update(ctx context.Context, obj client.Object, options ...client.UpdateOption) error {
	c.updates++
	return c.Client.Update(ctx, obj, options...)
}

func instanceResourceTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := litellmv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func instanceForResourceTest() *litellmv1alpha1.LiteLLMInstance {
	return &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "default", UID: types.UID("instance-uid")},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			Image: litellmv1alpha1.ImageSpec{Repository: "ghcr.io/berriai/litellm", Tag: "v1.93.0"},
			Database: litellmv1alpha1.DatabaseSpec{External: &litellmv1alpha1.ExternalDBSpec{
				ConnectionSecretRef: litellmv1alpha1.SecretKeyRef{Name: "database", Key: "url"},
			}},
		},
	}
}

// dbSecretForResourceTest is the Secret backing DATABASE_URL on the fixture
// instance. It must exist for the pod template's Secret digest to be stable.
func dbSecretForResourceTest() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "default"},
		Data:       map[string][]byte{"url": []byte("postgres://user:pw@db:5432/litellm")},
	}
}

func TestReconcileDeploymentSkipsUnchangedSpec(t *testing.T) {
	ctx := context.Background()
	instance := instanceForResourceTest()
	labels := labelsForInstance(instance.Name)
	scheme := instanceResourceTestScheme(t)
	countingClient := &updateCountingClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(dbSecretForResourceTest()).Build(),
	}
	reconciler := &LiteLLMInstanceReconciler{Client: countingClient, Scheme: scheme}

	// Create the Deployment through the reconciler so the fixture is genuinely
	// converged (it carries the pod-template Secret digest), then assert that a
	// second pass over identical state issues no Update.
	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("reconcile deployment: %v", err)
	}
	if countingClient.updates != 0 {
		t.Fatalf("expected unchanged Deployment to skip Update, got %d updates", countingClient.updates)
	}
}

// The end-to-end behaviour this mechanism exists for: rotating a Secret the pod
// consumes must change the pod template, so the Deployment actually rolls and the
// new value reaches the running proxy.
func TestReconcileDeploymentRollsOnSecretRotation(t *testing.T) {
	ctx := context.Background()
	instance := instanceForResourceTest()
	labels := labelsForInstance(instance.Name)
	scheme := instanceResourceTestScheme(t)
	secret := dbSecretForResourceTest()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	reconciler := &LiteLLMInstanceReconciler{Client: c, Scheme: scheme}

	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	var created appsv1.Deployment
	if err := c.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &created); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	before := created.Spec.Template.Annotations[AnnotationSecretHash]
	if before == "" {
		t.Fatal("expected the pod template to carry a Secret digest")
	}

	secret.Data["url"] = []byte("postgres://user:rotated@db:5432/litellm")
	if err := c.Update(ctx, secret); err != nil {
		t.Fatalf("rotate secret: %v", err)
	}
	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("reconcile after rotation: %v", err)
	}

	var rolled appsv1.Deployment
	if err := c.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &rolled); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if after := rolled.Spec.Template.Annotations[AnnotationSecretHash]; after == before {
		t.Error("pod template digest unchanged after rotation; the Deployment would not roll")
	}
}

// A `kubectl rollout restart` must still survive alongside the digest — the
// restart annotation is external and the digest is operator-managed, and both
// belong on the template at once.
func TestReconcileDeploymentKeepsRestartAnnotationAlongsideSecretHash(t *testing.T) {
	ctx := context.Background()
	instance := instanceForResourceTest()
	labels := labelsForInstance(instance.Name)
	scheme := instanceResourceTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dbSecretForResourceTest()).Build()
	reconciler := &LiteLLMInstanceReconciler{Client: c, Scheme: scheme}

	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	var dep appsv1.Deployment
	if err := c.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &dep); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = "2026-09-10T11:34:05Z"
	if err := c.Update(ctx, &dep); err != nil {
		t.Fatalf("simulate rollout restart: %v", err)
	}

	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("reconcile after restart: %v", err)
	}

	var after appsv1.Deployment
	if err := c.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &after); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got := after.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]; got != "2026-09-10T11:34:05Z" {
		t.Errorf("rollout restart annotation was dropped, got %q", got)
	}
	if after.Spec.Template.Annotations[AnnotationSecretHash] == "" {
		t.Error("Secret digest was dropped from the pod template")
	}
}

func TestReconcileServiceSkipsUpdateAndPreservesExternalAnnotations(t *testing.T) {
	ctx := context.Background()
	instance := instanceForResourceTest()
	labels := labelsForInstance(instance.Name)
	service := resources.BuildService(instance, labels)
	service.Annotations = map[string]string{"cloud.google.com/neg-status": "managed-by-gke"}
	scheme := instanceResourceTestScheme(t)
	countingClient := &updateCountingClient{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(service).Build()}
	reconciler := &LiteLLMInstanceReconciler{Client: countingClient, Scheme: scheme}

	if err := reconciler.reconcileService(ctx, instance, labels); err != nil {
		t.Fatalf("reconcile service: %v", err)
	}
	if countingClient.updates != 0 {
		t.Fatalf("expected unchanged Service to skip Update, got %d updates", countingClient.updates)
	}

	var actual corev1.Service
	if err := countingClient.Get(ctx, client.ObjectKeyFromObject(service), &actual); err != nil {
		t.Fatalf("get service: %v", err)
	}
	if actual.Annotations["cloud.google.com/neg-status"] != "managed-by-gke" {
		t.Fatal("expected GKE-managed Service annotation to be preserved")
	}
}

func TestReconcileDeploymentAppliesAnnotationsAndPreservesExternal(t *testing.T) {
	ctx := context.Background()
	instance := instanceForResourceTest()
	labels := labelsForInstance(instance.Name)

	// Existing Deployment carries an annotation added by another controller.
	existing := resources.BuildDeployment(instance, labels, "", nil)
	existing.Annotations = map[string]string{"external.example.com/managed": "keep"}
	scheme := instanceResourceTestScheme(t)
	countingClient := &updateCountingClient{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()}
	reconciler := &LiteLLMInstanceReconciler{Client: countingClient, Scheme: scheme}

	// Now the CR declares a Deployment annotation.
	instance.Spec.Deployment = &litellmv1alpha1.DeploymentSpec{
		Annotations: map[string]string{"reloader.stakater.com/auto": "true"},
	}
	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("reconcile deployment: %v", err)
	}
	if countingClient.updates != 1 {
		t.Fatalf("expected one Update to apply the new annotation, got %d", countingClient.updates)
	}

	var got appsv1.Deployment
	if err := countingClient.Get(ctx, client.ObjectKeyFromObject(existing), &got); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.Annotations["reloader.stakater.com/auto"] != "true" {
		t.Errorf("expected declared annotation to be applied, got %#v", got.Annotations)
	}
	if got.Annotations["external.example.com/managed"] != "keep" {
		t.Error("expected external Deployment annotation to be preserved")
	}

	// A second reconcile with no changes must not update again (idempotent).
	if err := reconciler.reconcileDeployment(ctx, instance, labels, "", nil); err != nil {
		t.Fatalf("second reconcile deployment: %v", err)
	}
	if countingClient.updates != 1 {
		t.Fatalf("expected no further Update on converged Deployment, got %d total", countingClient.updates)
	}
}

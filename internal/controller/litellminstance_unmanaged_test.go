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
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

func boolPtr(b bool) *bool { return &b }

// unmanagedInstance is a LiteLLMInstance attached to a proxy the operator did
// not deploy: no image, no database, just a master key and workload.managed=false.
func unmanagedInstance(endpoint string) *litellmv1alpha1.LiteLLMInstance {
	return &litellmv1alpha1.LiteLLMInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "default", UID: types.UID("instance-uid")},
		Spec: litellmv1alpha1.LiteLLMInstanceSpec{
			Workload: &litellmv1alpha1.WorkloadSpec{Managed: boolPtr(false), Endpoint: endpoint},
			MasterKey: litellmv1alpha1.MasterKeySpec{
				SecretRef: &litellmv1alpha1.SecretKeyRef{Name: "master-key", Key: "LITELLM_MASTER_KEY"},
			},
		},
	}
}

func masterKeySecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "master-key", Namespace: "default"},
		Data:       map[string][]byte{"LITELLM_MASTER_KEY": []byte("sk-test")},
	}
}

// reconcileUnmanaged runs the instance reconciler to completion (the first pass
// only adds the finalizer) and returns the client plus the reconciled instance.
func reconcileUnmanaged(
	t *testing.T,
	instance *litellmv1alpha1.LiteLLMInstance,
	livenessErr error,
	extra ...client.Object,
) (client.Client, *litellmv1alpha1.LiteLLMInstance) {
	t.Helper()
	scheme := instanceResourceTestScheme(t)
	objs := append([]client.Object{instance, masterKeySecret()}, extra...)
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&litellmv1alpha1.LiteLLMInstance{}).
		Build()

	mock := litellm.NewMockClient()
	mock.MockHealth.CheckLivenessFunc = func(context.Context) error { return livenessErr }

	r := &LiteLLMInstanceReconciler{
		Client:               c,
		Scheme:               scheme,
		LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}}
	for i := 0; i < 2; i++ {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}

	var out litellmv1alpha1.LiteLLMInstance
	if err := c.Get(context.Background(), req.NamespacedName, &out); err != nil {
		t.Fatalf("get instance: %v", err)
	}
	return c, &out
}

// The whole point of workload.managed=false: the operator provisions nothing.
func TestUnmanagedWorkloadCreatesNoResources(t *testing.T) {
	c, _ := reconcileUnmanaged(t, unmanagedInstance(""), nil)

	key := types.NamespacedName{Name: "litellm", Namespace: "default"}
	for name, obj := range map[string]client.Object{
		"Deployment":     &appsv1.Deployment{},
		"Service":        &corev1.Service{},
		"ConfigMap":      &corev1.ConfigMap{},
		"ServiceAccount": &corev1.ServiceAccount{},
	} {
		err := c.Get(context.Background(), key, obj)
		if !apierrors.IsNotFound(err) {
			t.Errorf("%s: want NotFound, got %v", name, err)
		}
	}
}

// Attaching to a proxy someone else owns must not adopt or mutate it. The
// name collides deliberately — that is the shape reconcileDeployment used to
// overwrite by name.
func TestUnmanagedWorkloadLeavesForeignObjectsUntouched(t *testing.T) {
	foreign := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "litellm", Namespace: "default",
			Labels: map[string]string{"app.kubernetes.io/managed-by": "Helm"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "helm-litellm"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "helm-litellm"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "litellm", Image: "helm/litellm:1.2.3"}}},
			},
		},
		Status: appsv1.DeploymentStatus{Replicas: 3, ReadyReplicas: 3},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80, Name: "http"}}},
	}

	c, _ := reconcileUnmanaged(t, unmanagedInstance(""), nil, foreign, svc)

	var got appsv1.Deployment
	if err := c.Get(context.Background(), types.NamespacedName{Name: "litellm", Namespace: "default"}, &got); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.ResourceVersion != foreign.ResourceVersion {
		t.Errorf("deployment was mutated: resourceVersion %s -> %s", foreign.ResourceVersion, got.ResourceVersion)
	}
	if got.Spec.Template.Spec.Containers[0].Image != "helm/litellm:1.2.3" {
		t.Errorf("deployment image overwritten: %s", got.Spec.Template.Spec.Containers[0].Image)
	}
	if len(got.OwnerReferences) != 0 {
		t.Errorf("foreign deployment adopted: %v", got.OwnerReferences)
	}
}

// A migration Job is a write to persistent state the operator does not own, at
// a schema version taken from spec.image.tag — meaningless for a proxy the
// operator did not deploy. It must never run, even when explicitly configured,
// and saying so beats skipping it silently.
func TestUnmanagedWorkloadNeverRunsMigrations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		migration *litellmv1alpha1.MigrationSpec
		wantMsg   string
	}{
		{"no migration block", nil, "Schema is owned by the externally-managed proxy"},
		{"migration explicitly enabled", &litellmv1alpha1.MigrationSpec{Enabled: true}, "is ignored"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instance := unmanagedInstance("http://litellm.platform.svc:4000")
			instance.Spec.Database.Migration = tc.migration
			c, out := reconcileUnmanaged(t, instance, nil)

			var jobs batchv1.JobList
			if err := c.List(context.Background(), &jobs); err != nil {
				t.Fatalf("list jobs: %v", err)
			}
			if len(jobs.Items) != 0 {
				t.Errorf("migration Job created for an unmanaged workload: %v", jobs.Items)
			}

			cond := meta.FindStatusCondition(out.Status.Conditions, ConditionDatabaseReady)
			if cond == nil || cond.Reason != "WorkloadUnmanaged" {
				t.Fatalf("DatabaseReady condition = %+v, want reason WorkloadUnmanaged", cond)
			}
			if !strings.Contains(cond.Message, tc.wantMsg) {
				t.Errorf("DatabaseReady message = %q, want it to contain %q", cond.Message, tc.wantMsg)
			}
		})
	}
}

// Readiness of an unmanaged proxy comes from the admin API answering, not from
// a Deployment that may not exist (StatefulSet, other namespace, off-cluster).
func TestUnmanagedReadinessFollowsLivenessProbe(t *testing.T) {
	for _, tc := range []struct {
		name        string
		livenessErr error
		wantReady   bool
		wantReason  string
	}{
		{"proxy answers", nil, true, "ProxyReachable"},
		{"proxy down", errors.New("connection refused"), false, "ProxyNotReachable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, out := reconcileUnmanaged(t, unmanagedInstance("http://litellm.platform.svc:4000"), tc.livenessErr)

			if out.Status.Ready != tc.wantReady {
				t.Errorf("status.ready = %v, want %v", out.Status.Ready, tc.wantReady)
			}
			cond := meta.FindStatusCondition(out.Status.Conditions, ConditionReady)
			if cond == nil || cond.Reason != tc.wantReason {
				t.Errorf("Ready condition = %+v, want reason %q", cond, tc.wantReason)
			}
			// No workload of ours, so no pod-level condition to report.
			if meta.FindStatusCondition(out.Status.Conditions, ConditionPodsHealthy) != nil {
				t.Error("PodsHealthy condition set for an unmanaged workload")
			}
		})
	}
}

// status.version for an unmanaged instance must never be spec.image.tag, which
// describes nothing the operator deployed and would print a fabricated
// "latest". LiteLLM only discloses litellm_version on /health/readiness when
// its own general_settings sets allow_public_health_readiness_details; the
// endpoint takes no auth, so the master key does not unlock it. Both shapes of
// payload have to behave.
func TestUnmanagedVersionComesFromTheProxy(t *testing.T) {
	for _, tc := range []struct {
		name      string
		readiness *litellm.ReadinessResponse
		want      string
	}{
		{
			// allow_public_health_readiness_details: true
			name:      "detailed payload reports the running version",
			readiness: &litellm.ReadinessResponse{Status: "healthy", LiteLLMVersion: "1.93.0"},
			want:      "1.93.0",
		},
		{
			// The default payload: {"status": ..., "db": ...}. Empty is honest —
			// the operator does not know, and must not invent "latest".
			name:      "minimal payload leaves the version empty",
			readiness: &litellm.ReadinessResponse{Status: "healthy", DBHealth: "connected"},
			want:      "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := instanceResourceTestScheme(t)
			instance := unmanagedInstance("http://litellm.platform.svc:4000")
			c := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(instance, masterKeySecret()).
				WithStatusSubresource(&litellmv1alpha1.LiteLLMInstance{}).
				Build()

			mock := litellm.NewMockClient()
			mock.MockHealth.ReadinessFunc = func(context.Context) (*litellm.ReadinessResponse, error) {
				return tc.readiness, nil
			}
			r := &LiteLLMInstanceReconciler{
				Client:               c,
				Scheme:               scheme,
				LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
			}
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}}
			for i := 0; i < 2; i++ {
				if _, err := r.Reconcile(context.Background(), req); err != nil {
					t.Fatalf("reconcile: %v", err)
				}
			}

			var out litellmv1alpha1.LiteLLMInstance
			if err := c.Get(context.Background(), req.NamespacedName, &out); err != nil {
				t.Fatalf("get instance: %v", err)
			}
			if out.Status.Version != tc.want {
				t.Errorf("status.version = %q, want %q", out.Status.Version, tc.want)
			}
		})
	}
}

// A name-matched Deployment must not make an unreachable proxy look ready.
func TestUnmanagedReadinessIgnoresNameMatchedDeployment(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "x"}}},
		},
		Status: appsv1.DeploymentStatus{Replicas: 2, ReadyReplicas: 2},
	}
	_, out := reconcileUnmanaged(t, unmanagedInstance(""), errors.New("connection refused"), dep)

	if out.Status.Ready {
		t.Error("status.ready = true from a Deployment we do not manage")
	}
}

func TestWorkloadManaged(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *litellmv1alpha1.WorkloadSpec
		want bool
	}{
		{"absent defaults to managed", nil, true},
		{"unset defaults to managed", &litellmv1alpha1.WorkloadSpec{}, true},
		{"managed true", &litellmv1alpha1.WorkloadSpec{Managed: boolPtr(true)}, true},
		{"managed false", &litellmv1alpha1.WorkloadSpec{Managed: boolPtr(false)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := &litellmv1alpha1.LiteLLMInstance{Spec: litellmv1alpha1.LiteLLMInstanceSpec{Workload: tc.spec}}
			if got := workloadManaged(i); got != tc.want {
				t.Errorf("workloadManaged = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInstanceEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(*litellmv1alpha1.LiteLLMInstance)
		expected string
	}{
		{
			name:     "defaults to the in-cluster Service on port 4000",
			mutate:   func(*litellmv1alpha1.LiteLLMInstance) {},
			expected: "http://litellm.default.svc:4000",
		},
		{
			name: "honours spec.service.port",
			mutate: func(i *litellmv1alpha1.LiteLLMInstance) {
				i.Spec.Service = litellmv1alpha1.ServiceSpec{Port: 80}
			},
			expected: "http://litellm.default.svc:80",
		},
		{
			name: "https when the proxy serves TLS",
			mutate: func(i *litellmv1alpha1.LiteLLMInstance) {
				i.Spec.TLS = &litellmv1alpha1.TLSSpec{
					ServerCertSecretRef: &litellmv1alpha1.SecretRef{Name: "serving-cert"},
				}
			},
			expected: "https://litellm.default.svc:4000",
		},
		{
			name: "explicit workload.endpoint wins",
			mutate: func(i *litellmv1alpha1.LiteLLMInstance) {
				i.Spec.Workload = &litellmv1alpha1.WorkloadSpec{
					Managed: boolPtr(false), Endpoint: "https://litellm.platform.svc:443",
				}
			},
			expected: "https://litellm.platform.svc:443",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "default"},
			}
			tc.mutate(i)
			if got := instanceEndpoint(i); got != tc.expected {
				t.Errorf("instanceEndpoint = %q, want %q", got, tc.expected)
			}
		})
	}
}

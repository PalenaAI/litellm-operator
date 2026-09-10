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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// interceptorReturningError makes every Secret read fail, simulating a transient
// API-server error rather than a genuine NotFound.
func interceptorReturningError() interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return errors.New("connection refused")
		},
	}
}

func secretHashScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme: %v", err)
	}
	return scheme
}

func secretOf(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data:       data,
	}
}

// envRefSpec is a pod spec consuming one key of one Secret via secretKeyRef —
// exactly how LITELLM_LICENSE reaches the proxy.
func envRefSpec(secretName, key string) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "litellm",
			Env: []corev1.EnvVar{{
				Name: "LITELLM_LICENSE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  key,
					},
				},
			}},
		}},
	}
}

func hashFor(t *testing.T, spec corev1.PodSpec, objs ...client.Object) string {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(secretHashScheme(t)).WithObjects(objs...).Build()
	h, err := podTemplateSecretHash(context.Background(), c, "default", spec)
	if err != nil {
		t.Fatalf("podTemplateSecretHash: %v", err)
	}
	return h
}

// The regression this whole mechanism exists for: updating the license Secret's
// value must change the pod template hash, otherwise the Deployment is
// byte-identical and the running pod keeps the old license forever.
func TestPodTemplateSecretHash_ChangesWhenLicenseValueRotates(t *testing.T) {
	spec := envRefSpec("litellm-license", "license-key")

	before := hashFor(t, spec, secretOf("litellm-license", map[string][]byte{"license-key": []byte("old-license")}))
	after := hashFor(t, spec, secretOf("litellm-license", map[string][]byte{"license-key": []byte("new-license")}))

	if before == "" || after == "" {
		t.Fatal("expected a non-empty hash when the pod consumes a Secret")
	}
	if before == after {
		t.Error("hash did not change when the license value rotated; the Deployment would not roll")
	}
}

func TestPodTemplateSecretHash_StableForUnchangedValue(t *testing.T) {
	spec := envRefSpec("litellm-license", "license-key")
	secret := map[string][]byte{"license-key": []byte("same-license")}

	first := hashFor(t, spec, secretOf("litellm-license", secret))
	second := hashFor(t, spec, secretOf("litellm-license", secret))

	if first != second {
		t.Error("hash is not deterministic; the Deployment would roll on every reconcile")
	}
}

// A Secret that does not exist yet must hash differently from one that does, so
// the pod rolls when a license is installed after the instance is created.
func TestPodTemplateSecretHash_AbsentSecretDiffersFromPresent(t *testing.T) {
	spec := envRefSpec("litellm-license", "license-key")

	absent := hashFor(t, spec)
	present := hashFor(t, spec, secretOf("litellm-license", map[string][]byte{"license-key": []byte("ent-key")}))

	if absent == "" {
		t.Fatal("expected a hash even when the referenced Secret is absent")
	}
	if absent == present {
		t.Error("absent and present Secret hashed the same; installing a license would not roll the pod")
	}
}

// Only referenced keys count: an unrelated key changing in a shared Secret must
// not roll the workload.
func TestPodTemplateSecretHash_IgnoresUnreferencedKeys(t *testing.T) {
	spec := envRefSpec("gateway-oidc", "client-secret")

	a := hashFor(t, spec, secretOf("gateway-oidc", map[string][]byte{
		"client-secret": []byte("s3cret"),
		"unrelated":     []byte("one"),
	}))
	b := hashFor(t, spec, secretOf("gateway-oidc", map[string][]byte{
		"client-secret": []byte("s3cret"),
		"unrelated":     []byte("two"),
	}))

	if a != b {
		t.Error("an unreferenced key changed the hash; the pod would roll for no reason")
	}
}

// envFrom consumes the whole Secret, so any key changing must roll.
func TestPodTemplateSecretHash_EnvFromCoversEveryKey(t *testing.T) {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "litellm",
			EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "sm-creds"},
				},
			}},
		}},
	}

	a := hashFor(t, spec, secretOf("sm-creds", map[string][]byte{"AWS_SECRET_ACCESS_KEY": []byte("one")}))
	b := hashFor(t, spec, secretOf("sm-creds", map[string][]byte{"AWS_SECRET_ACCESS_KEY": []byte("two")}))

	if a == b {
		t.Error("envFrom Secret rotation did not change the hash")
	}
}

// A Secret volume (TLS certs, DB TLS) is resolved at mount time and must roll too.
func TestPodTemplateSecretHash_SecretVolume(t *testing.T) {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{Name: "litellm"}},
		Volumes: []corev1.Volume{{
			Name: "tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "gateway-tls"},
			},
		}},
	}

	a := hashFor(t, spec, secretOf("gateway-tls", map[string][]byte{"tls.crt": []byte("cert-v1")}))
	b := hashFor(t, spec, secretOf("gateway-tls", map[string][]byte{"tls.crt": []byte("cert-v2")}))

	if a == b {
		t.Error("Secret volume rotation did not change the hash")
	}
}

// imagePullSecrets are read by the kubelet at pull time, not projected into the
// container, so rotating one must NOT roll a running pod.
func TestPodTemplateSecretHash_IgnoresImagePullSecrets(t *testing.T) {
	spec := corev1.PodSpec{
		Containers:       []corev1.Container{{Name: "litellm"}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "ghcr-creds"}},
	}

	if h := hashFor(t, spec, secretOf("ghcr-creds", map[string][]byte{".dockerconfigjson": []byte("{}")})); h != "" {
		t.Errorf("imagePullSecrets should not be hashed, got %q", h)
	}
}

// A pod consuming no Secrets gets no annotation at all, rather than a hash of
// nothing that would need to stay stable forever.
func TestPodTemplateSecretHash_NoSecretsYieldsEmpty(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "litellm"}}}

	if h := hashFor(t, spec); h != "" {
		t.Errorf("expected empty hash when no Secrets are consumed, got %q", h)
	}
}

// Two different Secrets whose names and values could be concatenated into the
// same byte stream must not collide (guards the length-prefixed encoding).
func TestPodTemplateSecretHash_NoConcatenationCollision(t *testing.T) {
	spec := envRefSpec("s", "k")

	a := hashFor(t, spec, secretOf("s", map[string][]byte{"k": []byte("ab")}))
	b := hashFor(t, spec, secretOf("s", map[string][]byte{"k": []byte("a")}))

	if a == b {
		t.Error("values with a shared prefix collided")
	}
}

// A transient API failure must surface as an error, never as a digest that looks
// like the Secret vanished — that would roll the workload on a blip.
func TestPodTemplateSecretHash_ReadErrorIsPropagated(t *testing.T) {
	c := fake.NewClientBuilder().
		WithScheme(secretHashScheme(t)).
		WithInterceptorFuncs(interceptorReturningError()).
		Build()

	if _, err := podTemplateSecretHash(context.Background(), c, "default", envRefSpec("litellm-license", "license-key")); err == nil {
		t.Error("expected a read failure to be propagated")
	}
}

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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func configMapOf(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data:       data,
	}
}

// proxyConfigSpec is a pod spec mounting the generated proxy config ConfigMap as
// a whole volume — exactly how BuildDeployment mounts it at /app/config.
func proxyConfigSpec(configMapName string) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:         "litellm",
			VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/app/config"}},
		}},
		Volumes: []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			},
		}},
	}
}

func configHashFor(t *testing.T, spec corev1.PodSpec, objs ...client.Object) string {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(secretHashScheme(t)).WithObjects(objs...).Build()
	h, err := podTemplateConfigMapHash(context.Background(), c, "default", spec)
	if err != nil {
		t.Fatalf("podTemplateConfigMapHash: %v", err)
	}
	return h
}

// The regression this mechanism exists for: deleting a LiteLLMGuardrail removes
// its entry from proxy_server_config.yaml, but litellm parses that file only at
// startup. Without a content digest on the pod template the Deployment stays
// byte-identical, nothing rolls, and the running proxy keeps enforcing the
// guardrail indefinitely.
func TestPodTemplateConfigMapHash_ChangesWhenGuardrailIsRemoved(t *testing.T) {
	spec := proxyConfigSpec("litellm-config")

	withGuardrail := `model_list: []
guardrails:
- guardrail_name: pii-detector
  litellm_params:
    guardrail: presidio
    mode: pre_call
`
	withoutGuardrail := "model_list: []\n"

	before := configHashFor(t, spec, configMapOf("litellm-config", map[string]string{
		"proxy_server_config.yaml": withGuardrail,
	}))
	after := configHashFor(t, spec, configMapOf("litellm-config", map[string]string{
		"proxy_server_config.yaml": withoutGuardrail,
	}))

	if before == "" || after == "" {
		t.Fatal("expected a non-empty hash when the pod consumes a ConfigMap")
	}
	if before == after {
		t.Error("hash did not change when the guardrail was dropped from the config; the Deployment would not roll")
	}
}

func TestPodTemplateConfigMapHash_StableForUnchangedConfig(t *testing.T) {
	spec := proxyConfigSpec("litellm-config")
	data := map[string]string{"proxy_server_config.yaml": "model_list: []\n"}

	first := configHashFor(t, spec, configMapOf("litellm-config", data))
	second := configHashFor(t, spec, configMapOf("litellm-config", data))

	if first != second {
		t.Errorf("hash is not stable for unchanged config: %q vs %q", first, second)
	}
}

func TestPodTemplateConfigMapHash_AbsentConfigMapDiffersFromPresent(t *testing.T) {
	spec := proxyConfigSpec("litellm-config")

	absent := configHashFor(t, spec)
	present := configHashFor(t, spec, configMapOf("litellm-config", map[string]string{
		"proxy_server_config.yaml": "model_list: []\n",
	}))

	if absent == "" {
		t.Fatal("expected a hash even when the ConfigMap does not exist yet")
	}
	if absent == present {
		t.Error("absent ConfigMap hashed the same as a present one; creating it later would not roll the pod")
	}
}

// A key the pod never projects must not roll it — the Admin UI colour theme is
// mounted with an explicit item list, so an unrelated key in that ConfigMap is
// not visible to the container.
func TestPodTemplateConfigMapHash_IgnoresUnreferencedKeys(t *testing.T) {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{Name: "litellm"}},
		Volumes: []corev1.Volume{{
			Name: "ui-theme",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "ui-theme"},
					Items:                []corev1.KeyToPath{{Key: "enterprise_colors.json", Path: "enterprise_colors.json"}},
				},
			},
		}},
	}

	before := configHashFor(t, spec, configMapOf("ui-theme", map[string]string{
		"enterprise_colors.json": `{"brand":"blue"}`,
		"unrelated":              "a",
	}))
	after := configHashFor(t, spec, configMapOf("ui-theme", map[string]string{
		"enterprise_colors.json": `{"brand":"blue"}`,
		"unrelated":              "b",
	}))

	if before != after {
		t.Error("hash changed for a key the pod does not project; the pod would roll for nothing")
	}
}

func TestPodTemplateConfigMapHash_EnvFromCoversEveryKey(t *testing.T) {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "litellm",
			EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "extra-env"},
				},
			}},
		}},
	}

	before := configHashFor(t, spec, configMapOf("extra-env", map[string]string{"LOG_LEVEL": "info"}))
	after := configHashFor(t, spec, configMapOf("extra-env", map[string]string{"LOG_LEVEL": "debug"}))

	if before == after {
		t.Error("envFrom hash did not change when a key changed; every key is projected")
	}
}

func TestPodTemplateConfigMapHash_ConfigMapKeyRef(t *testing.T) {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "litellm",
			Env: []corev1.EnvVar{{
				Name: "SOME_SETTING",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "settings"},
						Key:                  "some-setting",
					},
				},
			}},
		}},
	}

	before := configHashFor(t, spec, configMapOf("settings", map[string]string{"some-setting": "a"}))
	after := configHashFor(t, spec, configMapOf("settings", map[string]string{"some-setting": "b"}))

	if before == after {
		t.Error("hash did not change when a configMapKeyRef value changed")
	}
}

func TestPodTemplateConfigMapHash_BinaryData(t *testing.T) {
	spec := proxyConfigSpec("binary-cm")

	cmWith := func(v []byte) *corev1.ConfigMap {
		cm := configMapOf("binary-cm", nil)
		cm.BinaryData = map[string][]byte{"blob": v}
		return cm
	}

	before := configHashFor(t, spec, cmWith([]byte{0x01, 0x02}))
	after := configHashFor(t, spec, cmWith([]byte{0x01, 0x03}))

	if before == after {
		t.Error("hash did not change when binaryData changed")
	}
}

func TestPodTemplateConfigMapHash_NoConfigMapsYieldsEmpty(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "litellm"}}}

	if got := configHashFor(t, spec); got != "" {
		t.Errorf("expected an empty hash when no ConfigMap is consumed, got %q", got)
	}
}

// Length-prefixing must stop two different configs from hashing alike purely
// because their concatenated bytes coincide.
func TestPodTemplateConfigMapHash_NoConcatenationCollision(t *testing.T) {
	spec := proxyConfigSpec("litellm-config")

	first := configHashFor(t, spec, configMapOf("litellm-config", map[string]string{"ab": "c"}))
	second := configHashFor(t, spec, configMapOf("litellm-config", map[string]string{"a": "bc"}))

	if first == second {
		t.Error("distinct key/value splits collided; fields are not length-prefixed")
	}
}

// A transient API error must not be mistaken for "the ConfigMap is gone", which
// would roll the workload on every blip.
func TestPodTemplateConfigMapHash_ReadErrorIsPropagated(t *testing.T) {
	c := fake.NewClientBuilder().
		WithScheme(secretHashScheme(t)).
		WithInterceptorFuncs(interceptorReturningError()).
		Build()

	_, err := podTemplateConfigMapHash(context.Background(), c, "default", proxyConfigSpec("litellm-config"))
	if err == nil {
		t.Fatal("expected a transient read error to be propagated, not folded into the digest")
	}
	if !strings.Contains(err.Error(), "litellm-config") {
		t.Errorf("error should name the ConfigMap it failed on, got %v", err)
	}
}

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
	"testing"

	corev1 "k8s.io/api/core/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// envValue returns the literal Value of the named env var, or "" if absent or
// sourced from a ValueFrom.
func envValue(envs []corev1.EnvVar, name string) (string, bool) {
	for _, e := range envs {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

func volumeByName(vols []corev1.Volume, name string) (corev1.Volume, bool) {
	for _, v := range vols {
		if v.Name == name {
			return v, true
		}
	}
	return corev1.Volume{}, false
}

func mountByName(mounts []corev1.VolumeMount, name string) (corev1.VolumeMount, bool) {
	for _, m := range mounts {
		if m.Name == name {
			return m, true
		}
	}
	return corev1.VolumeMount{}, false
}

func TestBuildDeployment_TLSServeHTTPS(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.TLS = &litellmv1alpha1.TLSSpec{
		ServerCertSecretRef: &litellmv1alpha1.SecretRef{Name: "gateway-server-tls"},
	}

	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	c := dep.Spec.Template.Spec.Containers[0]

	if got, ok := envValue(c.Env, "SSL_CERTFILE_PATH"); !ok || got != tlsServerMountDir+"/tls.crt" {
		t.Errorf("SSL_CERTFILE_PATH = %q (found=%v), want %q", got, ok, tlsServerMountDir+"/tls.crt")
	}
	if got, ok := envValue(c.Env, "SSL_KEYFILE_PATH"); !ok || got != tlsServerMountDir+"/tls.key" {
		t.Errorf("SSL_KEYFILE_PATH = %q (found=%v), want %q", got, ok, tlsServerMountDir+"/tls.key")
	}

	vol, ok := volumeByName(dep.Spec.Template.Spec.Volumes, volumeNameTLSServer)
	if !ok {
		t.Fatal("tls-server volume not found")
	}
	if vol.Secret == nil || vol.Secret.SecretName != "gateway-server-tls" {
		t.Errorf("tls-server volume should reference secret 'gateway-server-tls', got %+v", vol.Secret)
	}
	mount, ok := mountByName(c.VolumeMounts, volumeNameTLSServer)
	if !ok || mount.MountPath != tlsServerMountDir || !mount.ReadOnly {
		t.Errorf("tls-server mount wrong: %+v", mount)
	}

	// Probes must switch to HTTPS so the handshake succeeds.
	for _, p := range []*corev1.Probe{c.LivenessProbe, c.ReadinessProbe, c.StartupProbe} {
		if p.HTTPGet.Scheme != corev1.URISchemeHTTPS {
			t.Errorf("probe scheme = %q, want HTTPS when serving TLS", p.HTTPGet.Scheme)
		}
	}
}

func TestBuildDeployment_TLSProbeSchemeHTTPByDefault(t *testing.T) {
	instance := newTestInstance()
	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	c := dep.Spec.Template.Spec.Containers[0]
	for _, p := range []*corev1.Probe{c.LivenessProbe, c.ReadinessProbe, c.StartupProbe} {
		if p.HTTPGet.Scheme != corev1.URISchemeHTTP {
			t.Errorf("probe scheme = %q, want HTTP without TLS", p.HTTPGet.Scheme)
		}
	}
}

func TestBuildDeployment_TLSOutboundCADefaultKey(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.TLS = &litellmv1alpha1.TLSSpec{
		TrustedCASecretRef: &litellmv1alpha1.CASecretRef{Name: "internal-ca-bundle"},
	}

	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	c := dep.Spec.Template.Spec.Containers[0]

	if got, ok := envValue(c.Env, "SSL_CERT_FILE"); !ok || got != tlsCAMountDir+"/ca.crt" {
		t.Errorf("SSL_CERT_FILE = %q (found=%v), want %q", got, ok, tlsCAMountDir+"/ca.crt")
	}
	// No serve cert → probes stay HTTP.
	if c.ReadinessProbe.HTTPGet.Scheme != corev1.URISchemeHTTP {
		t.Error("outbound-CA-only must not flip probes to HTTPS")
	}
	if _, ok := volumeByName(dep.Spec.Template.Spec.Volumes, volumeNameTLSCA); !ok {
		t.Error("tls-ca volume not found")
	}
}

func TestBuildDeployment_TLSOutboundCACustomKey(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.TLS = &litellmv1alpha1.TLSSpec{
		TrustedCASecretRef: &litellmv1alpha1.CASecretRef{Name: "bundle", Key: "root.pem"},
	}
	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	if got, ok := envValue(dep.Spec.Template.Spec.Containers[0].Env, "SSL_CERT_FILE"); !ok || got != tlsCAMountDir+"/root.pem" {
		t.Errorf("SSL_CERT_FILE = %q, want %q", got, tlsCAMountDir+"/root.pem")
	}
}

func TestBuildDeployment_TLSClientCert(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.TLS = &litellmv1alpha1.TLSSpec{
		ClientCertSecretRef: &litellmv1alpha1.SecretRef{Name: "outbound-client-tls"},
	}
	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	c := dep.Spec.Template.Spec.Containers[0]

	if got, ok := envValue(c.Env, "SSL_CERTIFICATE"); !ok || got != tlsClientMountDir+"/tls.crt" {
		t.Errorf("SSL_CERTIFICATE = %q (found=%v), want %q", got, ok, tlsClientMountDir+"/tls.crt")
	}
	if _, ok := mountByName(c.VolumeMounts, volumeNameTLSClient); !ok {
		t.Error("tls-client mount not found")
	}
}

func TestBuildDeployment_TLSNoneNoEnv(t *testing.T) {
	instance := newTestInstance()
	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	for _, name := range []string{"SSL_CERTFILE_PATH", "SSL_KEYFILE_PATH", "SSL_CERT_FILE", "SSL_CERTIFICATE"} {
		if _, ok := envValue(dep.Spec.Template.Spec.Containers[0].Env, name); ok {
			t.Errorf("%s should not be set when spec.tls is nil", name)
		}
	}
}

func TestBuildDeployment_DatabaseTLSMounts(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Database.TLS = &litellmv1alpha1.DatabaseTLSSpec{
		CASecretRef:         &litellmv1alpha1.CASecretRef{Name: "pg-ca"},
		ClientCertSecretRef: &litellmv1alpha1.SecretRef{Name: "pg-client"},
	}
	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)
	c := dep.Spec.Template.Spec.Containers[0]

	for _, vn := range []string{volumeNameDBTLSCA, volumeNameDBTLSClient} {
		if _, ok := volumeByName(dep.Spec.Template.Spec.Volumes, vn); !ok {
			t.Errorf("volume %q not found on deployment", vn)
		}
		if _, ok := mountByName(c.VolumeMounts, vn); !ok {
			t.Errorf("mount %q not found on container", vn)
		}
	}
}

func TestBuildMigrationJob_DatabaseTLSMounts(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.Database.Migration = &litellmv1alpha1.MigrationSpec{Enabled: true}
	instance.Spec.Database.TLS = &litellmv1alpha1.DatabaseTLSSpec{
		CASecretRef: &litellmv1alpha1.CASecretRef{Name: "pg-ca"},
	}
	job := BuildMigrationJob(instance, map[string]string{"app": "litellm"})
	podSpec := job.Spec.Template.Spec

	if _, ok := volumeByName(podSpec.Volumes, volumeNameDBTLSCA); !ok {
		t.Error("migration job missing db-tls-ca volume")
	}
	if _, ok := mountByName(podSpec.Containers[0].VolumeMounts, volumeNameDBTLSCA); !ok {
		t.Error("migration job container missing db-tls-ca mount")
	}
}

func TestBuildDeployment_ExtraVolumesAndMounts(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.ExtraVolumes = []corev1.Volume{{
		Name:         "scratch",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}}
	instance.Spec.ExtraVolumeMounts = []corev1.VolumeMount{{
		Name: "scratch", MountPath: "/scratch",
	}}
	dep := BuildDeployment(instance, map[string]string{"app": "litellm"}, "", nil)

	if _, ok := volumeByName(dep.Spec.Template.Spec.Volumes, "scratch"); !ok {
		t.Error("extra volume 'scratch' not found")
	}
	if _, ok := mountByName(dep.Spec.Template.Spec.Containers[0].VolumeMounts, "scratch"); !ok {
		t.Error("extra volume mount 'scratch' not found")
	}
}

func TestProxyBaseURL_HTTPSWhenServingTLS(t *testing.T) {
	instance := newTestInstance()
	instance.Spec.TLS = &litellmv1alpha1.TLSSpec{
		ServerCertSecretRef: &litellmv1alpha1.SecretRef{Name: "gateway-server-tls"},
	}
	got := proxyBaseURL(instance)
	want := "https://test-instance.default.svc:4000"
	if got != want {
		t.Errorf("proxyBaseURL = %q, want %q", got, want)
	}
}

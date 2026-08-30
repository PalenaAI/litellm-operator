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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// keySyncHash must ignore the Kubernetes-side Secret plumbing: those fields never
// reach LiteLLM, so moving them must not look like drift and trigger a needless
// POST /key/update.
func TestKeySyncHashIgnoresSecretPlumbing(t *testing.T) {
	base := litellmv1alpha1.LiteLLMVirtualKeySpec{
		InstanceRef: litellmv1alpha1.InstanceRef{Name: "inst"},
		KeyAlias:    "ci",
		Models:      []string{"gpt-4o"},
	}

	withSecretName := base
	withSecretName.KeySecretName = "somewhere-else"

	withTemplate := base
	withTemplate.KeySecretTemplate = &litellmv1alpha1.KeySecretTemplateSpec{
		Annotations: map[string]string{"reflector.v1.k8s.emberstack.com/reflection-allowed": "true"},
		Labels:      map[string]string{"app.kubernetes.io/part-of": "checkout"},
	}

	if got, want := keySyncHash(withSecretName), keySyncHash(base); got != want {
		t.Errorf("keySecretName changed the sync hash: got %s, want %s", got, want)
	}
	if got, want := keySyncHash(withTemplate), keySyncHash(base); got != want {
		t.Errorf("keySecretTemplate changed the sync hash: got %s, want %s", got, want)
	}

	// A field that genuinely reaches LiteLLM must still move the hash, otherwise
	// the exclusion above would be masking real drift.
	changed := base
	changed.Models = []string{"gpt-4o", "claude-sonnet-4"}
	if keySyncHash(changed) == keySyncHash(base) {
		t.Error("models did not change the sync hash; drift detection is broken")
	}
}

// keySyncHash must not mutate the caller's spec while zeroing fields on its copy.
func TestKeySyncHashDoesNotMutateSpec(t *testing.T) {
	spec := litellmv1alpha1.LiteLLMVirtualKeySpec{
		InstanceRef:       litellmv1alpha1.InstanceRef{Name: "inst"},
		KeyAlias:          "ci",
		KeySecretName:     "pinned",
		KeySecretTemplate: &litellmv1alpha1.KeySecretTemplateSpec{Labels: map[string]string{"a": "b"}},
	}
	_ = keySyncHash(spec)
	if spec.KeySecretName != "pinned" {
		t.Errorf("keySyncHash cleared caller's KeySecretName: %q", spec.KeySecretName)
	}
	if spec.KeySecretTemplate == nil {
		t.Error("keySyncHash cleared caller's KeySecretTemplate")
	}
}

var _ = Describe("LiteLLMVirtualKey key Secret", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "vks-instance"
		keyName      = "vks-key"
		secretName   = keyName + "-key"
	)

	reflectorAnnotations := map[string]string{
		"reflector.v1.k8s.emberstack.com/reflection-allowed":            "true",
		"reflector.v1.k8s.emberstack.com/reflection-auto-enabled":       "true",
		"reflector.v1.k8s.emberstack.com/reflection-auto-namespaces":    "apps",
		"reflector.v1.k8s.emberstack.com/reflection-allowed-namespaces": "apps",
	}

	newReconciler := func(mock *litellm.MockClient) *LiteLLMVirtualKeyReconciler {
		return &LiteLLMVirtualKeyReconciler{
			Client:               k8sClient,
			Scheme:               k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
		}
	}

	reconcileKey := func(r *LiteLLMVirtualKeyReconciler) {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: keyName, Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())
	}

	getSecret := func() *corev1.Secret {
		GinkgoHelper()
		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, &secret)).To(Succeed())
		return &secret
	}

	createKey := func(tmpl *litellmv1alpha1.KeySecretTemplateSpec) {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMVirtualKey{
			ObjectMeta: metav1.ObjectMeta{Name: keyName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
				InstanceRef:       litellmv1alpha1.InstanceRef{Name: instanceName},
				KeyAlias:          keyName,
				KeySecretTemplate: tmpl,
			},
		})).To(Succeed())
	}

	BeforeEach(func() {
		instance := &litellmv1alpha1.LiteLLMInstance{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ns}, instance); err != nil && errors.IsNotFound(err) {
			instance = &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns},
				Spec: litellmv1alpha1.LiteLLMInstanceSpec{
					MasterKey: litellmv1alpha1.MasterKeySpec{AutoGenerate: true},
					Database: litellmv1alpha1.DatabaseSpec{
						External: &litellmv1alpha1.ExternalDBSpec{
							ConnectionSecretRef: litellmv1alpha1.SecretKeyRef{Name: "db", Key: "url"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		}
		instance.Status.Ready = true
		Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())

		mk := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName + "-master-key", Namespace: ns}, mk); err != nil && errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns},
				Data:       map[string][]byte{"master-key": []byte("sk-master")},
			})).To(Succeed())
		}
	})

	AfterEach(func() {
		vk := &litellmv1alpha1.LiteLLMVirtualKey{ObjectMeta: metav1.ObjectMeta{Name: keyName, Namespace: ns}}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(vk), vk); err == nil {
			if len(vk.GetFinalizers()) > 0 {
				vk.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, vk)
			}
			_ = k8sClient.Delete(ctx, vk)
		}
		// envtest runs no garbage collector, so owned Secrets must be cleaned up
		// explicitly between specs.
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns}})
		_ = k8sClient.Delete(ctx, &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}})
	})

	It("applies keySecretTemplate annotations and labels to the generated Secret", func() {
		createKey(&litellmv1alpha1.KeySecretTemplateSpec{
			Annotations: reflectorAnnotations,
			Labels:      map[string]string{"app.kubernetes.io/part-of": "checkout"},
		})
		reconcileKey(newReconciler(litellm.NewMockClient()))

		secret := getSecret()
		for k, v := range reflectorAnnotations {
			Expect(secret.Annotations).To(HaveKeyWithValue(k, v))
		}
		Expect(secret.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", "checkout"))
		// The operator's own labels must survive alongside the template's.
		Expect(secret.Labels).To(HaveKeyWithValue(LabelInstanceName, instanceName))
		Expect(secret.Labels).To(HaveKeyWithValue(LabelResourceType, "virtual-key"))
		Expect(secret.Data).To(HaveKey("api_key"))

		var vk litellmv1alpha1.LiteLLMVirtualKey
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: ns}, &vk)).To(Succeed())
		Expect(metav1.IsControlledBy(secret, &vk)).To(BeTrue())
	})

	// The template is excluded from the sync hash, so this only works if the
	// Secret is reconciled on every pass rather than only at mint time.
	It("applies a keySecretTemplate added after the key was minted, without touching LiteLLM", func() {
		createKey(nil)

		mock := litellm.NewMockClient()
		updates := 0
		mock.MockKeys.UpdateFunc = func(context.Context, litellm.KeyUpdateRequest) error {
			updates++
			return nil
		}
		r := newReconciler(mock)
		reconcileKey(r)
		Expect(getSecret().Annotations).NotTo(HaveKey("reflector.v1.k8s.emberstack.com/reflection-allowed"))

		var vk litellmv1alpha1.LiteLLMVirtualKey
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: ns}, &vk)).To(Succeed())
		vk.Spec.KeySecretTemplate = &litellmv1alpha1.KeySecretTemplateSpec{Annotations: reflectorAnnotations}
		Expect(k8sClient.Update(ctx, &vk)).To(Succeed())

		reconcileKey(r)

		secret := getSecret()
		for k, v := range reflectorAnnotations {
			Expect(secret.Annotations).To(HaveKeyWithValue(k, v))
		}
		Expect(updates).To(BeZero(), "editing Secret metadata must not push an update to LiteLLM")
	})

	// Merge semantics, matching the ServiceAccount/Deployment annotation handling:
	// what another controller adds to the Secret has to survive reconciliation.
	It("preserves annotations added by other controllers", func() {
		createKey(&litellmv1alpha1.KeySecretTemplateSpec{Annotations: reflectorAnnotations})
		r := newReconciler(litellm.NewMockClient())
		reconcileKey(r)

		secret := getSecret()
		secret.Annotations["reflector.v1.k8s.emberstack.com/reflected-at"] = "2026-08-30T00:00:00Z"
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())

		reconcileKey(r)

		secret = getSecret()
		Expect(secret.Annotations).To(HaveKey("reflector.v1.k8s.emberstack.com/reflected-at"))
		Expect(secret.Annotations).To(HaveKeyWithValue("reflector.v1.k8s.emberstack.com/reflection-allowed", "true"))
	})

	// Regression: the Secret used to be written only on the mint path, so deleting
	// it left the VirtualKey permanently unusable — LiteLLM stores only a hash of
	// the key, so the material cannot be recovered and the key must be rotated.
	It("rotates the key and recreates the Secret when the Secret is deleted", func() {
		createKey(&litellmv1alpha1.KeySecretTemplateSpec{Annotations: reflectorAnnotations})

		mock := litellm.NewMockClient()
		mints := 0
		mock.MockKeys.GenerateFunc = func(context.Context, litellm.KeyGenerateRequest) (*litellm.KeyGenerateResponse, error) {
			mints++
			return &litellm.KeyGenerateResponse{
				Key:   fmt.Sprintf("sk-rotated-%d", mints),
				Token: fmt.Sprintf("tok-%d", mints),
			}, nil
		}
		deleted := []string{}
		mock.MockKeys.DeleteFunc = func(_ context.Context, token string) error {
			deleted = append(deleted, token)
			return nil
		}
		r := newReconciler(mock)

		reconcileKey(r)
		first := getSecret()
		Expect(mints).To(Equal(1))

		Expect(k8sClient.Delete(ctx, first)).To(Succeed())
		reconcileKey(r)

		second := getSecret()
		Expect(mints).To(Equal(2), "a missing Secret must mint a replacement key")
		Expect(deleted).To(ContainElement("tok-1"), "the orphaned key must be deleted from LiteLLM")
		Expect(second.Data["api_key"]).NotTo(Equal(first.Data["api_key"]))
		// The template still applies to the replacement.
		Expect(second.Annotations).To(HaveKeyWithValue("reflector.v1.k8s.emberstack.com/reflection-allowed", "true"))

		var vk litellmv1alpha1.LiteLLMVirtualKey
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: ns}, &vk)).To(Succeed())
		Expect(vk.Status.LiteLLMKeyToken).To(Equal("tok-2"))
	})

	// Regression: the old AlreadyExists path filled in the key but never set an
	// ownerReference, so a hand-created Secret leaked the credential on delete.
	It("adopts a pre-created Secret so it is still garbage collected", func() {
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        secretName,
				Namespace:   ns,
				Annotations: map[string]string{"set-by": "human"},
			},
		})).To(Succeed())

		createKey(&litellmv1alpha1.KeySecretTemplateSpec{Annotations: reflectorAnnotations})
		reconcileKey(newReconciler(litellm.NewMockClient()))

		secret := getSecret()
		Expect(secret.OwnerReferences).To(HaveLen(1))
		Expect(secret.OwnerReferences[0].Kind).To(Equal("LiteLLMVirtualKey"))
		Expect(secret.OwnerReferences[0].Controller).NotTo(BeNil())
		Expect(*secret.OwnerReferences[0].Controller).To(BeTrue())
		Expect(secret.Data).To(HaveKey("api_key"))
		Expect(secret.Annotations).To(HaveKeyWithValue("set-by", "human"))
		Expect(secret.Annotations).To(HaveKeyWithValue("reflector.v1.k8s.emberstack.com/reflection-allowed", "true"))
	})
})

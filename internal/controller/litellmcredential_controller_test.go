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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

var _ = Describe("LiteLLMCredential Controller", func() {
	Context("When reconciling a credential", func() {
		const (
			credName     = "test-credential"
			instanceName = "test-credential-instance"
			secretName   = "test-credential-secret"
		)
		ctx := context.Background()
		credKey := types.NamespacedName{Name: credName, Namespace: "default"}
		instanceKey := types.NamespacedName{Name: instanceName, Namespace: "default"}
		secretKey := types.NamespacedName{Name: secretName, Namespace: "default"}

		BeforeEach(func() {
			// Referenced instance
			instance := &litellmv1alpha1.LiteLLMInstance{}
			if err := k8sClient.Get(ctx, instanceKey, instance); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMInstance{
					ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: "default"},
					Spec: litellmv1alpha1.LiteLLMInstanceSpec{
						MasterKey: litellmv1alpha1.MasterKeySpec{AutoGenerate: true},
						Database: litellmv1alpha1.DatabaseSpec{
							External: &litellmv1alpha1.ExternalDBSpec{
								ConnectionSecretRef: litellmv1alpha1.SecretKeyRef{Name: "db", Key: "url"},
							},
						},
					},
				})).To(Succeed())
			}

			// API key Secret
			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, secretKey, secret); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"},
					Data:       map[string][]byte{"api-key": []byte("sk-test-value")},
				})).To(Succeed())
			}

			// Credential CR
			cred := &litellmv1alpha1.LiteLLMCredential{}
			if err := k8sClient.Get(ctx, credKey, cred); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMCredential{
					ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: "default"},
					Spec: litellmv1alpha1.LiteLLMCredentialSpec{
						InstanceRef:    litellmv1alpha1.InstanceRef{Name: instanceName},
						CredentialName: "openai-prod",
						APIKeySecretRef: litellmv1alpha1.SecretKeyRef{
							Name: secretName,
							Key:  "api-key",
						},
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() {
			cred := &litellmv1alpha1.LiteLLMCredential{}
			if err := k8sClient.Get(ctx, credKey, cred); err == nil {
				// Drop the finalizer so the object actually deletes during GC.
				if len(cred.Finalizers) > 0 {
					cred.Finalizers = nil
					Expect(k8sClient.Update(ctx, cred)).To(Succeed())
				}
				Expect(k8sClient.Delete(ctx, cred)).To(Succeed())
			}
			instance := &litellmv1alpha1.LiteLLMInstance{}
			if err := k8sClient.Get(ctx, instanceKey, instance); err == nil {
				Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
			}
			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, secretKey, secret); err == nil {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should validate credential and set Ready condition", func() {
			reconciler := &LiteLLMCredentialReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: credKey})
			Expect(err).NotTo(HaveOccurred())

			// Finalizer should be added on the first pass — run once more to exercise the validation path.
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: credKey})
			Expect(err).NotTo(HaveOccurred())

			updated := &litellmv1alpha1.LiteLLMCredential{}
			Expect(k8sClient.Get(ctx, credKey, updated)).To(Succeed())
			Expect(updated.Finalizers).To(ContainElement(FinalizerName))
			Expect(updated.Status.Configured).To(BeTrue())

			var readyFound bool
			for _, c := range updated.Status.Conditions {
				if c.Type == ConditionReady {
					readyFound = true
					Expect(string(c.Status)).To(Equal("True"))
					Expect(c.Reason).To(Equal("Validated"))
				}
			}
			Expect(readyFound).To(BeTrue(), "Ready condition not found")
		})

		It("should mark credential NotReady when Secret key is missing", func() {
			// Overwrite the Secret with a missing key to trigger the SecretKeyMissing branch.
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
			secret.Data = map[string][]byte{"wrong-key": []byte("value")}
			Expect(k8sClient.Update(ctx, secret)).To(Succeed())

			reconciler := &LiteLLMCredentialReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			// First pass adds the finalizer; second actually validates.
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: credKey})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: credKey})
			Expect(err).NotTo(HaveOccurred())

			updated := &litellmv1alpha1.LiteLLMCredential{}
			Expect(k8sClient.Get(ctx, credKey, updated)).To(Succeed())
			Expect(updated.Status.Configured).To(BeFalse())

			var reason string
			for _, c := range updated.Status.Conditions {
				if c.Type == ConditionReady {
					reason = c.Reason
					Expect(string(c.Status)).To(Equal("False"))
				}
			}
			Expect(reason).To(Equal("SecretKeyMissing"))
		})
	})
})

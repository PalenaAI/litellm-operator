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

// These tests exercise the model controller's provider-auth payload build for
// both auth modes (inline and credentialRef) and the transition between them.
// They guard the Azure credentialRef fix: the credential's api_base /
// api_version / api_key must be written INLINE on the /model/new payload, not
// left to LiteLLM's (cold-start-unreliable) named-credential resolution.
var _ = Describe("LiteLLMModel Controller — provider auth payload", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "auth-instance"
		modelName    = "auth-model"
		credName     = "auth-cred"
		modelSecret  = "auth-model-secret"
		credSecret   = "auth-cred-secret"
	)
	modelKey := types.NamespacedName{Name: modelName, Namespace: ns}

	// makeInstanceReady creates a Ready LiteLLMInstance plus its master-key
	// Secret so resolveInstance succeeds.
	makeInstanceReady := func() {
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
	}

	makeSecret := func(name, key, value string) {
		s := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, s); err != nil && errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Data:       map[string][]byte{key: []byte(value)},
			})).To(Succeed())
		}
	}

	BeforeEach(func() {
		makeInstanceReady()
	})

	AfterEach(func() {
		for _, obj := range []client.Object{
			&litellmv1alpha1.LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns}},
			&litellmv1alpha1.LiteLLMCredential{ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: ns}},
		} {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			if len(obj.GetFinalizers()) > 0 {
				obj.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, obj)
			}
			_ = k8sClient.Delete(ctx, obj)
		}
		for _, n := range []string{modelSecret, credSecret, instanceName + "-master-key"} {
			s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns}}
			_ = k8sClient.Delete(ctx, s)
		}
		inst := &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}}
		_ = k8sClient.Delete(ctx, inst)
	})

	It("inline auth: writes api_base/api_version/api_key, no credential name", func() {
		makeSecret(modelSecret, "api-key", "sk-inline")
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				ModelName:   "gpt-5.4",
				LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
					Model:           "azure/gpt-5.4",
					APIBase:         "https://forge-foundry.cognitiveservices.azure.com",
					APIVersion:      "2024-10-21",
					APIKeySecretRef: &litellmv1alpha1.SecretKeyRef{Name: modelSecret, Key: "api-key"},
				},
			},
		})).To(Succeed())

		var captured litellm.ModelCreateRequest
		mock := litellm.NewMockClient()
		mock.MockModels.CreateFunc = func(_ context.Context, req litellm.ModelCreateRequest) (*litellm.ModelCreateResponse, error) {
			captured = req
			return &litellm.ModelCreateResponse{ModelID: "m-inline"}, nil
		}
		r := &LiteLLMModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: modelKey})
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.LiteLLMParams.APIBase).To(Equal("https://forge-foundry.cognitiveservices.azure.com"))
		Expect(captured.LiteLLMParams.APIVersion).To(Equal("2024-10-21"))
		Expect(captured.LiteLLMParams.APIKey).To(Equal("sk-inline"))
		Expect(captured.LiteLLMParams.LiteLLMCredentialName).To(BeEmpty())

		updated := &litellmv1alpha1.LiteLLMModel{}
		Expect(k8sClient.Get(ctx, modelKey, updated)).To(Succeed())
		Expect(updated.Annotations[AnnotationAuthMode]).To(Equal(authModeInline))
	})

	It("credentialRef auth: resolves credential values INLINE plus the credential name", func() {
		makeSecret(credSecret, "OPENAI_API_KEY", "sk-from-credential")
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMCredential{
			ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: instanceName},
				CredentialName:  "gpt-5.4-credential",
				APIBase:         "https://forge-foundry.cognitiveservices.azure.com",
				APIVersion:      "2024-10-21",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: credSecret, Key: "OPENAI_API_KEY"},
			},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				ModelName:   "gpt-5.4",
				LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
					Model:         "azure/gpt-5.4",
					CredentialRef: &litellmv1alpha1.CredentialRef{Name: credName},
				},
			},
		})).To(Succeed())

		var captured litellm.ModelCreateRequest
		mock := litellm.NewMockClient()
		mock.MockModels.CreateFunc = func(_ context.Context, req litellm.ModelCreateRequest) (*litellm.ModelCreateResponse, error) {
			captured = req
			return &litellm.ModelCreateResponse{ModelID: "m-cred"}, nil
		}
		r := &LiteLLMModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: modelKey})
		Expect(err).NotTo(HaveOccurred())

		// The fix: Azure endpoint/version/key are inline so a DB model works
		// regardless of LiteLLM's startup credential-list load ordering.
		Expect(captured.LiteLLMParams.APIBase).To(Equal("https://forge-foundry.cognitiveservices.azure.com"))
		Expect(captured.LiteLLMParams.APIVersion).To(Equal("2024-10-21"))
		Expect(captured.LiteLLMParams.APIKey).To(Equal("sk-from-credential"))
		// Still sent for Admin UI association / extra-param merge.
		Expect(captured.LiteLLMParams.LiteLLMCredentialName).To(Equal("gpt-5.4-credential"))

		updated := &litellmv1alpha1.LiteLLMModel{}
		Expect(k8sClient.Get(ctx, modelKey, updated)).To(Succeed())
		Expect(updated.Annotations[AnnotationAuthMode]).To(Equal(authModeCredential))
	})

	It("switching credentialRef -> inline deletes and recreates (clears stale fields)", func() {
		makeSecret(credSecret, "OPENAI_API_KEY", "sk-from-credential")
		makeSecret(modelSecret, "api-key", "sk-inline")
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMCredential{
			ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMCredentialSpec{
				InstanceRef:     litellmv1alpha1.InstanceRef{Name: instanceName},
				CredentialName:  "gpt-5.4-credential",
				APIBase:         "https://forge-foundry.cognitiveservices.azure.com",
				APIVersion:      "2024-10-21",
				APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: credSecret, Key: "OPENAI_API_KEY"},
			},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				ModelName:   "gpt-5.4",
				LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
					Model:         "azure/gpt-5.4",
					CredentialRef: &litellmv1alpha1.CredentialRef{Name: credName},
				},
			},
		})).To(Succeed())

		var creates, updates, deletes int
		mock := litellm.NewMockClient()
		mock.MockModels.CreateFunc = func(_ context.Context, _ litellm.ModelCreateRequest) (*litellm.ModelCreateResponse, error) {
			creates++
			return &litellm.ModelCreateResponse{ModelID: "m-switch"}, nil
		}
		mock.MockModels.UpdateFunc = func(_ context.Context, _ litellm.ModelCreateRequest) error { updates++; return nil }
		mock.MockModels.DeleteFunc = func(_ context.Context, _ string) error { deletes++; return nil }
		r := &LiteLLMModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		// Initial create in credential mode.
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: modelKey})
		Expect(err).NotTo(HaveOccurred())
		Expect(creates).To(Equal(1))

		// Flip to inline auth.
		m := &litellmv1alpha1.LiteLLMModel{}
		Expect(k8sClient.Get(ctx, modelKey, m)).To(Succeed())
		m.Spec.LiteLLMParams.CredentialRef = nil
		m.Spec.LiteLLMParams.APIBase = "https://forge-foundry.cognitiveservices.azure.com"
		m.Spec.LiteLLMParams.APIVersion = "2024-10-21"
		m.Spec.LiteLLMParams.APIKeySecretRef = &litellmv1alpha1.SecretKeyRef{Name: modelSecret, Key: "api-key"}
		Expect(k8sClient.Update(ctx, m)).To(Succeed())

		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: modelKey})
		Expect(err).NotTo(HaveOccurred())

		// Mode flip must delete+recreate, never a plain update.
		Expect(deletes).To(Equal(1))
		Expect(creates).To(Equal(2))
		Expect(updates).To(Equal(0))

		updated := &litellmv1alpha1.LiteLLMModel{}
		Expect(k8sClient.Get(ctx, modelKey, updated)).To(Succeed())
		Expect(updated.Annotations[AnnotationAuthMode]).To(Equal(authModeInline))
	})
})

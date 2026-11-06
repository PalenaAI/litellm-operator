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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

var _ = Describe("Cross-namespace instanceRef", func() {
	ctx := context.Background()
	const (
		instanceNS   = "xns-gateway"
		tenantNS     = "xns-tenant"
		otherNS      = "xns-other"
		instanceName = "xns-instance"
		keyName      = "xns-key"
		tenantLabel  = "litellm.palena.ai/tenant"
	)
	instanceNN := types.NamespacedName{Name: instanceName, Namespace: instanceNS}
	keyNN := types.NamespacedName{Name: keyName, Namespace: tenantNS}

	ensureNamespace := func(name string, labels map[string]string) {
		GinkgoHelper()
		ns := &corev1.Namespace{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, ns); errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
			})).To(Succeed())
		}
	}

	setAllowed := func(allowed *litellmv1alpha1.AllowedNamespaces) {
		GinkgoHelper()
		instance := &litellmv1alpha1.LiteLLMInstance{}
		Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
		instance.Spec.AllowedNamespaces = allowed
		Expect(k8sClient.Update(ctx, instance)).To(Succeed())
	}

	tenantSelector := &litellmv1alpha1.AllowedNamespaces{
		From:     litellmv1alpha1.NamespacesFromSelector,
		Selector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: "true"}},
	}

	newKeyReconciler := func(mock *litellm.MockClient) *LiteLLMVirtualKeyReconciler {
		return &LiteLLMVirtualKeyReconciler{
			Client:               k8sClient,
			Scheme:               k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
		}
	}

	createKey := func(namespace string) {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMVirtualKey{
			ObjectMeta: metav1.ObjectMeta{Name: keyName, Namespace: namespace},
			Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
				KeyAlias:    keyName,
			},
		})).To(Succeed())
	}

	BeforeEach(func() {
		ensureNamespace(instanceNS, nil)
		ensureNamespace(tenantNS, map[string]string{tenantLabel: "true"})
		ensureNamespace(otherNS, nil)

		instance := &litellmv1alpha1.LiteLLMInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: instanceNS},
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
		instance.Status.Ready = true
		instance.Status.Endpoint = "http://litellm:4000"
		Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())

		// The master key exists only in the instance's namespace, so a key in
		// a tenant namespace can only mint if it is read from there.
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: instanceNS},
			Data:       map[string][]byte{"master-key": []byte("sk-master")},
		})).To(Succeed())
	})

	AfterEach(func() {
		for _, ns := range []string{tenantNS, otherNS} {
			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: ns}, vk); err == nil {
				vk.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, vk)
				_ = k8sClient.Delete(ctx, vk)
			}
			// envtest runs no garbage collector.
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: keyName + "-key", Namespace: ns}})
		}
		var models litellmv1alpha1.LiteLLMModelList
		_ = k8sClient.List(ctx, &models, client.InNamespace(tenantNS))
		for i := range models.Items {
			m := &models.Items[i]
			m.Finalizers = nil
			_ = k8sClient.Update(ctx, m)
			_ = k8sClient.Delete(ctx, m)
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: instanceNS}})
		instance := &litellmv1alpha1.LiteLLMInstance{}
		if err := k8sClient.Get(ctx, instanceNN, instance); err == nil {
			instance.Finalizers = nil
			_ = k8sClient.Update(ctx, instance)
			_ = k8sClient.Delete(ctx, instance)
		}
	})

	Describe("instanceAllowsNamespace", func() {
		check := func(allowed *litellmv1alpha1.AllowedNamespaces, namespace string) bool {
			GinkgoHelper()
			instance := &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: instanceNS},
				Spec:       litellmv1alpha1.LiteLLMInstanceSpec{AllowedNamespaces: allowed},
			}
			ok, err := instanceAllowsNamespace(ctx, k8sClient, instance, namespace)
			Expect(err).NotTo(HaveOccurred())
			return ok
		}

		It("always allows the instance's own namespace", func() {
			Expect(check(nil, instanceNS)).To(BeTrue())
		})
		It("allows no other namespace by default", func() {
			Expect(check(nil, tenantNS)).To(BeFalse())
			Expect(check(&litellmv1alpha1.AllowedNamespaces{From: litellmv1alpha1.NamespacesFromSame}, tenantNS)).To(BeFalse())
		})
		It("allows every namespace with All", func() {
			Expect(check(&litellmv1alpha1.AllowedNamespaces{From: litellmv1alpha1.NamespacesFromAll}, otherNS)).To(BeTrue())
		})
		It("allows only matching namespaces with Selector", func() {
			Expect(check(tenantSelector, tenantNS)).To(BeTrue())
			Expect(check(tenantSelector, otherNS)).To(BeFalse())
		})
	})

	Describe("a LiteLLMVirtualKey in another namespace", func() {
		It("is refused with RefNotPermitted when the instance does not allow its namespace", func() {
			createKey(otherNS)
			generated := false
			mock := litellm.NewMockClient()
			mock.MockKeys.GenerateFunc = func(context.Context, litellm.KeyGenerateRequest) (*litellm.KeyGenerateResponse, error) {
				generated = true
				return &litellm.KeyGenerateResponse{Key: "sk-x", Token: "t"}, nil
			}

			_, err := newKeyReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: keyName, Namespace: otherNS}})
			Expect(err).NotTo(HaveOccurred())
			Expect(generated).To(BeFalse())

			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: otherNS}, vk)).To(Succeed())
			cond := meta.FindStatusCondition(vk.Status.Conditions, ConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(ReasonRefNotPermitted))
		})

		It("mints its key into a Secret in its own namespace when the instance allows it", func() {
			setAllowed(tenantSelector)
			createKey(tenantNS)

			_, err := newKeyReconciler(litellm.NewMockClient()).Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())

			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			Expect(vk.Status.LiteLLMKeyToken).To(Equal("mock-token-hash"))

			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName + "-key", Namespace: tenantNS}, secret)).To(Succeed())
			Expect(secret.OwnerReferences).To(HaveLen(1))
			Expect(secret.OwnerReferences[0].Name).To(Equal(keyName))
		})

		It("still deletes its key from LiteLLM after the instance stops allowing it", func() {
			setAllowed(tenantSelector)
			createKey(tenantNS)
			r := newKeyReconciler(litellm.NewMockClient())
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())

			setAllowed(nil)
			deleted := ""
			mock := litellm.NewMockClient()
			mock.MockKeys.DeleteFunc = func(_ context.Context, token string) error {
				deleted = token
				return nil
			}
			r = newKeyReconciler(mock)
			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			Expect(k8sClient.Delete(ctx, vk)).To(Succeed())
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(Equal("mock-token-hash"))
		})
	})

	Describe("config sync", func() {
		It("counts a model in an allowed namespace as managed instead of pruning it", func() {
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			instance.Spec.AllowedNamespaces = tenantSelector
			instance.Spec.ConfigSync = &litellmv1alpha1.ConfigSyncSpec{
				Enabled:                 true,
				Interval:                "30s",
				UnmanagedResourcePolicy: "prune",
				ConflictResolution:      "crd-wins",
			}
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())

			model := &litellmv1alpha1.LiteLLMModel{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-model", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMModelSpec{
					InstanceRef:   litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					ModelName:     "gpt-4",
					LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{Model: "openai/gpt-4"},
				},
			}
			Expect(k8sClient.Create(ctx, model)).To(Succeed())
			model.Status.LiteLLMModelID = "xns-model-id"
			model.Status.Synced = true
			Expect(k8sClient.Status().Update(ctx, model)).To(Succeed())

			pruned := false
			mock := litellm.NewMockClient()
			mock.MockModels.ListFunc = func(context.Context) ([]litellm.ModelInfoResponse, error) {
				return []litellm.ModelInfoResponse{
					{ModelID: "xns-model-id", ModelName: "gpt-4", Params: litellm.ModelParams{Model: "openai/gpt-4"}},
				}, nil
			}
			mock.MockModels.DeleteFunc = func(context.Context, string) error {
				pruned = true
				return nil
			}

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(pruned).To(BeFalse())

			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			Expect(instance.Status.ConfigSync).NotTo(BeNil())
			Expect(instance.Status.ConfigSync.SyncedModels).To(Equal(1))
		})
	})

	Describe("validation", func() {
		It("rejects instanceRef.namespace on a LiteLLMGuardrail", func() {
			err := k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMGuardrail{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-guardrail", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
					InstanceRef:   litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					GuardrailName: "xns",
					Provider:      "presidio",
					Mode:          "pre_call",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("instanceRef.namespace is not supported"))
		})

		It("requires a selector when allowedNamespaces.from is Selector", func() {
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			instance.Spec.AllowedNamespaces = &litellmv1alpha1.AllowedNamespaces{From: litellmv1alpha1.NamespacesFromSelector}
			err := k8sClient.Update(ctx, instance)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("selector is required"))
		})
	})
})

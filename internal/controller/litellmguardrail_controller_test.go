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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

var _ = Describe("LiteLLMGuardrail Controller guardrailClass validation", func() {
	ctx := context.Background()

	reconcileGuardrail := func(key types.NamespacedName) {
		r := &LiteLLMGuardrailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
		// First pass adds the finalizer, second pass runs validation.
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
	}

	readyReason := func(key types.NamespacedName) (string, metav1.ConditionStatus) {
		updated := &litellmv1alpha1.LiteLLMGuardrail{}
		Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
		for _, c := range updated.Status.Conditions {
			if c.Type == ConditionReady {
				return c.Reason, c.Status
			}
		}
		return "", ""
	}

	cleanup := func(key types.NamespacedName) {
		g := &litellmv1alpha1.LiteLLMGuardrail{}
		if err := k8sClient.Get(ctx, key, g); err == nil {
			if len(g.Finalizers) > 0 {
				g.Finalizers = nil
				Expect(k8sClient.Update(ctx, g)).To(Succeed())
			}
			Expect(k8sClient.Delete(ctx, g)).To(Succeed())
		}
	}

	Context("when provider is custom_guardrail without guardrailClass", func() {
		const name = "test-guardrail-missing-class"
		key := types.NamespacedName{Name: name, Namespace: "default"}

		BeforeEach(func() {
			g := &litellmv1alpha1.LiteLLMGuardrail{}
			if err := k8sClient.Get(ctx, key, g); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMGuardrail{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
						InstanceRef:   litellmv1alpha1.InstanceRef{Name: "any-instance"},
						GuardrailName: "my-custom",
						Provider:      "custom_guardrail",
						Mode:          "pre_call",
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() { cleanup(key) })

		It("reports Ready=False with reason GuardrailClassRequired", func() {
			reconcileGuardrail(key)
			reason, status := readyReason(key)
			Expect(reason).To(Equal("GuardrailClassRequired"))
			Expect(status).To(Equal(metav1.ConditionFalse))

			updated := &litellmv1alpha1.LiteLLMGuardrail{}
			Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
			Expect(updated.Status.Configured).To(BeFalse())
		})
	})

	Context("when a non-custom provider sets guardrailClass", func() {
		const name = "test-guardrail-class-not-allowed"
		key := types.NamespacedName{Name: name, Namespace: "default"}

		BeforeEach(func() {
			g := &litellmv1alpha1.LiteLLMGuardrail{}
			if err := k8sClient.Get(ctx, key, g); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMGuardrail{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
						InstanceRef:    litellmv1alpha1.InstanceRef{Name: "any-instance"},
						GuardrailName:  "aporia-pii",
						Provider:       "aporia",
						Mode:           "pre_call",
						GuardrailClass: "my_pkg.adapters.MyGuardrail",
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() { cleanup(key) })

		It("reports Ready=False with reason GuardrailClassNotAllowed", func() {
			reconcileGuardrail(key)
			reason, status := readyReason(key)
			Expect(reason).To(Equal("GuardrailClassNotAllowed"))
			Expect(status).To(Equal(metav1.ConditionFalse))

			updated := &litellmv1alpha1.LiteLLMGuardrail{}
			Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
			Expect(updated.Status.Configured).To(BeFalse())
		})
	})

	Context("when provider is generic_guardrail_api without apiBase", func() {
		const name = "test-guardrail-missing-apibase"
		key := types.NamespacedName{Name: name, Namespace: "default"}

		BeforeEach(func() {
			g := &litellmv1alpha1.LiteLLMGuardrail{}
			if err := k8sClient.Get(ctx, key, g); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMGuardrail{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
						InstanceRef:   litellmv1alpha1.InstanceRef{Name: "any-instance"},
						GuardrailName: "ext-http",
						Provider:      "generic_guardrail_api",
						Mode:          "pre_call",
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() { cleanup(key) })

		It("reports Ready=False with reason APIBaseRequired", func() {
			reconcileGuardrail(key)
			reason, status := readyReason(key)
			Expect(reason).To(Equal("APIBaseRequired"))
			Expect(status).To(Equal(metav1.ConditionFalse))

			updated := &litellmv1alpha1.LiteLLMGuardrail{}
			Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
			Expect(updated.Status.Configured).To(BeFalse())
		})
	})

	Context("when unreachableFallback is set on a non-generic provider", func() {
		const name = "test-guardrail-fallback-not-allowed"
		key := types.NamespacedName{Name: name, Namespace: "default"}

		BeforeEach(func() {
			g := &litellmv1alpha1.LiteLLMGuardrail{}
			if err := k8sClient.Get(ctx, key, g); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMGuardrail{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
						InstanceRef:         litellmv1alpha1.InstanceRef{Name: "any-instance"},
						GuardrailName:       "aporia-pii",
						Provider:            "aporia",
						Mode:                "pre_call",
						UnreachableFallback: "fail_open",
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() { cleanup(key) })

		It("reports Ready=False with reason UnreachableFallbackNotAllowed", func() {
			reconcileGuardrail(key)
			reason, status := readyReason(key)
			Expect(reason).To(Equal("UnreachableFallbackNotAllowed"))
			Expect(status).To(Equal(metav1.ConditionFalse))

			updated := &litellmv1alpha1.LiteLLMGuardrail{}
			Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
			Expect(updated.Status.Configured).To(BeFalse())
		})
	})
})

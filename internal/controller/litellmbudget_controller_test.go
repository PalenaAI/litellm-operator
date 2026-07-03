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

var _ = Describe("LiteLLMBudget Controller", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "bdg-instance"
		budgetName   = "bdg-tier"
	)
	budgetKey := types.NamespacedName{Name: budgetName, Namespace: ns}

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

	BeforeEach(func() { makeInstanceReady() })

	AfterEach(func() {
		b := &litellmv1alpha1.LiteLLMBudget{ObjectMeta: metav1.ObjectMeta{Name: budgetName, Namespace: ns}}
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(b), b)
		if len(b.GetFinalizers()) > 0 {
			b.SetFinalizers(nil)
			_ = k8sClient.Update(ctx, b)
		}
		_ = k8sClient.Delete(ctx, b)
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns}})
		_ = k8sClient.Delete(ctx, &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}})
	})

	It("creates the budget with budgetId defaulting to the object name", func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMBudget{
			ObjectMeta: metav1.ObjectMeta{Name: budgetName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMBudgetSpec{
				InstanceRef:    litellmv1alpha1.InstanceRef{Name: instanceName},
				MaxBudget:      floatptr(100),
				SoftBudget:     floatptr(80),
				BudgetDuration: "30d",
				RPMLimit:       intptr(1000),
			},
		})).To(Succeed())

		var captured litellm.BudgetRequest
		mock := litellm.NewMockClient()
		mock.MockBudgets.CreateFunc = func(_ context.Context, req litellm.BudgetRequest) (*litellm.BudgetResponse, error) {
			captured = req
			return &litellm.BudgetResponse{BudgetID: req.BudgetID}, nil
		}
		r := &LiteLLMBudgetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: budgetKey})
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.BudgetID).To(Equal(budgetName))
		Expect(captured.MaxBudget).To(Equal(floatptr(100)))
		Expect(captured.SoftBudget).To(Equal(floatptr(80)))
		Expect(captured.BudgetDuration).To(Equal("30d"))
		Expect(captured.RPMLimit).To(Equal(intptr(1000)))

		updated := &litellmv1alpha1.LiteLLMBudget{}
		Expect(k8sClient.Get(ctx, budgetKey, updated)).To(Succeed())
		Expect(updated.Status.LiteLLMBudgetID).To(Equal(budgetName))
	})
})

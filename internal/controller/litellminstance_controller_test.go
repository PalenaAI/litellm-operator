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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

var _ = Describe("LiteLLMInstance Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-instance"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind LiteLLMInstance")
			instance := &litellmv1alpha1.LiteLLMInstance{}
			err := k8sClient.Get(ctx, typeNamespacedName, instance)
			if err != nil && errors.IsNotFound(err) {
				resource := &litellmv1alpha1.LiteLLMInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: litellmv1alpha1.LiteLLMInstanceSpec{
						MasterKey: litellmv1alpha1.MasterKeySpec{
							AutoGenerate: true,
						},
						Database: litellmv1alpha1.DatabaseSpec{
							External: &litellmv1alpha1.ExternalDBSpec{
								ConnectionSecretRef: litellmv1alpha1.SecretKeyRef{
									Name: "test-db-secret",
									Key:  "url",
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &litellmv1alpha1.LiteLLMInstance{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &LiteLLMInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

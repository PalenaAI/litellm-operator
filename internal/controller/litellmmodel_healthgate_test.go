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

// These tests guard the cost safeguard: the operator must only call GET /health
// (which live-probes every model — a paid completion each — when background
// checks are off) when the instance has opted in via backgroundHealthChecks:true.
var _ = Describe("LiteLLMModel Controller — health-poll gating", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "hg-instance"
		modelName    = "hg-model"
	)
	modelKey := types.NamespacedName{Name: modelName, Namespace: ns}

	// makeInstance creates a Ready instance with the given backgroundHealthChecks
	// setting (nil = unset).
	makeInstance := func(bg *bool) {
		gs := &litellmv1alpha1.GeneralSettingsSpec{BackgroundHealthChecks: bg}
		instance := &litellmv1alpha1.LiteLLMInstance{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ns}, instance); err != nil && errors.IsNotFound(err) {
			instance = &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns},
				Spec: litellmv1alpha1.LiteLLMInstanceSpec{
					MasterKey:       litellmv1alpha1.MasterKeySpec{AutoGenerate: true},
					GeneralSettings: gs,
					Database: litellmv1alpha1.DatabaseSpec{
						External: &litellmv1alpha1.ExternalDBSpec{
							ConnectionSecretRef: litellmv1alpha1.SecretKeyRef{Name: "db", Key: "url"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		} else {
			instance.Spec.GeneralSettings = gs
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
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

	reconcileWith := func(bg *bool) (healthCalled bool, health string) {
		makeInstance(bg)
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: instanceName},
				ModelName:     "gpt-4o",
				LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{Model: "openai/gpt-4o"},
			},
		})).To(Succeed())

		called := false
		mock := litellm.NewMockClient()
		mock.MockHealth.CheckFunc = func(_ context.Context) (*litellm.HealthCheckResponse, error) {
			called = true
			return &litellm.HealthCheckResponse{
				HealthyEndpoints: []litellm.HealthEndpoint{{Model: "gpt-4o"}},
			}, nil
		}
		r := &LiteLLMModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: modelKey})
		Expect(err).NotTo(HaveOccurred())

		updated := &litellmv1alpha1.LiteLLMModel{}
		Expect(k8sClient.Get(ctx, modelKey, updated)).To(Succeed())
		return called, updated.Status.Health
	}

	AfterEach(func() {
		m := &litellmv1alpha1.LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns}}
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(m), m)
		if len(m.GetFinalizers()) > 0 {
			m.SetFinalizers(nil)
			_ = k8sClient.Update(ctx, m)
		}
		_ = k8sClient.Delete(ctx, m)
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns}})
		_ = k8sClient.Delete(ctx, &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}})
	})

	It("does NOT call GET /health when backgroundHealthChecks is unset", func() {
		called, health := reconcileWith(nil)
		Expect(called).To(BeFalse(), "operator must not live-probe /health when checks are off")
		Expect(health).To(Equal("unknown"))
	})

	It("does NOT call GET /health when backgroundHealthChecks is false", func() {
		called, health := reconcileWith(boolptr(false))
		Expect(called).To(BeFalse(), "operator must not live-probe /health when checks are disabled")
		Expect(health).To(Equal("unknown"))
	})

	It("DOES call GET /health when backgroundHealthChecks is true (cached, cheap)", func() {
		called, health := reconcileWith(boolptr(true))
		Expect(called).To(BeTrue())
		Expect(health).To(Equal("healthy"))
	})
})

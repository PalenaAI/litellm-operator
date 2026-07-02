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

// intptr / boolptr / floatptr are small helpers for building pointer-valued
// spec fields inline in the tests below.
func intptr(i int) *int           { return &i }
func boolptr(b bool) *bool        { return &b }
func floatptr(f float64) *float64 { return &f }

// These tests guard that the extended LiteLLMModel surface — routing params,
// model_info metadata/cost fields, and the per-model health-check controls —
// is faithfully forwarded onto the /model/new payload. The health-check fields
// in particular must be flattened under model_info to match LiteLLM's wire
// format.
var _ = Describe("LiteLLMModel Controller — extended field passthrough", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "fields-instance"
		modelName    = "fields-model"
	)
	modelKey := types.NamespacedName{Name: modelName, Namespace: ns}

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

	BeforeEach(func() {
		makeInstanceReady()
	})

	AfterEach(func() {
		m := &litellmv1alpha1.LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns}}
		_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(m), m)
		if len(m.GetFinalizers()) > 0 {
			m.SetFinalizers(nil)
			_ = k8sClient.Update(ctx, m)
		}
		_ = k8sClient.Delete(ctx, m)
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns}}
		_ = k8sClient.Delete(ctx, s)
		inst := &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}}
		_ = k8sClient.Delete(ctx, inst)
	})

	It("forwards routing params, model_info metadata, and flattened health-check fields", func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: modelName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				ModelName:   "gpt-4o",
				LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
					Model:          "openai/gpt-4o",
					Weight:         intptr(3),
					Order:          intptr(1),
					MaxInputTokens: intptr(120000),
					Temperature:    floatptr(0.2),
					TopP:           floatptr(0.9),
					MaxTokens:      intptr(4096),
					Seed:           intptr(42),
					Organization:   "org-abc",
					AWSRegionName:  "us-east-1",
					ExtraHeaders:   map[string]string{"anthropic-beta": "prompt-caching-2024"},
				},
				ModelInfo: &litellmv1alpha1.ModelInfo{
					Mode:                        "chat",
					BaseModel:                   "azure/gpt-4o",
					Tier:                        "paid",
					RegionName:                  "us-east-1",
					AccessGroups:                []string{"beta"},
					SupportedEnvironments:       []string{"production"},
					UseInPassThrough:            boolptr(true),
					InputCostPerPixel:           floatptr(0.00001),
					InputCostPerSecond:          floatptr(0.0002),
					CacheReadInputTokenCost:     floatptr(0.0000003),
					CacheCreationInputTokenCost: floatptr(0.000004),
					HealthCheck: &litellmv1alpha1.ModelHealthCheck{
						DisableBackgroundHealthCheck: boolptr(true),
						TimeoutSeconds:               intptr(10),
						MaxTokens:                    intptr(5),
						MaxTokensReasoning:           intptr(128),
						MaxTokensNonReasoning:        intptr(1),
						ReasoningEffort:              "none",
						Voice:                        "alloy",
						Model:                        "openai/gpt-4o-mini",
					},
				},
			},
		})).To(Succeed())

		var captured litellm.ModelCreateRequest
		mock := litellm.NewMockClient()
		mock.MockModels.CreateFunc = func(_ context.Context, req litellm.ModelCreateRequest) (*litellm.ModelCreateResponse, error) {
			captured = req
			return &litellm.ModelCreateResponse{ModelID: "m-fields"}, nil
		}
		r := &LiteLLMModelReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: modelKey})
		Expect(err).NotTo(HaveOccurred())

		p := captured.LiteLLMParams
		Expect(p.Weight).To(Equal(intptr(3)))
		Expect(p.Order).To(Equal(intptr(1)))
		Expect(p.MaxInputTokens).To(Equal(intptr(120000)))
		Expect(p.Temperature).To(Equal(floatptr(0.2)))
		Expect(p.TopP).To(Equal(floatptr(0.9)))
		Expect(p.MaxTokens).To(Equal(intptr(4096)))
		Expect(p.Seed).To(Equal(intptr(42)))
		Expect(p.Organization).To(Equal("org-abc"))
		Expect(p.AWSRegionName).To(Equal("us-east-1"))
		Expect(p.ExtraHeaders).To(HaveKeyWithValue("anthropic-beta", "prompt-caching-2024"))

		Expect(captured.ModelInfo).NotTo(BeNil())
		mi := captured.ModelInfo
		Expect(mi.Mode).To(Equal("chat"))
		Expect(mi.BaseModel).To(Equal("azure/gpt-4o"))
		Expect(mi.Tier).To(Equal("paid"))
		Expect(mi.RegionName).To(Equal("us-east-1"))
		Expect(mi.AccessGroups).To(ConsistOf("beta"))
		Expect(mi.SupportedEnvironments).To(ConsistOf("production"))
		Expect(mi.UseInPassThrough).To(Equal(boolptr(true)))
		Expect(mi.InputCostPerPixel).To(Equal(floatptr(0.00001)))
		Expect(mi.InputCostPerSecond).To(Equal(floatptr(0.0002)))
		Expect(mi.CacheReadInputTokenCost).To(Equal(floatptr(0.0000003)))
		Expect(mi.CacheCreationInputTokenCost).To(Equal(floatptr(0.000004)))

		// Health-check fields must be flattened directly onto model_info.
		Expect(mi.DisableBackgroundHealthCheck).To(Equal(boolptr(true)))
		Expect(mi.HealthCheckTimeout).To(Equal(intptr(10)))
		Expect(mi.HealthCheckMaxTokens).To(Equal(intptr(5)))
		Expect(mi.HealthCheckMaxTokensReasoning).To(Equal(intptr(128)))
		Expect(mi.HealthCheckMaxTokensNonReasoning).To(Equal(intptr(1)))
		Expect(mi.HealthCheckReasoningEffort).To(Equal("none"))
		Expect(mi.HealthCheckVoice).To(Equal("alloy"))
		Expect(mi.HealthCheckModel).To(Equal("openai/gpt-4o-mini"))
	})
})

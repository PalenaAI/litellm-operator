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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

var _ = Describe("ConfigSync Controller", func() {
	const (
		instanceName = "cs-test-instance"
		namespace    = "default"
		secretName   = "cs-test-master-key"
	)
	ctx := context.Background()
	instanceNN := types.NamespacedName{Name: instanceName, Namespace: namespace}

	BeforeEach(func() {
		// Create the master key secret.
		secret := &corev1.Secret{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
		if err != nil && errors.IsNotFound(err) {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
				Data:       map[string][]byte{"master-key": []byte("sk-test-master-key")},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		}

		// Create the instance with configSync enabled.
		instance := &litellmv1alpha1.LiteLLMInstance{}
		err = k8sClient.Get(ctx, instanceNN, instance)
		if err != nil && errors.IsNotFound(err) {
			instance = &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: namespace},
				Spec: litellmv1alpha1.LiteLLMInstanceSpec{
					MasterKey: litellmv1alpha1.MasterKeySpec{
						SecretRef: &litellmv1alpha1.SecretKeyRef{
							Name: secretName,
							Key:  "master-key",
						},
					},
					ConfigSync: &litellmv1alpha1.ConfigSyncSpec{
						Enabled:                 true,
						Interval:                "30s",
						UnmanagedResourcePolicy: "preserve",
						ConflictResolution:      "crd-wins",
					},
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			// Set instance as ready so config sync proceeds.
			instance.Status.Ready = true
			instance.Status.Endpoint = "http://litellm:4000"
			Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())
		}
	})

	AfterEach(func() {
		// Clean up models — remove finalizers first to avoid stuck terminating objects.
		var models litellmv1alpha1.LiteLLMModelList
		_ = k8sClient.List(ctx, &models)
		for i := range models.Items {
			m := &models.Items[i]
			if len(m.Finalizers) > 0 {
				m.Finalizers = nil
				_ = k8sClient.Update(ctx, m)
			}
			_ = k8sClient.Delete(ctx, m)
		}
		// Clean up instance — remove finalizers first.
		instance := &litellmv1alpha1.LiteLLMInstance{}
		if err := k8sClient.Get(ctx, instanceNN, instance); err == nil {
			if len(instance.Finalizers) > 0 {
				instance.Finalizers = nil
				_ = k8sClient.Update(ctx, instance)
			}
			_ = k8sClient.Delete(ctx, instance)
		}
		// Clean up secret.
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret); err == nil {
			_ = k8sClient.Delete(ctx, secret)
		}
	})

	Context("When config sync is disabled", func() {
		It("should skip reconciliation", func() {
			// Update instance to disable config sync.
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			instance.Spec.ConfigSync = nil
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return litellm.NewMockClient() },
			}
			result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})

	Context("When config sync is enabled", func() {
		It("should reconcile with no errors and empty API", func() {
			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return litellm.NewMockClient() },
			}
			result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(30 * time.Second))

			// Verify status was updated.
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			Expect(instance.Status.ConfigSync).NotTo(BeNil())
			Expect(instance.Status.ConfigSync.LastSyncTime).NotTo(BeNil())
			Expect(instance.Status.ConfigSync.SyncErrors).To(BeEmpty())
		})

		It("should count managed models correctly", func() {
			// Create a model CRD referencing this instance.
			model := &litellmv1alpha1.LiteLLMModel{
				ObjectMeta: metav1.ObjectMeta{Name: "cs-managed-model", Namespace: namespace},
				Spec: litellmv1alpha1.LiteLLMModelSpec{
					InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
					ModelName:   "gpt-4",
					LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
						Model: "openai/gpt-4",
					},
				},
			}
			Expect(k8sClient.Create(ctx, model)).To(Succeed())

			// Set the model as synced with an API ID.
			model.Status.LiteLLMModelID = "model-123"
			model.Status.Synced = true
			Expect(k8sClient.Status().Update(ctx, model)).To(Succeed())

			// Mock the API to return this model.
			mockClient := litellm.NewMockClient()
			mockClient.MockModels.ListFunc = func(ctx context.Context) ([]litellm.ModelInfoResponse, error) {
				return []litellm.ModelInfoResponse{
					{ModelID: "model-123", ModelName: "gpt-4", Params: litellm.ModelParams{Model: "openai/gpt-4"}},
				}, nil
			}

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return mockClient },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())

			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			Expect(instance.Status.ConfigSync).NotTo(BeNil())
			Expect(instance.Status.ConfigSync.SyncedModels).To(Equal(1))
			Expect(instance.Status.ConfigSync.UnmanagedModels).To(Equal(0))
		})

		It("should count unmanaged models correctly", func() {
			// No model CRDs, but the API returns a model.
			mockClient := litellm.NewMockClient()
			mockClient.MockModels.ListFunc = func(ctx context.Context) ([]litellm.ModelInfoResponse, error) {
				return []litellm.ModelInfoResponse{
					{ModelID: "unmanaged-1", ModelName: "claude-3", Params: litellm.ModelParams{Model: "anthropic/claude-3"}},
				}, nil
			}

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return mockClient },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())

			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			Expect(instance.Status.ConfigSync.SyncedModels).To(Equal(0))
			Expect(instance.Status.ConfigSync.UnmanagedModels).To(Equal(1))
		})

		It("should detect model drift and clear sync hash for crd-wins", func() {
			// Create a model CRD.
			model := &litellmv1alpha1.LiteLLMModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "cs-drift-model",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationSyncHash: "old-hash"},
				},
				Spec: litellmv1alpha1.LiteLLMModelSpec{
					InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
					ModelName:   "gpt-4",
					LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
						Model: "openai/gpt-4",
					},
				},
			}
			Expect(k8sClient.Create(ctx, model)).To(Succeed())
			model.Status.LiteLLMModelID = "drift-model-123"
			model.Status.Synced = true
			Expect(k8sClient.Status().Update(ctx, model)).To(Succeed())

			// API returns a different model name (drift).
			mockClient := litellm.NewMockClient()
			mockClient.MockModels.ListFunc = func(ctx context.Context) ([]litellm.ModelInfoResponse, error) {
				return []litellm.ModelInfoResponse{
					{ModelID: "drift-model-123", ModelName: "gpt-4-renamed", Params: litellm.ModelParams{Model: "openai/gpt-4"}},
				}, nil
			}

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return mockClient },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())

			// The sync hash should have been cleared.
			updatedModel := &litellmv1alpha1.LiteLLMModel{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cs-drift-model", Namespace: namespace}, updatedModel)).To(Succeed())
			Expect(updatedModel.Annotations[AnnotationSyncHash]).To(BeEmpty())
		})

		It("should prune unmanaged models when policy is prune", func() {
			// Set unmanaged policy to prune.
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			instance.Spec.ConfigSync.UnmanagedResourcePolicy = "prune"
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())

			deleted := false
			mockClient := litellm.NewMockClient()
			mockClient.MockModels.ListFunc = func(ctx context.Context) ([]litellm.ModelInfoResponse, error) {
				return []litellm.ModelInfoResponse{
					{ModelID: "prune-me", ModelName: "old-model"},
				}, nil
			}
			mockClient.MockModels.DeleteFunc = func(ctx context.Context, modelID string) error {
				if modelID == "prune-me" {
					deleted = true
				}
				return nil
			}

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return mockClient },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())
		})

		It("should handle model deleted from API in crd-wins mode", func() {
			// Create a model CRD that thinks it's synced, but API has no such model.
			model := &litellmv1alpha1.LiteLLMModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "cs-deleted-api-model",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationSyncHash: "some-hash"},
				},
				Spec: litellmv1alpha1.LiteLLMModelSpec{
					InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
					ModelName:   "gpt-4",
					LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{
						Model: "openai/gpt-4",
					},
				},
			}
			Expect(k8sClient.Create(ctx, model)).To(Succeed())
			model.Status.LiteLLMModelID = "gone-from-api"
			model.Status.Synced = true
			Expect(k8sClient.Status().Update(ctx, model)).To(Succeed())

			// API returns empty list.
			mockClient := litellm.NewMockClient()
			mockClient.MockModels.ListFunc = func(ctx context.Context) ([]litellm.ModelInfoResponse, error) {
				return nil, nil
			}

			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client { return mockClient },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())

			// Status ID should be cleared so the per-resource controller recreates.
			updatedModel := &litellmv1alpha1.LiteLLMModel{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cs-deleted-api-model", Namespace: namespace}, updatedModel)).To(Succeed())
			Expect(updatedModel.Status.LiteLLMModelID).To(BeEmpty())
			Expect(updatedModel.Status.Synced).To(BeFalse())
			Expect(updatedModel.Annotations[AnnotationSyncHash]).To(BeEmpty())
		})
	})
})

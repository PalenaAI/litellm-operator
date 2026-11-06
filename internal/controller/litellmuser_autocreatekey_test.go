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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLM's /user/new mints a personal key unless auto_create_key is false, and
// that key is invisible to the operator, so it must be asked for explicitly.
var _ = Describe("LiteLLMUser autoCreateKey", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "autokey-instance"
		userName     = "autokey-user"
	)
	userKey := types.NamespacedName{Name: userName, Namespace: ns}

	BeforeEach(func() {
		instance := &litellmv1alpha1.LiteLLMInstance{
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
		instance.Status.Ready = true
		Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns},
			Data:       map[string][]byte{"master-key": []byte("sk-master")},
		})).To(Succeed())
	})

	AfterEach(func() {
		user := &litellmv1alpha1.LiteLLMUser{}
		if err := k8sClient.Get(ctx, userKey, user); err == nil {
			user.Finalizers = nil
			_ = k8sClient.Update(ctx, user)
			_ = k8sClient.Delete(ctx, user)
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns}})
		_ = k8sClient.Delete(ctx, &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}})
	})

	createUser := func(autoCreateKey *bool) {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMUser{
			ObjectMeta: metav1.ObjectMeta{Name: userName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMUserSpec{
				InstanceRef:   litellmv1alpha1.InstanceRef{Name: instanceName},
				UserID:        "autokey@example.com",
				AutoCreateKey: autoCreateKey,
			},
		})).To(Succeed())
	}

	reconcileWith := func(mock *litellm.MockClient) {
		GinkgoHelper()
		r := &LiteLLMUserReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).NotTo(HaveOccurred())
	}

	// createdWith reconciles a new user and returns the request /user/new got.
	createdWith := func(autoCreateKey *bool) litellm.UserCreateRequest {
		GinkgoHelper()
		createUser(autoCreateKey)
		var got litellm.UserCreateRequest
		mock := litellm.NewMockClient()
		mock.MockUsers.CreateFunc = func(_ context.Context, req litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
			got = req
			return &litellm.UserCreateResponse{UserID: req.UserID}, nil
		}
		reconcileWith(mock)
		return got
	}

	It("asks LiteLLM not to mint a key when omitted", func() {
		got := createdWith(nil)
		Expect(got.AutoCreateKey).NotTo(BeNil())
		Expect(*got.AutoCreateKey).To(BeFalse())
	})

	It("asks LiteLLM not to mint a key when false", func() {
		autoCreateKey := false
		got := createdWith(&autoCreateKey)
		Expect(got.AutoCreateKey).NotTo(BeNil())
		Expect(*got.AutoCreateKey).To(BeFalse())
	})

	It("lets LiteLLM mint a key when true", func() {
		autoCreateKey := true
		got := createdWith(&autoCreateKey)
		Expect(got.AutoCreateKey).NotTo(BeNil())
		Expect(*got.AutoCreateKey).To(BeTrue())
	})

	// The field must not change the hash of a CR that does not set it, or every
	// existing user would get a /user/update when the operator is upgraded.
	It("does not re-push a user synced before the field existed", func() {
		createUser(nil)
		user := &litellmv1alpha1.LiteLLMUser{}
		Expect(k8sClient.Get(ctx, userKey, user)).To(Succeed())
		Expect(user.Spec.AutoCreateKey).To(BeNil())

		reconcileWith(litellm.NewMockClient())

		updated := false
		mock := litellm.NewMockClient()
		mock.MockUsers.UpdateFunc = func(context.Context, litellm.UserCreateRequest) error {
			updated = true
			return nil
		}
		reconcileWith(mock)
		Expect(updated).To(BeFalse())

		Expect(k8sClient.Get(ctx, userKey, user)).To(Succeed())
		Expect(user.Spec.AutoCreateKey).To(BeNil())
		spec := user.Spec.DeepCopy()
		spec.AutoCreateKey = nil
		Expect(user.Annotations[AnnotationSyncHash]).To(Equal(computeSpecHash(*spec)))
	})
})

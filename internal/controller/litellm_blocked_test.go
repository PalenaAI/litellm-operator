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

// These tests guard the cross-CRD `blocked` field (Team/User/VirtualKey) and
// the Team `teamMemberBudget` field, verifying each reaches the LiteLLM API
// request payload.
var _ = Describe("Blocked + team-member-budget passthrough", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "blk-instance"
	)

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
		for _, obj := range []client.Object{
			&litellmv1alpha1.LiteLLMTeam{ObjectMeta: metav1.ObjectMeta{Name: "blk-team", Namespace: ns}},
			&litellmv1alpha1.LiteLLMUser{ObjectMeta: metav1.ObjectMeta{Name: "blk-user", Namespace: ns}},
			&litellmv1alpha1.LiteLLMVirtualKey{ObjectMeta: metav1.ObjectMeta{Name: "blk-key", Namespace: ns}},
		} {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			if len(obj.GetFinalizers()) > 0 {
				obj.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, obj)
			}
			_ = k8sClient.Delete(ctx, obj)
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: ns}})
		_ = k8sClient.Delete(ctx, &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}})
	})

	It("Team: blocked and teamMemberBudget reach /team/new", func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMTeam{
			ObjectMeta: metav1.ObjectMeta{Name: "blk-team", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMTeamSpec{
				InstanceRef:      litellmv1alpha1.InstanceRef{Name: instanceName},
				TeamAlias:        "blk-team",
				Blocked:          boolptr(true),
				TeamMemberBudget: floatptr(25),
				SoftBudget:       floatptr(80),
				ModelRPMLimit:    map[string]int{"gpt-4o": 100},
				ModelTPMLimit:    map[string]int{"gpt-4o": 50000},
				ObjectPermission: &litellmv1alpha1.ObjectPermission{
					MCPServers:   []string{"github-mcp"},
					VectorStores: []string{"kb-prod"},
				},
			},
		})).To(Succeed())

		var captured litellm.TeamCreateRequest
		mock := litellm.NewMockClient()
		mock.MockTeams.CreateFunc = func(_ context.Context, req litellm.TeamCreateRequest) (*litellm.TeamCreateResponse, error) {
			captured = req
			return &litellm.TeamCreateResponse{TeamID: "t-blk"}, nil
		}
		r := &LiteLLMTeamReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "blk-team", Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())
		Expect(captured.Blocked).To(Equal(boolptr(true)))
		Expect(captured.TeamMemberBudget).To(Equal(floatptr(25)))
		Expect(captured.SoftBudget).To(Equal(floatptr(80)))
		Expect(captured.ModelRPMLimit).To(HaveKeyWithValue("gpt-4o", 100))
		Expect(captured.ModelTPMLimit).To(HaveKeyWithValue("gpt-4o", 50000))
		Expect(captured.ObjectPermission).NotTo(BeNil())
		Expect(captured.ObjectPermission.MCPServers).To(ConsistOf("github-mcp"))
		Expect(captured.ObjectPermission.VectorStores).To(ConsistOf("kb-prod"))
	})

	It("User: blocked reaches /user/new", func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMUser{
			ObjectMeta: metav1.ObjectMeta{Name: "blk-user", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMUserSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				UserID:      "blk-user",
				Blocked:     boolptr(true),
			},
		})).To(Succeed())

		var captured litellm.UserCreateRequest
		mock := litellm.NewMockClient()
		mock.MockUsers.CreateFunc = func(_ context.Context, req litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
			captured = req
			return &litellm.UserCreateResponse{UserID: "blk-user"}, nil
		}
		r := &LiteLLMUserReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "blk-user", Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())
		Expect(captured.Blocked).To(Equal(boolptr(true)))
	})

	It("VirtualKey: blocked reaches /key/generate", func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMVirtualKey{
			ObjectMeta: metav1.ObjectMeta{Name: "blk-key", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				KeyAlias:    "blk-key",
				Blocked:     boolptr(true),
			},
		})).To(Succeed())

		var captured litellm.KeyGenerateRequest
		mock := litellm.NewMockClient()
		mock.MockKeys.GenerateFunc = func(_ context.Context, req litellm.KeyGenerateRequest) (*litellm.KeyGenerateResponse, error) {
			captured = req
			return &litellm.KeyGenerateResponse{Key: "sk-blk", Token: "tok-blk"}, nil
		}
		r := &LiteLLMVirtualKeyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "blk-key", Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())
		Expect(captured.Blocked).To(Equal(boolptr(true)))
	})

	// Regression: a VirtualKey that the proxy rejects for lack of a LiteLLM
	// Enterprise license must REQUEUE (self-heal once the license lands), not give
	// up. Re-applying an unchanged spec produces no reconcile event, so a no-requeue
	// return would leave the key — and anything waiting on its minted Secret, e.g.
	// an auto-wired ChatUI — stuck until the CR is manually recreated.
	It("VirtualKey: requeues when the Enterprise license is missing (no permanent give-up)", func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMVirtualKey{
			ObjectMeta: metav1.ObjectMeta{Name: "blk-key", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				KeyAlias:    "blk-key",
			},
		})).To(Succeed())

		mock := litellm.NewMockClient()
		mock.MockKeys.GenerateFunc = func(_ context.Context, _ litellm.KeyGenerateRequest) (*litellm.KeyGenerateResponse, error) {
			// The proxy rejects the mint with a 403 because no Enterprise license is
			// installed (isEnterpriseLicenseError matches 403 + "enterprise").
			return nil, &litellm.APIError{StatusCode: 403, Message: "Virtual Keys is an enterprise feature", Path: "/key/generate"}
		}
		r := &LiteLLMVirtualKeyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}

		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "blk-key", Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())                                  // no error → no backoff churn…
		Expect(res.RequeueAfter).To(Equal(enterpriseLicenseRetryInterval)) // …but a positive requeue so it retries

		var got litellmv1alpha1.LiteLLMVirtualKey
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "blk-key", Namespace: ns}, &got)).To(Succeed())
		cond := meta.FindStatusCondition(got.Status.Conditions, ConditionSynced)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Reason).To(Equal("EnterpriseLicenseRequired"))
	})
})

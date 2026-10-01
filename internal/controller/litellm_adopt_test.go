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

// These tests cover adopting users and teams that already exist on the proxy
// (created through the Admin UI, SSO, or before the operator was installed).
//
// Without adoption a CR for a pre-existing user loops forever on a 409 from
// /user/new and never records an ID, and a CR for a pre-existing team silently
// creates a second team with the same alias. Adopted objects are never deleted
// upstream when their CR goes away: the operator did not create them, and the
// keys and spend history attached to them cannot be recreated.
var _ = Describe("Adopting pre-existing users and teams", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "adopt-instance"
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
			&litellmv1alpha1.LiteLLMUser{ObjectMeta: metav1.ObjectMeta{Name: "adopt-user", Namespace: ns}},
			&litellmv1alpha1.LiteLLMTeam{ObjectMeta: metav1.ObjectMeta{Name: "adopt-team", Namespace: ns}},
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

	conflict := func(path, msg string) error {
		return &litellm.APIError{StatusCode: 409, Message: msg, Path: path}
	}
	notFound := func(path, msg string) error {
		return &litellm.APIError{StatusCode: 404, Message: msg, Path: path}
	}

	userKey := types.NamespacedName{Name: "adopt-user", Namespace: ns}

	createUser := func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMUser{
			ObjectMeta: metav1.ObjectMeta{Name: "adopt-user", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMUserSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				UserID:      "u-existing",
				UserEmail:   "existing@example.com",
				UserRole:    "internal_user_viewer",
			},
		})).To(Succeed())
	}

	userReconciler := func(mock *litellm.MockClient) *LiteLLMUserReconciler {
		return &LiteLLMUserReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
	}

	It("User: adopts the existing user when /user/new returns 409 for spec.userId", func() {
		createUser()

		var updated *litellm.UserCreateRequest
		mock := litellm.NewMockClient()
		mock.MockUsers.CreateFunc = func(context.Context, litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
			return nil, conflict("/user/new", "User with id u-existing already exists")
		}
		mock.MockUsers.GetFunc = func(_ context.Context, id string) (*litellm.UserInfo, error) {
			return &litellm.UserInfo{UserID: id}, nil
		}
		mock.MockUsers.UpdateFunc = func(_ context.Context, req litellm.UserCreateRequest) error {
			updated = &req
			return nil
		}

		_, err := userReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).NotTo(HaveOccurred())

		var got litellmv1alpha1.LiteLLMUser
		Expect(k8sClient.Get(ctx, userKey, &got)).To(Succeed())
		Expect(got.Status.LiteLLMUserID).To(Equal("u-existing"))
		Expect(got.Annotations).To(HaveKeyWithValue(AnnotationAdopted, AnnotationAdoptedValue))
		Expect(updated).NotTo(BeNil(), "the adopted user must be brought to the CR's spec")
		Expect(updated.UserID).To(Equal("u-existing"))
		Expect(updated.UserRole).To(Equal("internal_user_viewer"))
	})

	It("User: does not adopt when the 409 is not about spec.userId (e.g. email owned by another user)", func() {
		createUser()

		mock := litellm.NewMockClient()
		mock.MockUsers.CreateFunc = func(context.Context, litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
			return nil, conflict("/user/new", "User with email existing@example.com already exists")
		}
		mock.MockUsers.GetFunc = func(_ context.Context, id string) (*litellm.UserInfo, error) {
			return nil, notFound("/user/info", "User "+id+" not found")
		}
		mock.MockUsers.UpdateFunc = func(context.Context, litellm.UserCreateRequest) error {
			Fail("must not update a user it could not confirm exists under spec.userId")
			return nil
		}

		_, err := userReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).To(HaveOccurred())

		var got litellmv1alpha1.LiteLLMUser
		Expect(k8sClient.Get(ctx, userKey, &got)).To(Succeed())
		Expect(got.Status.LiteLLMUserID).To(BeEmpty())
		Expect(got.Annotations).NotTo(HaveKey(AnnotationAdopted))
	})

	It("User: does not adopt when the existence check fails for another reason", func() {
		createUser()

		mock := litellm.NewMockClient()
		mock.MockUsers.CreateFunc = func(context.Context, litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
			return nil, conflict("/user/new", "User with id u-existing already exists")
		}
		mock.MockUsers.GetFunc = func(context.Context, string) (*litellm.UserInfo, error) {
			return nil, &litellm.APIError{StatusCode: 503, Message: "unavailable", Path: "/user/info"}
		}
		mock.MockUsers.UpdateFunc = func(context.Context, litellm.UserCreateRequest) error {
			Fail("must not update a user it could not confirm exists under spec.userId")
			return nil
		}

		_, err := userReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).To(HaveOccurred())

		var got litellmv1alpha1.LiteLLMUser
		Expect(k8sClient.Get(ctx, userKey, &got)).To(Succeed())
		Expect(got.Status.LiteLLMUserID).To(BeEmpty())
		Expect(got.Annotations).NotTo(HaveKey(AnnotationAdopted))
	})

	It("User: deleting the CR of an adopted user leaves the user on the proxy", func() {
		createUser()

		deleted := false
		mock := litellm.NewMockClient()
		mock.MockUsers.CreateFunc = func(context.Context, litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
			return nil, conflict("/user/new", "User with id u-existing already exists")
		}
		mock.MockUsers.DeleteFunc = func(context.Context, string) error {
			deleted = true
			return nil
		}
		r := userReconciler(mock)

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).NotTo(HaveOccurred())

		var got litellmv1alpha1.LiteLLMUser
		Expect(k8sClient.Get(ctx, userKey, &got)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &got)).To(Succeed())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).NotTo(HaveOccurred())

		Expect(deleted).To(BeFalse(), "an adopted user was not created by the operator and must not be deleted by it")
		Expect(errors.IsNotFound(k8sClient.Get(ctx, userKey, &got))).To(BeTrue(), "the finalizer must still be released")
	})

	teamKey := types.NamespacedName{Name: "adopt-team", Namespace: ns}

	createTeam := func(teamID string) {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMTeam{
			ObjectMeta: metav1.ObjectMeta{Name: "adopt-team", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMTeamSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName},
				TeamID:      teamID,
				TeamAlias:   "company",
				Models:      []string{"gpt-4o"},
			},
		})).To(Succeed())
	}

	teamReconciler := func(mock *litellm.MockClient) *LiteLLMTeamReconciler {
		return &LiteLLMTeamReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
	}

	It("Team: adopts the existing team named by spec.teamId instead of creating a duplicate", func() {
		createTeam("t-existing")

		var updated *litellm.TeamUpdateRequest
		mock := litellm.NewMockClient()
		mock.MockTeams.CreateFunc = func(context.Context, litellm.TeamCreateRequest) (*litellm.TeamCreateResponse, error) {
			Fail("must not create a team when spec.teamId already exists on the proxy")
			return nil, nil
		}
		mock.MockTeams.GetFunc = func(_ context.Context, id string) (*litellm.TeamInfo, error) {
			return &litellm.TeamInfo{TeamID: id, TeamAlias: "company"}, nil
		}
		mock.MockTeams.UpdateFunc = func(_ context.Context, req litellm.TeamUpdateRequest) error {
			updated = &req
			return nil
		}

		_, err := teamReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: teamKey})
		Expect(err).NotTo(HaveOccurred())

		var got litellmv1alpha1.LiteLLMTeam
		Expect(k8sClient.Get(ctx, teamKey, &got)).To(Succeed())
		Expect(got.Status.LiteLLMTeamID).To(Equal("t-existing"))
		Expect(got.Annotations).To(HaveKeyWithValue(AnnotationAdopted, AnnotationAdoptedValue))
		Expect(updated).NotTo(BeNil(), "the adopted team must be brought to the CR's spec")
		Expect(updated.TeamID).To(Equal("t-existing"))
		Expect(updated.Models).To(ConsistOf("gpt-4o"))
	})

	It("Team: creates the team under spec.teamId when no team has that id", func() {
		createTeam("t-new")

		var created *litellm.TeamCreateRequest
		mock := litellm.NewMockClient()
		mock.MockTeams.GetFunc = func(_ context.Context, id string) (*litellm.TeamInfo, error) {
			if created == nil {
				return nil, notFound("/team/info", "Team not found, passed team id: "+id)
			}
			return &litellm.TeamInfo{TeamID: id}, nil
		}
		mock.MockTeams.CreateFunc = func(_ context.Context, req litellm.TeamCreateRequest) (*litellm.TeamCreateResponse, error) {
			created = &req
			return &litellm.TeamCreateResponse{TeamID: req.TeamID}, nil
		}

		_, err := teamReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: teamKey})
		Expect(err).NotTo(HaveOccurred())

		Expect(created).NotTo(BeNil())
		Expect(created.TeamID).To(Equal("t-new"))
		var got litellmv1alpha1.LiteLLMTeam
		Expect(k8sClient.Get(ctx, teamKey, &got)).To(Succeed())
		Expect(got.Status.LiteLLMTeamID).To(Equal("t-new"))
		Expect(got.Annotations).NotTo(HaveKey(AnnotationAdopted))
	})

	It("Team: neither creates nor adopts when the existence check fails for another reason", func() {
		createTeam("t-unknown")

		mock := litellm.NewMockClient()
		mock.MockTeams.GetFunc = func(context.Context, string) (*litellm.TeamInfo, error) {
			return nil, &litellm.APIError{StatusCode: 503, Message: "unavailable", Path: "/team/info"}
		}
		mock.MockTeams.CreateFunc = func(context.Context, litellm.TeamCreateRequest) (*litellm.TeamCreateResponse, error) {
			Fail("must not create a team while it cannot tell whether spec.teamId exists")
			return nil, nil
		}

		_, err := teamReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: teamKey})
		Expect(err).To(HaveOccurred())

		var got litellmv1alpha1.LiteLLMTeam
		Expect(k8sClient.Get(ctx, teamKey, &got)).To(Succeed())
		Expect(got.Status.LiteLLMTeamID).To(BeEmpty())
	})

	It("Team: deleting the CR of an adopted team leaves the team on the proxy", func() {
		createTeam("t-existing")

		deleted := false
		mock := litellm.NewMockClient()
		mock.MockTeams.GetFunc = func(_ context.Context, id string) (*litellm.TeamInfo, error) {
			return &litellm.TeamInfo{TeamID: id}, nil
		}
		mock.MockTeams.DeleteFunc = func(context.Context, string) error {
			deleted = true
			return nil
		}
		r := teamReconciler(mock)

		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: teamKey})
		Expect(err).NotTo(HaveOccurred())

		var got litellmv1alpha1.LiteLLMTeam
		Expect(k8sClient.Get(ctx, teamKey, &got)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &got)).To(Succeed())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: teamKey})
		Expect(err).NotTo(HaveOccurred())

		Expect(deleted).To(BeFalse(), "an adopted team was not created by the operator and must not be deleted by it")
		Expect(errors.IsNotFound(k8sClient.Get(ctx, teamKey, &got))).To(BeTrue(), "the finalizer must still be released")
	})
})

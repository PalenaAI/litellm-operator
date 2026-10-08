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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// These tests cover spec.initialPasswordSecretRef. The password is an
// *initial* one: users are expected to change it in the Admin UI afterwards,
// so the operator must apply it once and then only when the Secret's value
// changes — never on every resync, which would silently undo the user's own
// password change.
var _ = Describe("LiteLLMUser initial password", func() {
	ctx := context.Background()
	const (
		ns           = "default"
		instanceName = "pw-instance"
		userName     = "pw-user"
		secretName   = "pw-user-password"
	)
	userKey := types.NamespacedName{Name: userName, Namespace: ns}

	BeforeEach(func() {
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
	})

	AfterEach(func() {
		user := &litellmv1alpha1.LiteLLMUser{ObjectMeta: metav1.ObjectMeta{Name: userName, Namespace: ns}}
		_ = k8sClient.Get(ctx, userKey, user)
		if len(user.Finalizers) > 0 {
			user.Finalizers = nil
			_ = k8sClient.Update(ctx, user)
		}
		_ = k8sClient.Delete(ctx, user)
		for _, name := range []string{secretName, instanceName + "-master-key"} {
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}})
		}
		_ = k8sClient.Delete(ctx, &litellmv1alpha1.LiteLLMInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns}})
	})

	createSecret := func(password string) {
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns},
			Data:       map[string][]byte{"password": []byte(password)},
		})).To(Succeed())
	}
	setSecret := func(password string) {
		var s corev1.Secret
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, &s)).To(Succeed())
		s.Data["password"] = []byte(password)
		Expect(k8sClient.Update(ctx, &s)).To(Succeed())
	}
	createUser := func() {
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMUser{
			ObjectMeta: metav1.ObjectMeta{Name: userName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMUserSpec{
				InstanceRef:              litellmv1alpha1.InstanceRef{Name: instanceName},
				UserID:                   "pw-user-id",
				UserEmail:                "pw@example.com",
				InitialPasswordSecretRef: &litellmv1alpha1.SecretKeyRef{Name: secretName, Key: "password"},
			},
		})).To(Succeed())
	}

	type setPasswordCall struct{ userID, password string }
	newMock := func(calls *[]setPasswordCall) *litellm.MockClient {
		mock := litellm.NewMockClient()
		mock.MockUsers.SetPasswordFunc = func(_ context.Context, userID, password string) error {
			*calls = append(*calls, setPasswordCall{userID, password})
			return nil
		}
		return mock
	}
	reconcileWith := func(mock *litellm.MockClient) {
		r := &LiteLLMUserReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: userKey})
		Expect(err).NotTo(HaveOccurred())
	}
	getUser := func() *litellmv1alpha1.LiteLLMUser {
		var u litellmv1alpha1.LiteLLMUser
		Expect(k8sClient.Get(ctx, userKey, &u)).To(Succeed())
		return &u
	}

	It("applies the password once after creating the user and records only a digest", func() {
		createSecret("Initial-Pa55word!")
		createUser()
		var calls []setPasswordCall
		reconcileWith(newMock(&calls))

		Expect(calls).To(Equal([]setPasswordCall{{"pw-user-id", "Initial-Pa55word!"}}))
		u := getUser()
		Expect(u.Status.LiteLLMUserID).To(Equal("pw-user-id"))
		Expect(u.Status.InitialPasswordDigest).NotTo(BeEmpty())
		Expect(u.Status.InitialPasswordDigest).NotTo(ContainSubstring("Initial-Pa55word!"))
		Expect(passwordMatchesDigest(u.Status.InitialPasswordDigest, "Initial-Pa55word!")).To(BeTrue())
		cond := meta.FindStatusCondition(u.Status.Conditions, ConditionInitialPasswordApplied)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	})

	It("does not re-apply an unchanged password on later reconciles", func() {
		createSecret("Initial-Pa55word!")
		createUser()
		var calls []setPasswordCall
		mock := newMock(&calls)
		reconcileWith(mock)
		reconcileWith(mock)
		reconcileWith(mock)
		Expect(calls).To(HaveLen(1))
	})

	It("re-applies the password when the Secret's value changes", func() {
		createSecret("Initial-Pa55word!")
		createUser()
		var calls []setPasswordCall
		mock := newMock(&calls)
		reconcileWith(mock)

		setSecret("Rotated-Pa55word!")
		reconcileWith(mock)

		Expect(calls).To(HaveLen(2))
		Expect(calls[1].password).To(Equal("Rotated-Pa55word!"))
		Expect(passwordMatchesDigest(getUser().Status.InitialPasswordDigest, "Rotated-Pa55word!")).To(BeTrue())
	})

	It("still syncs the user when the Secret is missing, and applies the password once it appears", func() {
		createUser()
		var calls []setPasswordCall
		mock := newMock(&calls)
		reconcileWith(mock)

		u := getUser()
		Expect(u.Status.LiteLLMUserID).To(Equal("pw-user-id"))
		Expect(meta.IsStatusConditionTrue(u.Status.Conditions, ConditionSynced)).To(BeTrue())
		cond := meta.FindStatusCondition(u.Status.Conditions, ConditionInitialPasswordApplied)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal("SecretNotFound"))
		Expect(calls).To(BeEmpty())

		createSecret("Initial-Pa55word!")
		reconcileWith(mock)
		Expect(calls).To(HaveLen(1))
	})

	It("reports a rejected password without recording a digest or failing the user sync", func() {
		createSecret("weak")
		createUser()
		mock := litellm.NewMockClient()
		mock.MockUsers.SetPasswordFunc = func(context.Context, string, string) error {
			return &litellm.APIError{StatusCode: 400, Path: "/user/update",
				Message: "Password does not meet the password policy"}
		}
		reconcileWith(mock)

		u := getUser()
		Expect(u.Status.InitialPasswordDigest).To(BeEmpty())
		Expect(meta.IsStatusConditionTrue(u.Status.Conditions, ConditionSynced)).To(BeTrue())
		cond := meta.FindStatusCondition(u.Status.Conditions, ConditionInitialPasswordApplied)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Reason).To(Equal("PasswordRejected"))
	})

	It("forgets the digest when the field is removed, without touching the password", func() {
		createSecret("Initial-Pa55word!")
		createUser()
		var calls []setPasswordCall
		mock := newMock(&calls)
		reconcileWith(mock)

		u := getUser()
		u.Spec.InitialPasswordSecretRef = nil
		Expect(k8sClient.Update(ctx, u)).To(Succeed())
		reconcileWith(mock)

		u = getUser()
		Expect(calls).To(HaveLen(1))
		Expect(u.Status.InitialPasswordDigest).To(BeEmpty())
		Expect(meta.FindStatusCondition(u.Status.Conditions, ConditionInitialPasswordApplied)).To(BeNil())
	})

	It("enqueues only users that reference the changed Secret", func() {
		createSecret("Initial-Pa55word!")
		createUser()
		r := &LiteLLMUserReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

		reqs := r.findUsersForSecret(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}})
		Expect(reqs).To(ConsistOf(reconcile.Request{NamespacedName: userKey}))

		reqs = r.findUsersForSecret(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: ns}})
		Expect(reqs).To(BeEmpty())
	})
})

var _ = Describe("password digest", func() {
	It("distinguishes passwords longer than bcrypt's 72-byte limit", func() {
		long := strings.Repeat("a", 80)
		digest, err := passwordDigest(long + "1")
		Expect(err).NotTo(HaveOccurred())
		Expect(passwordMatchesDigest(digest, long+"1")).To(BeTrue())
		Expect(passwordMatchesDigest(digest, long+"2")).To(BeFalse())
	})

	It("never matches an empty digest", func() {
		Expect(passwordMatchesDigest("", "anything")).To(BeFalse())
	})
})

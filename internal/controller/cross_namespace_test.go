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

var _ = Describe("Cross-namespace instanceRef", func() {
	ctx := context.Background()
	const (
		instanceNS   = "xns-gateway"
		tenantNS     = "xns-tenant"
		otherNS      = "xns-other"
		instanceName = "xns-instance"
		keyName      = "xns-key"
		tenantLabel  = "litellm.palena.ai/tenant"
	)
	instanceNN := types.NamespacedName{Name: instanceName, Namespace: instanceNS}
	keyNN := types.NamespacedName{Name: keyName, Namespace: tenantNS}

	ensureNamespace := func(name string, labels map[string]string) {
		GinkgoHelper()
		ns := &corev1.Namespace{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, ns); errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
			})).To(Succeed())
		}
	}

	setAllowed := func(allowed *litellmv1alpha1.AllowedNamespaces) {
		GinkgoHelper()
		instance := &litellmv1alpha1.LiteLLMInstance{}
		Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
		instance.Spec.AllowedNamespaces = allowed
		Expect(k8sClient.Update(ctx, instance)).To(Succeed())
	}

	tenantSelector := &litellmv1alpha1.AllowedNamespaces{
		From:     litellmv1alpha1.NamespacesFromSelector,
		Selector: &metav1.LabelSelector{MatchLabels: map[string]string{tenantLabel: "true"}},
	}

	newKeyReconciler := func(mock *litellm.MockClient) *LiteLLMVirtualKeyReconciler {
		return &LiteLLMVirtualKeyReconciler{
			Client:               k8sClient,
			Scheme:               k8sClient.Scheme(),
			LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
		}
	}

	createKey := func(namespace string) {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMVirtualKey{
			ObjectMeta: metav1.ObjectMeta{Name: keyName, Namespace: namespace},
			Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
				KeyAlias:    keyName,
			},
		})).To(Succeed())
	}

	removeWithFinalizers := func(obj client.Object) {
		obj.SetFinalizers(nil)
		_ = k8sClient.Update(ctx, obj)
		_ = k8sClient.Delete(ctx, obj)
	}

	allowKinds := func(kinds ...litellmv1alpha1.AllowedNamespaceKind) *litellmv1alpha1.AllowedNamespaces {
		return &litellmv1alpha1.AllowedNamespaces{From: tenantSelector.From, Selector: tenantSelector.Selector, Kinds: kinds}
	}

	newTeam := func(name string) *litellmv1alpha1.LiteLLMTeam {
		return &litellmv1alpha1.LiteLLMTeam{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tenantNS},
			Spec: litellmv1alpha1.LiteLLMTeamSpec{
				InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
				TeamAlias:   name,
			},
		}
	}

	syncedCondition := func(obj client.Object, conditions func() []metav1.Condition) *metav1.Condition {
		GinkgoHelper()
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		return meta.FindStatusCondition(conditions(), ConditionSynced)
	}

	BeforeEach(func() {
		ensureNamespace(instanceNS, nil)
		ensureNamespace(tenantNS, map[string]string{tenantLabel: "true"})
		ensureNamespace(otherNS, nil)

		instance := &litellmv1alpha1.LiteLLMInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: instanceNS},
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
		instance.Status.Endpoint = "http://litellm:4000"
		Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())

		// The master key exists only in the instance's namespace, so a key in
		// a tenant namespace can only mint if it is read from there.
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: instanceNS},
			Data:       map[string][]byte{"master-key": []byte("sk-master")},
		})).To(Succeed())
	})

	AfterEach(func() {
		for _, ns := range []string{tenantNS, otherNS} {
			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: ns}, vk); err == nil {
				vk.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, vk)
				_ = k8sClient.Delete(ctx, vk)
			}
			// envtest runs no garbage collector.
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: keyName + "-key", Namespace: ns}})
		}
		var teams litellmv1alpha1.LiteLLMTeamList
		_ = k8sClient.List(ctx, &teams, client.InNamespace(tenantNS))
		for i := range teams.Items {
			removeWithFinalizers(&teams.Items[i])
		}
		var users litellmv1alpha1.LiteLLMUserList
		_ = k8sClient.List(ctx, &users, client.InNamespace(tenantNS))
		for i := range users.Items {
			removeWithFinalizers(&users.Items[i])
		}
		var customers litellmv1alpha1.LiteLLMCustomerList
		_ = k8sClient.List(ctx, &customers, client.InNamespace(tenantNS))
		for i := range customers.Items {
			removeWithFinalizers(&customers.Items[i])
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instanceName + "-master-key", Namespace: instanceNS}})
		instance := &litellmv1alpha1.LiteLLMInstance{}
		if err := k8sClient.Get(ctx, instanceNN, instance); err == nil {
			instance.Finalizers = nil
			_ = k8sClient.Update(ctx, instance)
			_ = k8sClient.Delete(ctx, instance)
		}
	})

	Describe("instanceAllowsNamespace", func() {
		check := func(allowed *litellmv1alpha1.AllowedNamespaces, namespace string) bool {
			GinkgoHelper()
			instance := &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: instanceNS},
				Spec:       litellmv1alpha1.LiteLLMInstanceSpec{AllowedNamespaces: allowed},
			}
			ok, err := instanceAllowsNamespace(ctx, k8sClient, instance, namespace)
			Expect(err).NotTo(HaveOccurred())
			return ok
		}

		It("always allows the instance's own namespace", func() {
			Expect(check(nil, instanceNS)).To(BeTrue())
		})
		It("allows no other namespace by default", func() {
			Expect(check(nil, tenantNS)).To(BeFalse())
			Expect(check(&litellmv1alpha1.AllowedNamespaces{From: litellmv1alpha1.NamespacesFromSame}, tenantNS)).To(BeFalse())
		})
		It("allows every namespace with All", func() {
			Expect(check(&litellmv1alpha1.AllowedNamespaces{From: litellmv1alpha1.NamespacesFromAll}, otherNS)).To(BeTrue())
		})
		It("allows only matching namespaces with Selector", func() {
			Expect(check(tenantSelector, tenantNS)).To(BeTrue())
			Expect(check(tenantSelector, otherNS)).To(BeFalse())
		})
	})

	Describe("a LiteLLMVirtualKey in another namespace", func() {
		It("is refused with RefNotPermitted when the instance does not allow its namespace", func() {
			createKey(otherNS)
			generated := false
			mock := litellm.NewMockClient()
			mock.MockKeys.GenerateFunc = func(context.Context, litellm.KeyGenerateRequest) (*litellm.KeyGenerateResponse, error) {
				generated = true
				return &litellm.KeyGenerateResponse{Key: "sk-x", Token: "t"}, nil
			}

			_, err := newKeyReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: keyName, Namespace: otherNS}})
			Expect(err).NotTo(HaveOccurred())
			Expect(generated).To(BeFalse())

			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName, Namespace: otherNS}, vk)).To(Succeed())
			cond := meta.FindStatusCondition(vk.Status.Conditions, ConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(ReasonRefNotPermitted))
		})

		It("mints its key into a Secret in its own namespace when the instance allows it", func() {
			setAllowed(tenantSelector)
			createKey(tenantNS)

			_, err := newKeyReconciler(litellm.NewMockClient()).Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())

			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			Expect(vk.Status.LiteLLMKeyToken).To(Equal("mock-token-hash"))

			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyName + "-key", Namespace: tenantNS}, secret)).To(Succeed())
			Expect(secret.OwnerReferences).To(HaveLen(1))
			Expect(secret.OwnerReferences[0].Name).To(Equal(keyName))
		})

		It("still deletes its key from LiteLLM after the instance stops allowing it", func() {
			setAllowed(tenantSelector)
			createKey(tenantNS)
			r := newKeyReconciler(litellm.NewMockClient())
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())

			setAllowed(nil)
			deleted := ""
			mock := litellm.NewMockClient()
			mock.MockKeys.DeleteFunc = func(_ context.Context, token string) error {
				deleted = token
				return nil
			}
			r = newKeyReconciler(mock)
			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			Expect(k8sClient.Delete(ctx, vk)).To(Succeed())
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(Equal("mock-token-hash"))
		})
	})

	Describe("allowedNamespaces.kinds", func() {
		instanceWith := func(allowed *litellmv1alpha1.AllowedNamespaces) *litellmv1alpha1.LiteLLMInstance {
			return &litellmv1alpha1.LiteLLMInstance{
				ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: instanceNS},
				Spec:       litellmv1alpha1.LiteLLMInstanceSpec{AllowedNamespaces: allowed},
			}
		}

		It("admits only LiteLLMVirtualKey when omitted", func() {
			instance := instanceWith(tenantSelector)
			Expect(instanceAllowsKind(instance, tenantNS, kindVirtualKey)).To(BeTrue())
			for _, kind := range []string{kindTeam, kindUser, kindOrganization, kindBudget, kindCustomer, kindModel, kindCredential} {
				Expect(instanceAllowsKind(instance, tenantNS, kind)).To(BeFalse(), kind)
			}
		})

		It("replaces the default when set", func() {
			instance := instanceWith(allowKinds(litellmv1alpha1.AllowedKindTeam))
			Expect(instanceAllowsKind(instance, tenantNS, kindTeam)).To(BeTrue())
			Expect(instanceAllowsKind(instance, tenantNS, kindVirtualKey)).To(BeFalse())
		})

		It("never admits LiteLLMModel or LiteLLMCredential from another namespace", func() {
			instance := instanceWith(allowKinds(
				litellmv1alpha1.AllowedKindVirtualKey, litellmv1alpha1.AllowedKindTeam, litellmv1alpha1.AllowedKindUser,
				litellmv1alpha1.AllowedKindOrganization, litellmv1alpha1.AllowedKindBudget, litellmv1alpha1.AllowedKindCustomer,
			))
			Expect(instanceAllowsKind(instance, tenantNS, kindModel)).To(BeFalse())
			Expect(instanceAllowsKind(instance, tenantNS, kindCredential)).To(BeFalse())
		})

		It("does not restrict the instance's own namespace", func() {
			for _, kind := range []string{kindVirtualKey, kindTeam, kindUser, kindModel, kindCredential} {
				Expect(instanceAllowsKind(instanceWith(nil), instanceNS, kind)).To(BeTrue(), kind)
			}
		})

		It("refuses a proxy-admin LiteLLMUser from an allowed namespace by default", func() {
			setAllowed(tenantSelector)
			user := &litellmv1alpha1.LiteLLMUser{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-admin", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMUserSpec{
					InstanceRef:              litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					UserID:                   "admin@example.com",
					UserRole:                 "proxy_admin",
					InitialPasswordSecretRef: &litellmv1alpha1.SecretKeyRef{Name: "pw", Key: "password"},
				},
			}
			Expect(k8sClient.Create(ctx, user)).To(Succeed())

			touched := false
			mock := litellm.NewMockClient()
			mock.MockUsers.CreateFunc = func(context.Context, litellm.UserCreateRequest) (*litellm.UserCreateResponse, error) {
				touched = true
				return &litellm.UserCreateResponse{UserID: "admin@example.com"}, nil
			}
			mock.MockUsers.UpdateFunc = func(context.Context, litellm.UserCreateRequest) error {
				touched = true
				return nil
			}
			mock.MockUsers.SetPasswordFunc = func(context.Context, string, string) error {
				touched = true
				return nil
			}
			r := &LiteLLMUserReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
				LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(user)})
			Expect(err).NotTo(HaveOccurred())
			Expect(touched).To(BeFalse())

			cond := syncedCondition(user, func() []metav1.Condition { return user.Status.Conditions })
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(ReasonRefNotPermitted))
			Expect(cond.Message).To(ContainSubstring("LiteLLMUser"))
		})

		teamReconciler := func(mock *litellm.MockClient) *LiteLLMTeamReconciler {
			return &LiteLLMTeamReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
				LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock }}
		}

		It("refuses to adopt another tenant's team from an allowed namespace by default", func() {
			setAllowed(tenantSelector)
			team := newTeam("xns-takeover")
			team.Spec.TeamID = "another-tenants-team"
			Expect(k8sClient.Create(ctx, team)).To(Succeed())

			touched := false
			mock := litellm.NewMockClient()
			mock.MockTeams.GetFunc = func(context.Context, string) (*litellm.TeamInfo, error) {
				touched = true
				return &litellm.TeamInfo{}, nil
			}
			mock.MockTeams.UpdateFunc = func(context.Context, litellm.TeamUpdateRequest) error {
				touched = true
				return nil
			}
			_, err := teamReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(team)})
			Expect(err).NotTo(HaveOccurred())
			Expect(touched).To(BeFalse())

			cond := syncedCondition(team, func() []metav1.Condition { return team.Status.Conditions })
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(ReasonRefNotPermitted))
		})

		It("reconciles a LiteLLMTeam from an allowed namespace once the kind is listed", func() {
			setAllowed(allowKinds(litellmv1alpha1.AllowedKindTeam))
			team := newTeam("xns-listed")
			Expect(k8sClient.Create(ctx, team)).To(Succeed())

			created := false
			mock := litellm.NewMockClient()
			mock.MockTeams.CreateFunc = func(context.Context, litellm.TeamCreateRequest) (*litellm.TeamCreateResponse, error) {
				created = true
				return &litellm.TeamCreateResponse{TeamID: "t-listed"}, nil
			}
			_, err := teamReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(team)})
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		It("refuses a LiteLLMVirtualKey when kinds is set without it", func() {
			setAllowed(allowKinds(litellmv1alpha1.AllowedKindTeam))
			createKey(tenantNS)
			generated := false
			mock := litellm.NewMockClient()
			mock.MockKeys.GenerateFunc = func(context.Context, litellm.KeyGenerateRequest) (*litellm.KeyGenerateResponse, error) {
				generated = true
				return &litellm.KeyGenerateResponse{Key: "sk-x", Token: "t"}, nil
			}
			_, err := newKeyReconciler(mock).Reconcile(ctx, reconcile.Request{NamespacedName: keyNN})
			Expect(err).NotTo(HaveOccurred())
			Expect(generated).To(BeFalse())
		})

		It("still deletes what a resource created after its kind stops being listed", func() {
			setAllowed(allowKinds(litellmv1alpha1.AllowedKindTeam))
			team := newTeam("xns-unlisted")
			Expect(k8sClient.Create(ctx, team)).To(Succeed())
			mock := litellm.NewMockClient()
			mock.MockTeams.CreateFunc = func(context.Context, litellm.TeamCreateRequest) (*litellm.TeamCreateResponse, error) {
				return &litellm.TeamCreateResponse{TeamID: "t-unlisted"}, nil
			}
			deleted := ""
			mock.MockTeams.DeleteFunc = func(_ context.Context, teamID string) error {
				deleted = teamID
				return nil
			}
			r := teamReconciler(mock)
			req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(team)}
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			setAllowed(tenantSelector)
			Expect(k8sClient.Get(ctx, req.NamespacedName, team)).To(Succeed())
			Expect(k8sClient.Delete(ctx, team)).To(Succeed())
			_, err = r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(Equal("t-unlisted"))
		})
	})

	Describe("config sync", func() {
		enablePrune := func(allowed *litellmv1alpha1.AllowedNamespaces) {
			GinkgoHelper()
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			instance.Spec.AllowedNamespaces = allowed
			instance.Spec.ConfigSync = &litellmv1alpha1.ConfigSyncSpec{
				Enabled:                 true,
				Interval:                "30s",
				UnmanagedResourcePolicy: "prune",
				ConflictResolution:      "crd-wins",
			}
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
		}

		runSync := func(mock *litellm.MockClient) *litellmv1alpha1.LiteLLMInstance {
			GinkgoHelper()
			r := &ConfigSyncReconciler{
				Client:               k8sClient,
				Scheme:               k8sClient.Scheme(),
				LiteLLMClientFactory: func(string, string, ...litellm.ClientOption) litellm.Client { return mock },
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: instanceNN})
			Expect(err).NotTo(HaveOccurred())
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			Expect(instance.Status.ConfigSync).NotTo(BeNil())
			return instance
		}

		// createSyncedKey creates a key in the tenant namespace that the operator
		// has already minted as "xns-token".
		createSyncedKey := func() {
			GinkgoHelper()
			createKey(tenantNS)
			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			vk.Status.LiteLLMKeyToken = "xns-token"
			vk.Status.Synced = true
			Expect(k8sClient.Status().Update(ctx, vk)).To(Succeed())
		}

		keyMock := func(pruned *[]string) *litellm.MockClient {
			mock := litellm.NewMockClient()
			mock.MockKeys.ListFunc = func(context.Context) ([]litellm.KeyInfo, error) {
				return []litellm.KeyInfo{{Token: "xns-token", KeyAlias: keyName}}, nil
			}
			mock.MockKeys.DeleteFunc = func(_ context.Context, token string) error {
				*pruned = append(*pruned, token)
				return nil
			}
			return mock
		}

		It("counts a key in an allowed namespace as managed instead of pruning it", func() {
			enablePrune(tenantSelector)
			createSyncedKey()
			var pruned []string
			instance := runSync(keyMock(&pruned))
			Expect(pruned).To(BeEmpty())
			Expect(instance.Status.ConfigSync.SyncedKeys).To(Equal(1))
		})

		// Setting kinds replaces its default. An admin adding LiteLLMTeam without
		// repeating LiteLLMVirtualKey must not delete every tenant's keys: a
		// deleted key's material cannot be recovered.
		It("does not prune a key when kinds stops admitting LiteLLMVirtualKey", func() {
			enablePrune(allowKinds(litellmv1alpha1.AllowedKindTeam))
			createSyncedKey()
			var pruned []string
			instance := runSync(keyMock(&pruned))
			Expect(pruned).To(BeEmpty())
			Expect(instance.Status.ConfigSync.SyncedKeys).To(Equal(0))
			Expect(instance.Status.ConfigSync.UnmanagedKeys).To(Equal(0))
		})

		It("does not remediate drift on a resource whose kind is not admitted", func() {
			enablePrune(allowKinds(litellmv1alpha1.AllowedKindTeam))
			createSyncedKey()
			vk := &litellmv1alpha1.LiteLLMVirtualKey{}
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			vk.Annotations = map[string]string{AnnotationSyncHash: "keep-me"}
			Expect(k8sClient.Update(ctx, vk)).To(Succeed())

			var pruned []string
			mock := keyMock(&pruned)
			mock.MockKeys.ListFunc = func(context.Context) ([]litellm.KeyInfo, error) {
				return []litellm.KeyInfo{{Token: "xns-token", KeyAlias: "drifted-alias"}}, nil
			}
			runSync(mock)
			Expect(k8sClient.Get(ctx, keyNN, vk)).To(Succeed())
			Expect(vk.Annotations[AnnotationSyncHash]).To(Equal("keep-me"))
		})

		It("still prunes objects no resource claims when a kind is not admitted", func() {
			enablePrune(allowKinds(litellmv1alpha1.AllowedKindTeam))
			createSyncedKey()
			var pruned []string
			mock := keyMock(&pruned)
			mock.MockKeys.ListFunc = func(context.Context) ([]litellm.KeyInfo, error) {
				return []litellm.KeyInfo{{Token: "xns-token", KeyAlias: keyName}, {Token: "stray", KeyAlias: "stray"}}, nil
			}
			runSync(mock)
			Expect(pruned).To(ConsistOf("stray"))
		})

		// Revoking a namespace is the documented way to cut a tenant off.
		It("prunes what a namespace created once the instance stops allowing it", func() {
			enablePrune(nil)
			createSyncedKey()
			var pruned []string
			runSync(keyMock(&pruned))
			Expect(pruned).To(ConsistOf("xns-token"))
		})

		customerMock := func(pruned *[]string) *litellm.MockClient {
			mock := litellm.NewMockClient()
			mock.MockCustomers.ListFunc = func(context.Context) ([]litellm.CustomerInfo, error) {
				return []litellm.CustomerInfo{{UserID: "cust-1"}}, nil
			}
			mock.MockCustomers.DeleteFunc = func(_ context.Context, id string) error {
				*pruned = append(*pruned, id)
				return nil
			}
			return mock
		}

		createCustomer := func() *litellmv1alpha1.LiteLLMCustomer {
			GinkgoHelper()
			customer := &litellmv1alpha1.LiteLLMCustomer{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-customer", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMCustomerSpec{
					InstanceRef: litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					CustomerID:  "cust-1",
				},
			}
			Expect(k8sClient.Create(ctx, customer)).To(Succeed())
			return customer
		}

		It("does not prune a customer the operator synced before its kind stopped being admitted", func() {
			enablePrune(tenantSelector)
			customer := createCustomer()
			customer.Status.Synced = true
			Expect(k8sClient.Status().Update(ctx, customer)).To(Succeed())
			var pruned []string
			runSync(customerMock(&pruned))
			Expect(pruned).To(BeEmpty())
		})

		// customerId is written by the user, so naming a customer in a CR the
		// operator refused must not shield it.
		It("does not let an unsynced customer of an unadmitted kind shield an ID from pruning", func() {
			enablePrune(tenantSelector)
			createCustomer()
			var pruned []string
			runSync(customerMock(&pruned))
			Expect(pruned).To(ConsistOf("cust-1"))
		})
	})

	Describe("validation", func() {
		It("rejects instanceRef.namespace on a LiteLLMGuardrail", func() {
			err := k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMGuardrail{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-guardrail", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
					InstanceRef:   litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					GuardrailName: "xns",
					Provider:      "presidio",
					Mode:          "pre_call",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("instanceRef.namespace is not supported"))
		})

		It("rejects instanceRef.namespace on a LiteLLMModel", func() {
			err := k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMModel{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-model", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMModelSpec{
					InstanceRef:   litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					ModelName:     "gpt-4",
					LiteLLMParams: litellmv1alpha1.LiteLLMModelParams{Model: "openai/gpt-4"},
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("instanceRef.namespace is not supported"))
		})

		It("rejects instanceRef.namespace on a LiteLLMCredential", func() {
			err := k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMCredential{
				ObjectMeta: metav1.ObjectMeta{Name: "xns-credential", Namespace: tenantNS},
				Spec: litellmv1alpha1.LiteLLMCredentialSpec{
					InstanceRef:     litellmv1alpha1.InstanceRef{Name: instanceName, Namespace: instanceNS},
					CredentialName:  "xns",
					APIKeySecretRef: litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("instanceRef.namespace is not supported"))
		})

		It("rejects a kind that can never be admitted, and an empty kinds list", func() {
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			for _, kinds := range [][]litellmv1alpha1.AllowedNamespaceKind{{"LiteLLMModel"}, {"LiteLLMCredential"}, {"LiteLLMGuardrail"}} {
				candidate := instance.DeepCopy()
				candidate.Spec.AllowedNamespaces = allowKinds(kinds...)
				Expect(k8sClient.Update(ctx, candidate)).NotTo(Succeed(), string(kinds[0]))
			}
			raw := client.RawPatch(types.MergePatchType, []byte(`{"spec":{"allowedNamespaces":{"from":"All","kinds":[]}}}`))
			Expect(k8sClient.Patch(ctx, instance.DeepCopy(), raw)).NotTo(Succeed())
		})

		It("requires a selector when allowedNamespaces.from is Selector", func() {
			instance := &litellmv1alpha1.LiteLLMInstance{}
			Expect(k8sClient.Get(ctx, instanceNN, instance)).To(Succeed())
			instance.Spec.AllowedNamespaces = &litellmv1alpha1.AllowedNamespaces{From: litellmv1alpha1.NamespacesFromSelector}
			err := k8sClient.Update(ctx, instance)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("selector is required"))
		})
	})
})

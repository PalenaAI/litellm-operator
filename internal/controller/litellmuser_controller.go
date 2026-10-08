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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMUserReconciler reconciles a LiteLLMUser object.
type LiteLLMUserReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmusers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

func (r *LiteLLMUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var user litellmv1alpha1.LiteLLMUser
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !user.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &user)
	}

	if !controllerutil.ContainsFinalizer(&user, FinalizerName) {
		controllerutil.AddFinalizer(&user, FinalizerName)
		if err := r.Update(ctx, &user); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, user.Namespace, user.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		emitEvent(r.Recorder, &user, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", user.Spec.InstanceRef.Name, err)
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "InstanceNotReady", Message: err.Error(),
		})
		_ = r.Status().Update(ctx, &user)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileUser(ctx, &user, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			emitEvent(r.Recorder, &user, corev1.EventTypeWarning, EventReasonEnterpriseRequired,
				"User %q requires a LiteLLM Enterprise license", user.Spec.UserEmail)
			meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: user.Generation,
			})
			_ = r.Status().Update(ctx, &user)
			return ctrl.Result{RequeueAfter: enterpriseLicenseRetryInterval}, nil
		}
		log.Error(err, "failed to reconcile user")
		emitEvent(r.Recorder, &user, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile user %q: %v", user.Spec.UserEmail, err)
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "SyncFailed", Message: err.Error(),
		})
	} else {
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionTrue, Reason: "Synced", Message: "User synced to LiteLLM",
		})
	}

	if statusErr := r.Status().Update(ctx, &user); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

func (r *LiteLLMUserReconciler) reconcileUser(
	ctx context.Context,
	user *litellmv1alpha1.LiteLLMUser,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))

	teamIDs, err := r.resolveTeamRefs(ctx, user)
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("resolve team refs: %w", err)
	}

	req := litellm.UserCreateRequest{
		UserID:           user.Spec.UserID,
		UserEmail:        user.Spec.UserEmail,
		UserRole:         user.Spec.UserRole,
		MaxBudget:        user.Spec.MaxBudget,
		BudgetDuration:   user.Spec.BudgetDuration,
		Models:           user.Spec.Models,
		Teams:            teamIDs,
		TPMLimit:         user.Spec.TPMLimit,
		RPMLimit:         user.Spec.RPMLimit,
		SoftBudget:       user.Spec.SoftBudget,
		ModelRPMLimit:    user.Spec.ModelRPMLimit,
		ModelTPMLimit:    user.Spec.ModelTPMLimit,
		ObjectPermission: mapObjectPermission(user.Spec.ObjectPermission),
		Metadata:         user.Spec.Metadata,
		Blocked:          user.Spec.Blocked,
	}

	if user.Status.LiteLLMUserID == "" {
		var userID string
		adopted := false
		resp, err := apiClient.Users().Create(ctx, req)
		if err != nil {
			if err := adoptExistingUser(ctx, apiClient, user, req, err); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
			// Persist the adopted mark before the id, so a failure between the
			// two writes can never leave an adopted user looking operator-created
			// (which would let a later CR deletion delete it). If it fails here,
			// the next reconcile simply adopts again.
			if err := markAdopted(ctx, r.Client, user); err != nil {
				return ctrl.Result{}, err
			}
			adopted = true
			userID = user.Spec.UserID
		} else {
			userID = resp.UserID
		}
		user.Status.LiteLLMUserID = userID
		user.Status.Synced = true
		if err := r.Status().Update(ctx, user); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if user.Annotations == nil {
			user.Annotations = map[string]string{}
		}
		user.Annotations[AnnotationSyncHash] = computeSpecHash(user.Spec)
		if err := r.Update(ctx, user); err != nil {
			return ctrl.Result{}, err
		}
		if adopted {
			log.Info("adopted existing user", "userId", userID)
			emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonAdopted,
				"Adopted existing LiteLLM user %q (id=%s)", user.Spec.UserEmail, userID)
		} else {
			log.Info("created user", "userId", userID)
			emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonCreated,
				"User %q registered with LiteLLM (id=%s)", user.Spec.UserEmail, userID)
		}
	} else {
		currentHash := computeSpecHash(user.Spec)
		if user.Annotations[AnnotationSyncHash] != currentHash {
			if err := apiClient.Users().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update user: %w", err)
			}
			if user.Annotations == nil {
				user.Annotations = map[string]string{}
			}
			user.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, user); err != nil {
				return ctrl.Result{}, err
			}
			user.Status.Synced = true
			log.Info("updated user", "userId", user.Status.LiteLLMUserID)
			emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonUpdated,
				"User %q updated in LiteLLM", user.Spec.UserEmail)
		}
	}

	result := ctrl.Result{RequeueAfter: 5 * time.Minute}
	if retry := r.reconcileInitialPassword(ctx, user, apiClient); retry > 0 {
		result.RequeueAfter = retry
	}

	info, err := apiClient.Users().Get(ctx, user.Status.LiteLLMUserID)
	if err == nil && info != nil {
		user.Status.CurrentSpend = info.Spend
	}

	now := metav1.Now()
	user.Status.LastSyncTime = &now
	return result, nil
}

// reconcileInitialPassword applies spec.initialPasswordSecretRef to the user.
// The password is sent only when it differs from the one the operator last
// applied (tracked as a bcrypt digest in status), so a password the user
// changes in the Admin UI is never overwritten by a resync — only by a change
// of the Secret's value. It goes through /user/update because LiteLLM rejects
// a password on /user/new.
//
// Failures are reported on the InitialPasswordApplied condition rather than
// returned, so they never mark the user itself as not synced. The returned
// duration is a requeue delay for transient failures, or 0. A missing Secret
// or a rejected password needs no short requeue: the Secret watch triggers a
// reconcile when it is created or changed.
func (r *LiteLLMUserReconciler) reconcileInitialPassword(
	ctx context.Context,
	user *litellmv1alpha1.LiteLLMUser,
	apiClient litellm.Client,
) time.Duration {
	ref := user.Spec.InitialPasswordSecretRef
	if ref == nil {
		// Forget the digest so re-adding the field applies the password again.
		user.Status.InitialPasswordDigest = ""
		meta.RemoveStatusCondition(&user.Status.Conditions, ConditionInitialPasswordApplied)
		return 0
	}

	setCondition := func(status metav1.ConditionStatus, reason, message string) {
		meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{
			Type:               ConditionInitialPasswordApplied,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: user.Generation,
		})
	}

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: user.Namespace}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			emitEvent(r.Recorder, user, corev1.EventTypeWarning, EventReasonSecretNotFound,
				"Initial password Secret %q not found", ref.Name)
			setCondition(metav1.ConditionFalse, "SecretNotFound",
				fmt.Sprintf("Secret %q not found", ref.Name))
			return 0
		}
		setCondition(metav1.ConditionFalse, "SecretFetchFailed",
			fmt.Sprintf("get Secret %q: %v", ref.Name, err))
		return 30 * time.Second
	}
	password := string(secret.Data[ref.Key])
	if password == "" {
		emitEvent(r.Recorder, user, corev1.EventTypeWarning, EventReasonSecretKeyMissing,
			"Initial password Secret %q has no non-empty key %q", ref.Name, ref.Key)
		setCondition(metav1.ConditionFalse, "SecretKeyMissing",
			fmt.Sprintf("Secret %q has no non-empty key %q", ref.Name, ref.Key))
		return 0
	}

	if passwordMatchesDigest(user.Status.InitialPasswordDigest, password) {
		setCondition(metav1.ConditionTrue, "Applied", "Initial password applied")
		return 0
	}

	if err := apiClient.Users().SetPassword(ctx, user.Status.LiteLLMUserID, password); err != nil {
		retry := 30 * time.Second
		reason := "ApplyFailed"
		if apiErr, ok := litellm.IsAPIError(err); ok && !apiErr.IsTransient() {
			// Typically a password-policy or breached-password rejection;
			// retrying the same value cannot succeed.
			retry, reason = 0, "PasswordRejected"
		}
		emitEvent(r.Recorder, user, corev1.EventTypeWarning, EventReasonPasswordRejected,
			"Failed to apply initial password from Secret %q: %v", ref.Name, err)
		setCondition(metav1.ConditionFalse, reason, err.Error())
		return retry
	}

	digest, err := passwordDigest(password)
	if err != nil {
		setCondition(metav1.ConditionFalse, "DigestFailed", err.Error())
		return 30 * time.Second
	}
	// Persist the digest right away with a merge patch (no resourceVersion
	// conflict) — if it were lost, the next resync would re-apply the password
	// and overwrite one the user may have changed since.
	base := user.DeepCopy()
	user.Status.InitialPasswordDigest = digest
	setCondition(metav1.ConditionTrue, "Applied", "Initial password applied")
	if err := r.Status().Patch(ctx, user, client.MergeFrom(base)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to record initial password digest")
	}
	logf.FromContext(ctx).Info("applied initial password", "userId", user.Status.LiteLLMUserID)
	emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonPasswordApplied,
		"Initial password from Secret %q applied to user %q", ref.Name, user.Status.LiteLLMUserID)
	return 0
}

// passwordDigest returns a bcrypt digest of password. The password is
// pre-hashed with SHA-256 so values beyond bcrypt's 72-byte limit are still
// fully compared.
func passwordDigest(password string) (string, error) {
	digest, err := bcrypt.GenerateFromPassword(prehashPassword(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("compute password digest: %w", err)
	}
	return string(digest), nil
}

// passwordMatchesDigest reports whether password is the one digest was made from.
func passwordMatchesDigest(digest, password string) bool {
	if digest == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(digest), prehashPassword(password)) == nil
}

func prehashPassword(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return []byte(hex.EncodeToString(sum[:]))
}

// adoptExistingUser handles a failed /user/new. It adopts the user only when
// the failure is a 409 and /user/info confirms spec.userId exists. LiteLLM also
// answers 409 when spec.userEmail belongs to a different user, and adopting
// then would bind the CR to the wrong account, so the conflict alone is not
// enough. On adoption the user is brought to the CR's spec with /user/update.
func adoptExistingUser(
	ctx context.Context,
	apiClient litellm.Client,
	user *litellmv1alpha1.LiteLLMUser,
	req litellm.UserCreateRequest,
	createErr error,
) error {
	apiErr, ok := litellm.IsAPIError(createErr)
	if !ok || apiErr.StatusCode != http.StatusConflict || user.Spec.UserID == "" {
		return fmt.Errorf("create user: %w", createErr)
	}
	if _, err := apiClient.Users().Get(ctx, user.Spec.UserID); err != nil {
		if getErr, ok := litellm.IsAPIError(err); ok && getErr.IsNotFound() {
			return fmt.Errorf("create user: %w (no user with id %q, so not adopting)", createErr, user.Spec.UserID)
		}
		return fmt.Errorf("create user: %w (could not check for existing user %q: %v)", createErr, user.Spec.UserID, err)
	}
	if err := apiClient.Users().Update(ctx, req); err != nil {
		return fmt.Errorf("update adopted user: %w", err)
	}
	return nil
}

func (r *LiteLLMUserReconciler) resolveTeamRefs(
	ctx context.Context,
	user *litellmv1alpha1.LiteLLMUser,
) ([]litellm.UserTeam, error) {
	teams := make([]litellm.UserTeam, 0, len(user.Spec.Teams))
	for _, t := range user.Spec.Teams {
		teamID := t.TeamID
		if t.TeamRef != nil {
			var team litellmv1alpha1.LiteLLMTeam
			if err := r.Get(ctx, types.NamespacedName{
				Name: t.TeamRef.Name, Namespace: user.Namespace,
			}, &team); err != nil {
				return nil, fmt.Errorf("resolve team ref %q: %w", t.TeamRef.Name, err)
			}
			teamID = team.Status.LiteLLMTeamID
			if teamID == "" {
				return nil, fmt.Errorf("team %q not yet synced (no litellmTeamId)", t.TeamRef.Name)
			}
		}
		teams = append(teams, litellm.UserTeam{
			TeamID:          teamID,
			Role:            t.Role,
			MaxBudgetInTeam: t.MaxBudgetInTeam,
		})
	}
	return teams, nil
}

func (r *LiteLLMUserReconciler) handleDeletion(ctx context.Context, user *litellmv1alpha1.LiteLLMUser) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(user, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if user.Annotations[AnnotationAdopted] == AnnotationAdoptedValue {
		emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonDeleted,
			"User %q was adopted, not created, by the operator; left in place in LiteLLM", user.Spec.UserEmail)
	} else if user.Status.LiteLLMUserID != "" {
		resolved, err := resolveInstance(ctx, r.Client, user.Namespace, user.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))
			if err := apiClient.Users().Delete(ctx, user.Status.LiteLLMUserID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete user from LiteLLM")
				emitEvent(r.Recorder, user, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete user %q from LiteLLM: %v", user.Spec.UserEmail, err)
			} else {
				emitEvent(r.Recorder, user, corev1.EventTypeNormal, EventReasonDeleted,
					"User %q deleted from LiteLLM", user.Spec.UserEmail)
			}
		}
	}
	controllerutil.RemoveFinalizer(user, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, user)
}

// findUsersForSecret enqueues every LiteLLMUser in the namespace of the
// changed Secret whose initialPasswordSecretRef points at it, so a new
// password is applied as soon as the Secret changes.
func (r *LiteLLMUserReconciler) findUsersForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	var users litellmv1alpha1.LiteLLMUserList
	if err := r.List(ctx, &users, client.InNamespace(secret.Namespace)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list LiteLLMUsers for Secret", "secret", secret.Name)
		return nil
	}
	var out []reconcile.Request
	for _, u := range users.Items {
		if ref := u.Spec.InitialPasswordSecretRef; ref != nil && ref.Name == secret.Name {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      u.Name,
				Namespace: u.Namespace,
			}})
		}
	}
	return out
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMUser{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.findUsersForSecret)).
		Named("litellmuser").
		Complete(r)
}

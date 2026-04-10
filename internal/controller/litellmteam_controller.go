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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMTeamReconciler reconciles a LiteLLMTeam object.
type LiteLLMTeamReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmteams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmteams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmteams/finalizers,verbs=update

func (r *LiteLLMTeamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var team litellmv1alpha1.LiteLLMTeam
	if err := r.Get(ctx, req.NamespacedName, &team); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !team.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &team)
	}

	if !controllerutil.ContainsFinalizer(&team, FinalizerName) {
		controllerutil.AddFinalizer(&team, FinalizerName)
		if err := r.Update(ctx, &team); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, team.Namespace, team.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		meta.SetStatusCondition(&team.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "InstanceNotReady", Message: err.Error(),
		})
		_ = r.Status().Update(ctx, &team)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileTeam(ctx, &team, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			meta.SetStatusCondition(&team.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: team.Generation,
			})
			_ = r.Status().Update(ctx, &team)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to reconcile team")
		meta.SetStatusCondition(&team.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "SyncFailed", Message: err.Error(),
		})
	} else {
		meta.SetStatusCondition(&team.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionTrue, Reason: "Synced", Message: "Team synced to LiteLLM",
		})
	}

	if statusErr := r.Status().Update(ctx, &team); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

func (r *LiteLLMTeamReconciler) reconcileTeam(
	ctx context.Context,
	team *litellmv1alpha1.LiteLLMTeam,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)

	if team.Status.LiteLLMTeamID == "" {
		req := litellm.TeamCreateRequest{
			TeamAlias:          team.Spec.TeamAlias,
			Models:             team.Spec.Models,
			MaxBudget:          team.Spec.MaxBudgetMonthly,
			BudgetDuration:     team.Spec.BudgetDuration,
			TPMLimit:           team.Spec.TPMLimit,
			RPMLimit:           team.Spec.RPMLimit,
			TeamMemberRPMLimit: team.Spec.TeamMemberRPMLimit,
			TeamMemberTPMLimit: team.Spec.TeamMemberTPMLimit,
			Metadata:           team.Spec.Metadata,
			Tags:               team.Spec.Tags,
		}
		resp, err := apiClient.Teams().Create(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create team: %w", err)
		}
		team.Status.LiteLLMTeamID = resp.TeamID
		team.Status.Synced = true
		if err := r.Status().Update(ctx, team); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if team.Annotations == nil {
			team.Annotations = map[string]string{}
		}
		team.Annotations[AnnotationSyncHash] = computeSpecHash(team.Spec)
		if err := r.Update(ctx, team); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created team", "teamId", resp.TeamID)
	} else {
		currentHash := computeSpecHash(team.Spec)
		if team.Annotations[AnnotationSyncHash] != currentHash {
			req := litellm.TeamUpdateRequest{
				TeamID:         team.Status.LiteLLMTeamID,
				TeamAlias:      team.Spec.TeamAlias,
				Models:         team.Spec.Models,
				MaxBudget:      team.Spec.MaxBudgetMonthly,
				BudgetDuration: team.Spec.BudgetDuration,
				TPMLimit:       team.Spec.TPMLimit,
				RPMLimit:       team.Spec.RPMLimit,
				Metadata:       team.Spec.Metadata,
				Tags:           team.Spec.Tags,
			}
			if err := apiClient.Teams().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update team: %w", err)
			}
			if team.Annotations == nil {
				team.Annotations = map[string]string{}
			}
			team.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, team); err != nil {
				return ctrl.Result{}, err
			}
			team.Status.Synced = true
			log.Info("updated team", "teamId", team.Status.LiteLLMTeamID)
		}
	}

	// Reconcile members
	if err := r.reconcileMembers(ctx, team, apiClient.Teams()); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("reconcile members: %w", err)
	}

	info, err := apiClient.Teams().Get(ctx, team.Status.LiteLLMTeamID)
	if err == nil && info != nil {
		team.Status.CurrentSpend = info.Spend
	}

	now := metav1.Now()
	team.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMTeamReconciler) reconcileMembers(
	ctx context.Context,
	team *litellmv1alpha1.LiteLLMTeam,
	teamSvc litellm.TeamService,
) error {
	log := logf.FromContext(ctx)
	mgmt := team.Spec.MemberManagement
	if mgmt == "" {
		mgmt = "mixed"
	}

	switch mgmt {
	case "sso":
		apiMembers, err := teamSvc.ListMembers(ctx, team.Status.LiteLLMTeamID)
		if err != nil {
			return fmt.Errorf("list members: %w", err)
		}
		team.Status.CRDMembers = nil
		team.Status.SSOMembers = toMemberStatusList(apiMembers, "sso")
		team.Status.TotalMemberCount = len(apiMembers)
		return nil

	case "crd":
		apiMembers, err := teamSvc.ListMembers(ctx, team.Status.LiteLLMTeamID)
		if err != nil {
			return fmt.Errorf("list members: %w", err)
		}
		desired := memberSet(team.Spec.Members)
		actual := memberEmailSet(apiMembers)

		for _, m := range team.Spec.Members {
			if !actual[m.Email] {
				if err := teamSvc.AddMember(ctx, team.Status.LiteLLMTeamID, m.Email, m.Role); err != nil {
					log.Error(err, "failed to add member", "email", m.Email)
					continue
				}
			}
		}
		for _, am := range apiMembers {
			if !desired[am.Email] {
				if err := teamSvc.RemoveMember(ctx, team.Status.LiteLLMTeamID, am.Email); err != nil {
					log.Error(err, "failed to remove member", "email", am.Email)
					continue
				}
			}
		}
		team.Status.CRDMembers = toMemberStatusListFromSpec(team.Spec.Members, "crd")
		team.Status.SSOMembers = nil
		team.Status.TotalMemberCount = len(team.Spec.Members)
		return nil

	case "mixed":
		apiMembers, err := teamSvc.ListMembers(ctx, team.Status.LiteLLMTeamID)
		if err != nil {
			return fmt.Errorf("list members: %w", err)
		}
		desired := memberSet(team.Spec.Members)
		actual := memberEmailSet(apiMembers)
		previousCRD := crdMemberSet(team.Status.CRDMembers)

		for _, m := range team.Spec.Members {
			if !actual[m.Email] {
				if err := teamSvc.AddMember(ctx, team.Status.LiteLLMTeamID, m.Email, m.Role); err != nil {
					log.Error(err, "failed to add member", "email", m.Email)
					continue
				}
			}
		}
		for email := range previousCRD {
			if !desired[email] {
				if err := teamSvc.RemoveMember(ctx, team.Status.LiteLLMTeamID, email); err != nil {
					log.Error(err, "failed to remove former CRD member", "email", email)
					continue
				}
			}
		}
		team.Status.CRDMembers = toMemberStatusListFromSpec(team.Spec.Members, "crd")
		var ssoMembers []litellmv1alpha1.TeamMemberStatus
		for _, am := range apiMembers {
			if !desired[am.Email] {
				ssoMembers = append(ssoMembers, litellmv1alpha1.TeamMemberStatus{
					Email: am.Email, Role: am.Role, Source: "sso", Synced: true,
				})
			}
		}
		team.Status.SSOMembers = ssoMembers
		team.Status.TotalMemberCount = len(team.Spec.Members) + len(ssoMembers)
		return nil

	default:
		return fmt.Errorf("unknown memberManagement mode: %q", mgmt)
	}
}

func (r *LiteLLMTeamReconciler) handleDeletion(ctx context.Context, team *litellmv1alpha1.LiteLLMTeam) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(team, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if team.Status.LiteLLMTeamID != "" {
		resolved, err := resolveInstance(ctx, r.Client, team.Namespace, team.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)
			if err := apiClient.Teams().Delete(ctx, team.Status.LiteLLMTeamID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete team from LiteLLM")
			}
		}
	}
	controllerutil.RemoveFinalizer(team, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, team)
}

func memberSet(members []litellmv1alpha1.TeamMember) map[string]bool {
	s := make(map[string]bool, len(members))
	for _, m := range members {
		s[m.Email] = true
	}
	return s
}

func memberEmailSet(members []litellm.TeamMemberInfo) map[string]bool {
	s := make(map[string]bool, len(members))
	for _, m := range members {
		s[m.Email] = true
	}
	return s
}

func crdMemberSet(members []litellmv1alpha1.TeamMemberStatus) map[string]bool {
	s := make(map[string]bool, len(members))
	for _, m := range members {
		s[m.Email] = true
	}
	return s
}

func toMemberStatusList(members []litellm.TeamMemberInfo, source string) []litellmv1alpha1.TeamMemberStatus {
	result := make([]litellmv1alpha1.TeamMemberStatus, 0, len(members))
	for _, m := range members {
		result = append(result, litellmv1alpha1.TeamMemberStatus{
			Email: m.Email, Role: m.Role, Source: source, Synced: true,
		})
	}
	return result
}

func toMemberStatusListFromSpec(members []litellmv1alpha1.TeamMember, source string) []litellmv1alpha1.TeamMemberStatus {
	result := make([]litellmv1alpha1.TeamMemberStatus, 0, len(members))
	for _, m := range members {
		role := m.Role
		if role == "" {
			role = "user"
		}
		result = append(result, litellmv1alpha1.TeamMemberStatus{
			Email: m.Email, Role: role, Source: source, Synced: true,
		})
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMTeamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMTeam{}).
		Named("litellmteam").
		Complete(r)
}

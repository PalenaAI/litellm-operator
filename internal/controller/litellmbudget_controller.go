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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// LiteLLMBudgetReconciler reconciles a LiteLLMBudget object.
type LiteLLMBudgetReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmbudgets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmbudgets/finalizers,verbs=update

func (r *LiteLLMBudgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var budget litellmv1alpha1.LiteLLMBudget
	if err := r.Get(ctx, req.NamespacedName, &budget); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !budget.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &budget)
	}

	if !controllerutil.ContainsFinalizer(&budget, FinalizerName) {
		controllerutil.AddFinalizer(&budget, FinalizerName)
		if err := r.Update(ctx, &budget); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, budget.Namespace, budget.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		emitEvent(r.Recorder, &budget, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", budget.Spec.InstanceRef.Name, err)
		meta.SetStatusCondition(&budget.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "InstanceNotReady", Message: err.Error(),
			ObservedGeneration: budget.Generation,
		})
		_ = r.Status().Update(ctx, &budget)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileBudget(ctx, &budget, resolved)
	if err != nil {
		log.Error(err, "failed to reconcile budget")
		emitEvent(r.Recorder, &budget, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile budget %q: %v", budget.Name, err)
		meta.SetStatusCondition(&budget.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionFalse, Reason: "SyncFailed", Message: err.Error(),
			ObservedGeneration: budget.Generation,
		})
	} else {
		meta.SetStatusCondition(&budget.Status.Conditions, metav1.Condition{
			Type: ConditionSynced, Status: metav1.ConditionTrue, Reason: "Synced", Message: "Budget synced to LiteLLM",
			ObservedGeneration: budget.Generation,
		})
	}

	if statusErr := r.Status().Update(ctx, &budget); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

// resolveBudgetID returns the stable budget_id: spec.budgetId when set,
// otherwise the object name.
func resolveBudgetID(budget *litellmv1alpha1.LiteLLMBudget) string {
	if budget.Spec.BudgetID != "" {
		return budget.Spec.BudgetID
	}
	return budget.Name
}

func (r *LiteLLMBudgetReconciler) reconcileBudget(
	ctx context.Context,
	budget *litellmv1alpha1.LiteLLMBudget,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))

	budgetID := resolveBudgetID(budget)
	req := litellm.BudgetRequest{
		BudgetID:            budgetID,
		MaxBudget:           budget.Spec.MaxBudget,
		SoftBudget:          budget.Spec.SoftBudget,
		BudgetDuration:      budget.Spec.BudgetDuration,
		TPMLimit:            budget.Spec.TPMLimit,
		RPMLimit:            budget.Spec.RPMLimit,
		MaxParallelRequests: budget.Spec.MaxParallelRequests,
		ModelMaxBudget:      budget.Spec.ModelMaxBudget,
	}

	if budget.Status.LiteLLMBudgetID == "" {
		resp, err := apiClient.Budgets().Create(ctx, req)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create budget: %w", err)
		}
		id := resp.BudgetID
		if id == "" {
			id = budgetID
		}
		budget.Status.LiteLLMBudgetID = id
		budget.Status.Synced = true
		if err := r.Status().Update(ctx, budget); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if budget.Annotations == nil {
			budget.Annotations = map[string]string{}
		}
		budget.Annotations[AnnotationSyncHash] = computeSpecHash(budget.Spec)
		if err := r.Update(ctx, budget); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created budget", "budgetId", id)
		emitEvent(r.Recorder, budget, corev1.EventTypeNormal, EventReasonCreated,
			"Budget registered with LiteLLM (id=%s)", id)
	} else {
		currentHash := computeSpecHash(budget.Spec)
		if budget.Annotations[AnnotationSyncHash] != currentHash {
			req.BudgetID = budget.Status.LiteLLMBudgetID
			if err := apiClient.Budgets().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update budget: %w", err)
			}
			if budget.Annotations == nil {
				budget.Annotations = map[string]string{}
			}
			budget.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, budget); err != nil {
				return ctrl.Result{}, err
			}
			budget.Status.Synced = true
			log.Info("updated budget", "budgetId", budget.Status.LiteLLMBudgetID)
			emitEvent(r.Recorder, budget, corev1.EventTypeNormal, EventReasonUpdated,
				"Budget %q updated in LiteLLM", budget.Status.LiteLLMBudgetID)
		}
	}

	// Refresh spend from API (best-effort).
	info, err := apiClient.Budgets().Get(ctx, budget.Status.LiteLLMBudgetID)
	if err == nil && info != nil {
		budget.Status.CurrentSpend = info.Spend
	} else if err != nil {
		log.V(1).Info("failed to refresh budget info from API", "error", err)
	}

	now := metav1.Now()
	budget.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMBudgetReconciler) handleDeletion(ctx context.Context, budget *litellmv1alpha1.LiteLLMBudget) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(budget, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if budget.Status.LiteLLMBudgetID != "" {
		resolved, err := resolveInstance(ctx, r.Client, budget.Namespace, budget.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey, litellm.WithCACert(resolved.CACert))
			if err := apiClient.Budgets().Delete(ctx, budget.Status.LiteLLMBudgetID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete budget from LiteLLM")
				emitEvent(r.Recorder, budget, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete budget %q from LiteLLM: %v", budget.Status.LiteLLMBudgetID, err)
			} else {
				emitEvent(r.Recorder, budget, corev1.EventTypeNormal, EventReasonDeleted,
					"Budget %q deleted from LiteLLM", budget.Status.LiteLLMBudgetID)
			}
		}
	}
	controllerutil.RemoveFinalizer(budget, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, budget)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMBudget{}).
		Named("litellmbudget").
		Complete(r)
}

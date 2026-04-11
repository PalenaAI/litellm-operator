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

// LiteLLMCustomerReconciler reconciles a LiteLLMCustomer object.
type LiteLLMCustomerReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcustomers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcustomers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcustomers/finalizers,verbs=update

func (r *LiteLLMCustomerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var customer litellmv1alpha1.LiteLLMCustomer
	if err := r.Get(ctx, req.NamespacedName, &customer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !customer.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &customer)
	}

	if !controllerutil.ContainsFinalizer(&customer, FinalizerName) {
		controllerutil.AddFinalizer(&customer, FinalizerName)
		if err := r.Update(ctx, &customer); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolved, err := resolveInstance(ctx, r.Client, customer.Namespace, customer.Spec.InstanceRef)
	if err != nil {
		log.Error(err, "failed to resolve instance")
		emitEvent(r.Recorder, &customer, corev1.EventTypeWarning, EventReasonInstanceNotReady,
			"Referenced LiteLLMInstance %q is not ready: %v", customer.Spec.InstanceRef.Name, err)
		meta.SetStatusCondition(&customer.Status.Conditions, metav1.Condition{
			Type:               ConditionSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "InstanceNotReady",
			Message:            err.Error(),
			ObservedGeneration: customer.Generation,
		})
		_ = r.Status().Update(ctx, &customer)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	result, err := r.reconcileCustomer(ctx, &customer, resolved)
	if err != nil {
		if isEnterpriseLicenseError(err) {
			emitEvent(r.Recorder, &customer, corev1.EventTypeWarning, EventReasonEnterpriseRequired,
				"Customer %q requires a LiteLLM Enterprise license", customer.Spec.CustomerID)
			meta.SetStatusCondition(&customer.Status.Conditions, metav1.Condition{
				Type:               ConditionSynced,
				Status:             metav1.ConditionFalse,
				Reason:             "EnterpriseLicenseRequired",
				Message:            "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate.",
				ObservedGeneration: customer.Generation,
			})
			_ = r.Status().Update(ctx, &customer)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to reconcile customer")
		emitEvent(r.Recorder, &customer, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Failed to reconcile customer %q: %v", customer.Spec.CustomerID, err)
		meta.SetStatusCondition(&customer.Status.Conditions, metav1.Condition{
			Type:               ConditionSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "SyncFailed",
			Message:            err.Error(),
			ObservedGeneration: customer.Generation,
		})
	} else {
		meta.SetStatusCondition(&customer.Status.Conditions, metav1.Condition{
			Type:               ConditionSynced,
			Status:             metav1.ConditionTrue,
			Reason:             "Synced",
			Message:            "Customer synced to LiteLLM",
			ObservedGeneration: customer.Generation,
		})
	}

	if statusErr := r.Status().Update(ctx, &customer); statusErr != nil {
		log.Error(statusErr, "failed to update status")
	}
	return result, err
}

func (r *LiteLLMCustomerReconciler) reconcileCustomer(
	ctx context.Context,
	customer *litellmv1alpha1.LiteLLMCustomer,
	resolved *ResolvedInstance,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)

	objectPerm := mapCustomerObjectPermission(customer.Spec.ObjectPermission)

	if !customer.Status.Synced {
		req := litellm.CustomerCreateRequest{
			UserID:             customer.Spec.CustomerID,
			Alias:              customer.Spec.Alias,
			MaxBudget:          customer.Spec.MaxBudget,
			BudgetDuration:     customer.Spec.BudgetDuration,
			BudgetID:           customer.Spec.BudgetID,
			TpmLimit:           customer.Spec.TpmLimit,
			RpmLimit:           customer.Spec.RpmLimit,
			AllowedModelRegion: customer.Spec.AllowedModelRegion,
			DefaultModel:       customer.Spec.DefaultModel,
			Models:             customer.Spec.Models,
			Blocked:            customer.Spec.Blocked,
			ObjectPermission:   objectPerm,
			Metadata:           customer.Spec.Metadata,
		}
		if _, err := apiClient.Customers().Create(ctx, req); err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("create customer: %w", err)
		}
		customer.Status.Synced = true
		if err := r.Status().Update(ctx, customer); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after create: %w", err)
		}
		if customer.Annotations == nil {
			customer.Annotations = map[string]string{}
		}
		customer.Annotations[AnnotationSyncHash] = computeSpecHash(customer.Spec)
		if err := r.Update(ctx, customer); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("created customer", "customerId", customer.Spec.CustomerID)
		emitEvent(r.Recorder, customer, corev1.EventTypeNormal, EventReasonCreated,
			"Customer %q registered with LiteLLM", customer.Spec.CustomerID)
	} else {
		currentHash := computeSpecHash(customer.Spec)
		if customer.Annotations[AnnotationSyncHash] != currentHash {
			req := litellm.CustomerUpdateRequest{
				UserID:             customer.Spec.CustomerID,
				Alias:              customer.Spec.Alias,
				MaxBudget:          customer.Spec.MaxBudget,
				BudgetDuration:     customer.Spec.BudgetDuration,
				BudgetID:           customer.Spec.BudgetID,
				TpmLimit:           customer.Spec.TpmLimit,
				RpmLimit:           customer.Spec.RpmLimit,
				AllowedModelRegion: customer.Spec.AllowedModelRegion,
				DefaultModel:       customer.Spec.DefaultModel,
				Models:             customer.Spec.Models,
				Blocked:            customer.Spec.Blocked,
				ObjectPermission:   objectPerm,
				Metadata:           customer.Spec.Metadata,
			}
			if _, err := apiClient.Customers().Update(ctx, req); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("update customer: %w", err)
			}
			if customer.Annotations == nil {
				customer.Annotations = map[string]string{}
			}
			customer.Annotations[AnnotationSyncHash] = currentHash
			if err := r.Update(ctx, customer); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("updated customer", "customerId", customer.Spec.CustomerID)
			emitEvent(r.Recorder, customer, corev1.EventTypeNormal, EventReasonUpdated,
				"Customer %q updated in LiteLLM", customer.Spec.CustomerID)
		}
	}

	// Refresh spend and blocked state from API (best-effort)
	info, err := apiClient.Customers().Get(ctx, customer.Spec.CustomerID)
	if err == nil && info != nil {
		customer.Status.CurrentSpend = info.Spend
		customer.Status.Blocked = info.Blocked
	} else if err != nil {
		log.V(1).Info("failed to refresh customer info from API", "error", err)
	}

	now := metav1.Now()
	customer.Status.LastSyncTime = &now
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *LiteLLMCustomerReconciler) handleDeletion(
	ctx context.Context,
	customer *litellmv1alpha1.LiteLLMCustomer,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(customer, FinalizerName) {
		return ctrl.Result{}, nil
	}
	if customer.Status.Synced {
		resolved, err := resolveInstance(ctx, r.Client, customer.Namespace, customer.Spec.InstanceRef)
		if err == nil {
			apiClient := r.LiteLLMClientFactory(resolved.Endpoint, resolved.MasterKey)
			if err := apiClient.Customers().Delete(ctx, customer.Spec.CustomerID); err != nil {
				logf.FromContext(ctx).Error(err, "failed to delete customer from LiteLLM")
				emitEvent(r.Recorder, customer, corev1.EventTypeWarning, EventReasonReconcileFailed,
					"Failed to delete customer %q from LiteLLM: %v", customer.Spec.CustomerID, err)
			} else {
				emitEvent(r.Recorder, customer, corev1.EventTypeNormal, EventReasonDeleted,
					"Customer %q deleted from LiteLLM", customer.Spec.CustomerID)
			}
		}
	}
	controllerutil.RemoveFinalizer(customer, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, customer)
}

// mapCustomerObjectPermission converts a CRD ObjectPermission into the API shape.
func mapCustomerObjectPermission(p *litellmv1alpha1.CustomerObjectPermission) *litellm.CustomerObjectPermission {
	if p == nil {
		return nil
	}
	return &litellm.CustomerObjectPermission{
		MCPServers:   p.MCPServers,
		AccessGroups: p.AccessGroups,
		VectorStores: p.VectorStores,
		Agents:       p.Agents,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMCustomerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMCustomer{}).
		Named("litellmcustomer").
		Complete(r)
}

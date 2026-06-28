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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
	"github.com/PalenaAI/litellm-operator/internal/resources"
)

// LiteLLMInstanceReconciler reconciles a LiteLLMInstance object.
type LiteLLMInstanceReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	LiteLLMClientFactory litellm.ClientFactory
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmcredentials,verbs=get;list;watch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellmguardrails,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps;services;secrets;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=scheduledbackups,verbs=get;list;watch;create;update;patch;delete

func (r *LiteLLMInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var instance litellmv1alpha1.LiteLLMInstance
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &instance)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&instance, FinalizerName) {
		controllerutil.AddFinalizer(&instance, FinalizerName)
		return ctrl.Result{}, r.Update(ctx, &instance)
	}

	labels := labelsForInstance(instance.Name)

	// Detect license Secret
	licenseSecretName := r.reconcileLicense(ctx, &instance)

	// Fetch LiteLLMGuardrail CRs bound to this instance. Failure is
	// non-fatal — guardrails are an optional config-level feature.
	guardrails, err := r.listGuardrailsForInstance(ctx, &instance)
	if err != nil {
		logf.FromContext(ctx).Error(err, "failed to list guardrails for instance")
	}

	// Reconcile database migration status
	r.reconcileMigrationStatus(ctx, &instance, labels)

	// Reconcile all managed resources
	reconcileErr := r.reconcileResources(ctx, &instance, labels, licenseSecretName, guardrails)

	// Auto-rollback: track successful deployment revision and rollback on failure
	r.reconcileAutoRollback(ctx, &instance)

	// Update status
	r.updateInstanceStatus(ctx, &instance, reconcileErr)

	if reconcileErr != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// reconcileMigrationStatus reconciles the database migration job and updates the DatabaseReady condition.
func (r *LiteLLMInstanceReconciler) reconcileMigrationStatus(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) {
	log := logf.FromContext(ctx)

	if instance.Spec.Database.Migration == nil || !instance.Spec.Database.Migration.Enabled {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionDatabaseReady,
			Status:             metav1.ConditionTrue,
			Reason:             "MigrationSkipped",
			Message:            "Database migration not enabled; LiteLLM handles migrations on startup",
			ObservedGeneration: instance.Generation,
		})
		return
	}

	migrationDone, err := r.reconcileMigrationJob(ctx, instance, labels)
	if err != nil {
		log.Error(err, "failed to reconcile migration Job")
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionDatabaseReady,
			Status:             metav1.ConditionFalse,
			Reason:             "MigrationFailed",
			Message:            err.Error(),
			ObservedGeneration: instance.Generation,
		})
		return
	}

	if migrationDone {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionDatabaseReady,
			Status:             metav1.ConditionTrue,
			Reason:             "MigrationComplete",
			Message:            "Database migration completed successfully",
			ObservedGeneration: instance.Generation,
		})
	} else {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionDatabaseReady,
			Status:             metav1.ConditionFalse,
			Reason:             "MigrationRunning",
			Message:            "Database migration is in progress",
			ObservedGeneration: instance.Generation,
		})
	}
}

// listGuardrailsForInstance returns every LiteLLMGuardrail in the instance's
// namespace whose spec.instanceRef.name matches the instance.
func (r *LiteLLMInstanceReconciler) listGuardrailsForInstance(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) ([]litellmv1alpha1.LiteLLMGuardrail, error) {
	var list litellmv1alpha1.LiteLLMGuardrailList
	if err := r.List(ctx, &list, client.InNamespace(instance.Namespace)); err != nil {
		return nil, err
	}
	filtered := make([]litellmv1alpha1.LiteLLMGuardrail, 0, len(list.Items))
	for _, g := range list.Items {
		if g.Spec.InstanceRef.Name == instance.Name {
			filtered = append(filtered, g)
		}
	}
	return filtered, nil
}

// reconcileResources reconciles all managed sub-resources for the instance.
func (r *LiteLLMInstanceReconciler) reconcileResources(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string, licenseSecretName string, guardrails []litellmv1alpha1.LiteLLMGuardrail) error {
	log := logf.FromContext(ctx)
	var reconcileErr error

	if err := r.reconcileSecrets(ctx, instance); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile secrets")
	}

	if err := r.reconcileConfigMap(ctx, instance, labels, guardrails); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile ConfigMap")
	}

	if err := r.reconcileServiceAccount(ctx, instance, labels); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile ServiceAccount")
	}

	if err := r.reconcileDeployment(ctx, instance, labels, licenseSecretName, guardrails); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile Deployment")
	}

	if err := r.reconcileService(ctx, instance, labels); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile Service")
	}

	if err := r.reconcileOptionalResources(ctx, instance, labels); err != nil {
		reconcileErr = err
	}

	return reconcileErr
}

// reconcileOptionalResources reconciles resources that are conditionally enabled.
func (r *LiteLLMInstanceReconciler) reconcileOptionalResources(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	var reconcileErr error

	if err := r.reconcileNetworkingResources(ctx, instance, labels); err != nil {
		reconcileErr = err
	}

	if err := r.reconcileScalingResources(ctx, instance, labels); err != nil {
		reconcileErr = err
	}

	if err := r.reconcileSecurityResources(ctx, instance); err != nil {
		reconcileErr = err
	}

	if err := r.reconcileObservabilityResources(ctx, instance, labels); err != nil {
		reconcileErr = err
	}

	return reconcileErr
}

// reconcileNetworkingResources reconciles Ingress, Route, and HTTPRoute.
func (r *LiteLLMInstanceReconciler) reconcileNetworkingResources(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	log := logf.FromContext(ctx)
	var reconcileErr error

	if instance.Spec.Ingress != nil && instance.Spec.Ingress.Enabled {
		if err := r.reconcileIngress(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile Ingress")
		}
	}

	if instance.Spec.Route != nil && instance.Spec.Route.Enabled {
		if err := r.reconcileRoute(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile Route")
		}
	}

	if instance.Spec.GatewayHTTPRoute != nil && instance.Spec.GatewayHTTPRoute.Enabled {
		if err := r.reconcileHTTPRoute(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile HTTPRoute")
		}
	}

	return reconcileErr
}

// reconcileScalingResources reconciles HPA, PDB, and NetworkPolicy.
func (r *LiteLLMInstanceReconciler) reconcileScalingResources(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	log := logf.FromContext(ctx)
	var reconcileErr error

	if instance.Spec.Autoscaling != nil && instance.Spec.Autoscaling.Enabled {
		if err := r.reconcileHPA(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile HPA")
		}
	}

	if instance.Spec.PodDisruptionBudget != nil && instance.Spec.PodDisruptionBudget.Enabled {
		if err := r.reconcilePDB(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile PDB")
		}
	}

	if instance.Spec.Security != nil && instance.Spec.Security.NetworkPolicy != nil && instance.Spec.Security.NetworkPolicy.Enabled {
		if err := r.reconcileNetworkPolicy(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile NetworkPolicy")
		}
	}

	return reconcileErr
}

// reconcileSecurityResources reconciles SCIM tokens.
func (r *LiteLLMInstanceReconciler) reconcileSecurityResources(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) error {
	log := logf.FromContext(ctx)

	if instance.Spec.SCIM != nil && instance.Spec.SCIM.Enabled {
		if err := r.reconcileSCIMToken(ctx, instance); err != nil {
			log.Error(err, "failed to reconcile SCIM token")
			return err
		}
	}

	return nil
}

// reconcileObservabilityResources reconciles ServiceMonitor, PrometheusRule, Grafana dashboard, and CNPG backup.
func (r *LiteLLMInstanceReconciler) reconcileObservabilityResources(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	log := logf.FromContext(ctx)
	var reconcileErr error

	if instance.Spec.Observability != nil && instance.Spec.Observability.ServiceMonitor != nil && instance.Spec.Observability.ServiceMonitor.Enabled {
		if err := r.reconcileServiceMonitor(ctx, instance, labels); err != nil {
			log.V(1).Info("failed to reconcile ServiceMonitor (monitoring.coreos.com CRDs may not be installed)", "error", err)
		}
	}

	if instance.Spec.Observability != nil && instance.Spec.Observability.PrometheusRule != nil && instance.Spec.Observability.PrometheusRule.Enabled {
		if err := r.reconcilePrometheusRule(ctx, instance, labels); err != nil {
			log.V(1).Info("failed to reconcile PrometheusRule (monitoring.coreos.com CRDs may not be installed)", "error", err)
		}
	}

	if instance.Spec.Observability != nil && instance.Spec.Observability.GrafanaDashboard != nil && instance.Spec.Observability.GrafanaDashboard.Enabled {
		if err := r.reconcileGrafanaDashboard(ctx, instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile Grafana dashboard ConfigMap")
		}
	}

	if instance.Spec.Database.CloudNativePG != nil && instance.Spec.Database.CloudNativePG.Backup != nil && instance.Spec.Database.CloudNativePG.Backup.Enabled {
		if err := r.reconcileCNPGBackup(ctx, instance, labels); err != nil {
			log.V(1).Info("failed to reconcile CNPG ScheduledBackup (postgresql.cnpg.io CRDs may not be installed)", "error", err)
		} else {
			instance.Status.Backup = &litellmv1alpha1.BackupStatus{
				Configured: true,
			}
		}
	}

	return reconcileErr
}

// reconcileLicense detects a license Secret for the instance.
// Returns the Secret name if found, or empty string if not.
func (r *LiteLLMInstanceReconciler) reconcileLicense(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) string {
	log := logf.FromContext(ctx)

	// Try per-instance Secret first, then namespace-wide fallback
	secretNames := []string{
		instance.Name + "-license",
		"litellm-license",
	}

	for _, name := range secretNames {
		var secret corev1.Secret
		err := r.Get(ctx, client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      name,
		}, &secret)

		if err == nil {
			if _, ok := secret.Data["license-key"]; ok {
				log.V(1).Info("license Secret found", "secret", name)
				instance.Status.License = &litellmv1alpha1.LicenseStatus{
					Active:     true,
					SecretName: name,
				}
				return name
			}
			log.Info("license Secret found but missing 'license-key' key", "secret", name)
			continue
		}

		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to check license Secret", "secret", name)
		}
	}

	log.V(1).Info("no license Secret found, running in open-source mode")
	instance.Status.License = &litellmv1alpha1.LicenseStatus{
		Active: false,
	}
	return ""
}

// findInstanceForLicenseSecret maps a Secret event to the LiteLLMInstance(s) that should be reconciled.
func (r *LiteLLMInstanceReconciler) findInstanceForLicenseSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	// Namespace-wide license Secret — reconcile all instances in this namespace
	if secret.Name == "litellm-license" {
		var instances litellmv1alpha1.LiteLLMInstanceList
		if err := r.List(ctx, &instances, client.InNamespace(secret.Namespace)); err != nil {
			return nil
		}
		requests := make([]reconcile.Request, 0, len(instances.Items))
		for _, inst := range instances.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      inst.Name,
					Namespace: inst.Namespace,
				},
			})
		}
		return requests
	}

	// Per-instance license Secret ({name}-license)
	if !strings.HasSuffix(secret.Name, "-license") {
		return nil
	}
	instanceName := strings.TrimSuffix(secret.Name, "-license")

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      instanceName,
			Namespace: secret.Namespace,
		},
	}}
}

func (r *LiteLLMInstanceReconciler) handleDeletion(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(instance, FinalizerName) {
		return ctrl.Result{}, nil
	}
	controllerutil.RemoveFinalizer(instance, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, instance)
}

func (r *LiteLLMInstanceReconciler) reconcileSecrets(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) error {
	// Auto-generate master key
	if instance.Spec.MasterKey.AutoGenerate {
		if err := r.ensureGeneratedSecret(ctx, instance, instance.Name+"-master-key", "master-key"); err != nil {
			return fmt.Errorf("auto-generate master key: %w", err)
		}
	}
	// Auto-generate salt key
	if instance.Spec.SaltKey != nil && instance.Spec.SaltKey.AutoGenerate {
		if err := r.ensureGeneratedSecret(ctx, instance, instance.Name+"-salt-key", "salt-key"); err != nil {
			return fmt.Errorf("auto-generate salt key: %w", err)
		}
	}
	return nil
}

func (r *LiteLLMInstanceReconciler) ensureGeneratedSecret(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, name, key string) error {
	var existing corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, &existing)
	if err == nil {
		return nil // already exists
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	token := generateRandomToken(32)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForInstance(instance.Name),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			key: "sk-" + token,
		},
	}
	if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, secret)
}

func (r *LiteLLMInstanceReconciler) reconcileConfigMap(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string, guardrails []litellmv1alpha1.LiteLLMGuardrail) error {
	desired, err := resources.BuildConfigMap(instance, labels, guardrails)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &corev1.ConfigMap{})
}

func (r *LiteLLMInstanceReconciler) reconcileServiceAccount(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildServiceAccount(instance, labels)
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}

	var existing corev1.ServiceAccount
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	return err
}

// reconcileMigrationJob ensures the migration Job exists and reports its status.
// Returns (true, nil) if the job succeeded, (false, nil) if still running, or (false, err) on failure.
func (r *LiteLLMInstanceReconciler) reconcileMigrationJob(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) (bool, error) {
	desired := resources.BuildMigrationJob(instance, labels)
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return false, err
	}

	var existing batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		return false, r.Create(ctx, desired)
	}
	if err != nil {
		return false, err
	}

	// Check job status
	if existing.Status.Succeeded > 0 {
		return true, nil
	}
	if existing.Status.Failed > 0 && (existing.Spec.BackoffLimit != nil && existing.Status.Failed >= *existing.Spec.BackoffLimit) {
		return false, fmt.Errorf("migration job %s failed after %d attempts", desired.Name, existing.Status.Failed)
	}

	return false, nil // still running
}

func (r *LiteLLMInstanceReconciler) reconcileDeployment(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string, licenseSecretName string, guardrails []litellmv1alpha1.LiteLLMGuardrail) error {
	desired := resources.BuildDeployment(instance, labels, licenseSecretName, guardrails)
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update deployment spec
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template = desired.Spec.Template
	existing.Spec.Strategy = desired.Spec.Strategy
	return r.Update(ctx, &existing)
}

func (r *LiteLLMInstanceReconciler) reconcileService(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildService(instance, labels)
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &corev1.Service{})
}

func (r *LiteLLMInstanceReconciler) reconcileIngress(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildIngress(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &networkingv1.Ingress{})
}

func (r *LiteLLMInstanceReconciler) reconcileHPA(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildHPA(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &autoscalingv2.HorizontalPodAutoscaler{})
}

func (r *LiteLLMInstanceReconciler) reconcilePDB(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildPDB(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &policyv1.PodDisruptionBudget{})
}

func (r *LiteLLMInstanceReconciler) reconcileNetworkPolicy(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildNetworkPolicy(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &networkingv1.NetworkPolicy{})
}

func (r *LiteLLMInstanceReconciler) reconcileRoute(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildRoute(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	})
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, desired)
}

func (r *LiteLLMInstanceReconciler) reconcileHTTPRoute(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildHTTPRoute(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &gatewayv1.HTTPRoute{})
}

func (r *LiteLLMInstanceReconciler) reconcileSCIMToken(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) error {
	if instance.Spec.SCIM.TokenSecretRef != nil {
		// User-provided token; nothing to do
		instance.Status.SCIM = &litellmv1alpha1.SCIMStatus{
			Configured:      true,
			TokenSecretName: instance.Spec.SCIM.TokenSecretRef.Name,
		}
		return nil
	}

	secretName := instance.Spec.SCIM.GeneratedTokenSecretName
	if secretName == "" {
		secretName = "litellm-scim-token"
	}

	if err := r.ensureGeneratedSecret(ctx, instance, secretName, "token"); err != nil {
		return err
	}

	instance.Status.SCIM = &litellmv1alpha1.SCIMStatus{
		Configured:      true,
		TokenSecretName: secretName,
	}
	return nil
}

func (r *LiteLLMInstanceReconciler) reconcileServiceMonitor(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildServiceMonitor(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	})
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, desired)
}

func (r *LiteLLMInstanceReconciler) reconcilePrometheusRule(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildPrometheusRule(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "PrometheusRule",
	})
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, desired)
}

func (r *LiteLLMInstanceReconciler) reconcileGrafanaDashboard(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildGrafanaDashboardConfigMap(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}
	return r.createOrUpdate(ctx, desired, &corev1.ConfigMap{})
}

func (r *LiteLLMInstanceReconciler) reconcileCNPGBackup(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildCNPGScheduledBackup(instance, labels)
	if desired == nil {
		return nil
	}
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return err
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "postgresql.cnpg.io",
		Version: "v1",
		Kind:    "ScheduledBackup",
	})
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, desired)
}

// reconcileAutoRollback checks whether the deployment is healthy after an upgrade.
// If auto-rollback is enabled and the deployment is in a failed state, it rolls back
// to the last known-good revision by restoring the previous pod template hash.
func (r *LiteLLMInstanceReconciler) reconcileAutoRollback(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) {
	log := logf.FromContext(ctx)

	if instance.Spec.Upgrade == nil || !instance.Spec.Upgrade.AutoRollback {
		return
	}

	var dep appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &dep); err != nil {
		return
	}

	currentRevision := dep.Annotations["deployment.kubernetes.io/revision"]

	// If the deployment is healthy (all replicas available), record this as a good revision.
	if dep.Status.AvailableReplicas == dep.Status.Replicas && dep.Status.Replicas > 0 {
		if instance.Status.LastSuccessfulRevision != currentRevision {
			instance.Status.LastSuccessfulRevision = currentRevision
			log.V(1).Info("recorded successful deployment revision", "revision", currentRevision)
		}
		return
	}

	// If the deployment is unhealthy and we have a previous good revision, check if rollback is needed.
	if instance.Status.LastSuccessfulRevision == "" || instance.Status.LastSuccessfulRevision == currentRevision {
		return
	}

	// Check if the deployment has been stuck for a while (at least one progress deadline exceeded condition).
	for _, cond := range dep.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse && cond.Reason == "ProgressDeadlineExceeded" {
			log.Info("auto-rollback triggered: deployment progress deadline exceeded",
				"currentRevision", currentRevision,
				"lastSuccessfulRevision", instance.Status.LastSuccessfulRevision,
			)

			// Trigger rollback by performing a rollout restart annotation change.
			// This forces a new rollout which, combined with the operator re-reconciling
			// the desired state, effectively rolls back to the last working config.
			if dep.Spec.Template.Annotations == nil {
				dep.Spec.Template.Annotations = make(map[string]string)
			}
			dep.Spec.Template.Annotations["litellm.palena.ai/auto-rollback"] = time.Now().Format(time.RFC3339)

			if err := r.Update(ctx, &dep); err != nil {
				log.Error(err, "failed to trigger auto-rollback")
				return
			}

			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             "AutoRollback",
				Message:            fmt.Sprintf("Auto-rollback triggered from revision %s (last successful: %s)", currentRevision, instance.Status.LastSuccessfulRevision),
				ObservedGeneration: instance.Generation,
			})
			return
		}
	}
}

func (r *LiteLLMInstanceReconciler) updateInstanceStatus(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, reconcileErr error) {
	// Fetch deployment status
	var dep appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, &dep); err == nil {
		instance.Status.Replicas = dep.Status.Replicas
		instance.Status.ReadyReplicas = dep.Status.ReadyReplicas
		instance.Status.Ready = dep.Status.ReadyReplicas > 0
	}

	// Set endpoint
	port := instance.Spec.Service.Port
	if port == 0 {
		port = 4000
	}
	instance.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc:%d", instance.Name, instance.Namespace, port)

	// Set version
	instance.Status.Version = instance.Spec.Image.Tag
	if instance.Status.Version == "" {
		instance.Status.Version = "latest"
	}

	// SSO status
	if instance.Spec.SSO != nil && instance.Spec.SSO.Enabled {
		instance.Status.SSO = &litellmv1alpha1.SSOStatus{
			Configured: true,
			Provider:   instance.Spec.SSO.Provider,
		}
	}

	// Enterprise features warning: JWT/OAuth2 auth require a license
	r.checkEnterpriseFeaturesWarning(instance)

	// Secret manager status
	r.reconcileSecretManagerStatus(ctx, instance)

	// Validate TLS Secrets (serve cert, outbound CA, client cert, DB TLS)
	r.validateTLSSecrets(ctx, instance)

	// Ready condition
	if reconcileErr != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ReconcileError",
			Message:            reconcileErr.Error(),
			ObservedGeneration: instance.Generation,
		})
		emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonReconcileFailed,
			"Reconcile failed: %v", reconcileErr)
	} else if instance.Status.Ready {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "AllResourcesReady",
			Message:            "All managed resources are ready",
			ObservedGeneration: instance.Generation,
		})
	} else {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "DeploymentNotReady",
			Message:            "Waiting for deployment to become ready",
			ObservedGeneration: instance.Generation,
		})
	}

	// Probe the LiteLLM health endpoints once the deployment has at least
	// one ready replica. This populates operand-level health into status,
	// sets the RedisReady condition when caching is wired to Redis, and
	// emits HealthDegraded / HealthRestored events so operators are
	// notified without tailing logs.
	if instance.Status.Ready && r.LiteLLMClientFactory != nil {
		r.probeInstanceHealth(ctx, instance)
	}

	_ = r.Status().Update(ctx, instance)
}

// reconcileSecretManagerStatus validates the secret manager configuration and
// updates status.secretManager. When a credentialsSecretRef is specified, the
// referenced Secret must exist; otherwise a warning event is emitted.
func (r *LiteLLMInstanceReconciler) reconcileSecretManagerStatus(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) {
	sm := instance.Spec.SecretManager
	if sm == nil {
		instance.Status.SecretManager = nil
		return
	}

	instance.Status.SecretManager = &litellmv1alpha1.SecretManagerStatus{
		Configured: true,
		Provider:   sm.Provider,
	}

	// Validate credentials Secret exists when referenced
	if sm.CredentialsSecretRef != nil {
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{
			Name: sm.CredentialsSecretRef.Name, Namespace: instance.Namespace,
		}, &secret); err != nil {
			instance.Status.SecretManager.Configured = false
			emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonSecretNotFound,
				"Secret manager credentials Secret %q not found", sm.CredentialsSecretRef.Name)
		}
	}
}

// validateTLSSecrets checks that the Secrets referenced by spec.tls and
// spec.database.tls exist and carry the expected keys. Problems are surfaced as
// warning events (mounting a missing/incomplete Secret would otherwise leave
// the pod stuck in ContainerCreating with no clear signal). Validation is
// non-fatal — it does not block the rest of the reconcile.
func (r *LiteLLMInstanceReconciler) validateTLSSecrets(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) {
	if instance.Spec.TLS != nil {
		tls := instance.Spec.TLS
		// Server cert and client cert are kubernetes.io/tls Secrets and must
		// carry BOTH tls.crt and tls.key.
		if tls.ServerCertSecretRef != nil {
			r.requireSecretKeys(ctx, instance, tls.ServerCertSecretRef.Name, "spec.tls.serverCertSecretRef", "tls.crt", "tls.key")
		}
		if tls.ClientCertSecretRef != nil {
			r.requireSecretKeys(ctx, instance, tls.ClientCertSecretRef.Name, "spec.tls.clientCertSecretRef", "tls.crt", "tls.key")
		}
		if tls.TrustedCASecretRef != nil {
			key := tls.TrustedCASecretRef.Key
			if key == "" {
				key = "ca.crt"
			}
			r.requireSecretKeys(ctx, instance, tls.TrustedCASecretRef.Name, "spec.tls.trustedCASecretRef", key)
		}
	}

	if instance.Spec.Database.TLS != nil {
		dbTLS := instance.Spec.Database.TLS
		if dbTLS.CASecretRef != nil {
			key := dbTLS.CASecretRef.Key
			if key == "" {
				key = "ca.crt"
			}
			r.requireSecretKeys(ctx, instance, dbTLS.CASecretRef.Name, "spec.database.tls.caSecretRef", key)
		}
		if dbTLS.ClientCertSecretRef != nil {
			r.requireSecretKeys(ctx, instance, dbTLS.ClientCertSecretRef.Name, "spec.database.tls.clientCertSecretRef", "tls.crt", "tls.key")
		}
	}
}

// requireSecretKeys emits a warning event if the named Secret is missing or is
// missing any of the required keys.
func (r *LiteLLMInstanceReconciler) requireSecretKeys(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, name, field string, keys ...string) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, &secret); err != nil {
		emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonSecretNotFound,
			"%s: Secret %q not found", field, name)
		return
	}
	for _, k := range keys {
		if _, ok := secret.Data[k]; !ok {
			emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonSecretKeyMissing,
				"%s: Secret %q is missing key %q", field, name, k)
		}
	}
}

// checkEnterpriseFeaturesWarning sets a warning condition and emits an event
// when enterprise-only features (JWT auth, OAuth2 auth) are configured but no
// LiteLLM license Secret is detected.
func (r *LiteLLMInstanceReconciler) checkEnterpriseFeaturesWarning(instance *litellmv1alpha1.LiteLLMInstance) {
	hasLicense := instance.Status.License != nil && instance.Status.License.Active

	var features []string
	if instance.Spec.JWTAuth != nil && instance.Spec.JWTAuth.Enabled {
		features = append(features, "JWT auth")
	}
	if instance.Spec.OAuth2Auth != nil && instance.Spec.OAuth2Auth.Enabled {
		features = append(features, "OAuth2 auth")
	}
	if instance.Spec.RBAC != nil && instance.Spec.RBAC.Enabled {
		if instance.Spec.RBAC.KeyGeneration != nil || len(instance.Spec.RBAC.RolePermissions) > 0 {
			features = append(features, "RBAC (key_generation_settings, role_permissions)")
		}
	}

	if len(features) > 0 && !hasLicense {
		msg := fmt.Sprintf("%s requires a LiteLLM Enterprise license", strings.Join(features, ", "))
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "EnterpriseFeaturesConfigured",
			Status:             metav1.ConditionTrue,
			Reason:             "EnterpriseFeaturesConfigured",
			Message:            msg,
			ObservedGeneration: instance.Generation,
		})
		emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonEnterpriseRequired, msg)
	} else {
		meta.RemoveStatusCondition(&instance.Status.Conditions, "EnterpriseFeaturesConfigured")
	}
}

// probeInstanceHealth calls /health/liveliness and /health/readiness on the
// running proxy, updates the Ready / RedisReady conditions, and emits
// Kubernetes Events on transitions. The master key is resolved the same way
// downstream controllers resolve it so the probe works with both auto-
// generated and user-provided keys.
func (r *LiteLLMInstanceReconciler) probeInstanceHealth(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance) {
	log := logf.FromContext(ctx)

	masterKeyRef := instance.Spec.MasterKey.SecretRef
	if masterKeyRef == nil && instance.Spec.MasterKey.AutoGenerate {
		masterKeyRef = &litellmv1alpha1.SecretKeyRef{
			Name: instance.Name + "-master-key",
			Key:  "master-key",
		}
	}
	if masterKeyRef == nil {
		return
	}
	masterKey, err := getSecretValue(ctx, r.Client, instance.Namespace, masterKeyRef)
	if err != nil {
		log.V(1).Info("health probe skipped, cannot resolve master key", "error", err)
		return
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api := r.LiteLLMClientFactory(instance.Status.Endpoint, masterKey)

	previouslyHealthy := meta.IsStatusConditionTrue(instance.Status.Conditions, ConditionReady)

	if err := api.Health().CheckLiveness(probeCtx); err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "LivenessProbeFailed",
			Message:            fmt.Sprintf("LiteLLM liveness probe failed: %v", err),
			ObservedGeneration: instance.Generation,
		})
		if previouslyHealthy {
			emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonHealthDegraded,
				"LiteLLM liveness probe failed: %v", err)
		}
		return
	}

	if _, err := api.Health().Readiness(probeCtx); err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ReadinessProbeFailed",
			Message:            fmt.Sprintf("LiteLLM readiness probe failed: %v", err),
			ObservedGeneration: instance.Generation,
		})
		if previouslyHealthy {
			emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonHealthDegraded,
				"LiteLLM readiness probe failed: %v", err)
		}
		return
	}

	if !previouslyHealthy {
		emitEvent(r.Recorder, instance, corev1.EventTypeNormal, EventReasonHealthRestored,
			"LiteLLM instance is healthy again")
	}

	// Redis status is probed separately: LiteLLM's /health/readiness does not
	// report Redis connectivity (see ReadinessResponse docs).
	r.probeRedisHealth(probeCtx, instance, api)
}

// probeRedisHealth sets the RedisReady condition and status.Redis.
//
// LiteLLM exposes no Redis-health field on /health/readiness. The only
// endpoint that genuinely tests Redis is GET /cache/ping, and it only works
// when response caching is backed by Redis. We therefore:
//
//   - skip entirely when Redis is not enabled (absent condition == "not applicable");
//   - call /cache/ping for a real connectivity verdict when caching is Redis-backed;
//   - otherwise (Redis wired only for router coordination) report it as
//     configured rather than emitting spurious "disconnected" warnings, because
//     LiteLLM surfaces no runtime signal for that usage.
func (r *LiteLLMInstanceReconciler) probeRedisHealth(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, api litellm.Client) {
	if instance.Spec.Redis == nil || !instance.Spec.Redis.Enabled {
		// Not applicable: clear any stale state.
		instance.Status.Redis = nil
		meta.RemoveStatusCondition(&instance.Status.Conditions, ConditionRedisReady)
		return
	}

	prevConnected := instance.Status.Redis != nil && instance.Status.Redis.Connected

	// When Redis is configured purely for router coordination (no Redis-backed
	// response cache), LiteLLM provides no endpoint to test the connection.
	// Report it as configured so the proxy being Ready is the implicit signal,
	// instead of perpetually claiming disconnection.
	if !isRedisBackedCaching(instance) {
		instance.Status.Redis = &litellmv1alpha1.RedisStatus{Connected: true}
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionRedisReady,
			Status:             metav1.ConditionTrue,
			Reason:             "RedisConfigured",
			Message:            "Redis is configured; enable Redis response caching to surface runtime connectivity checks",
			ObservedGeneration: instance.Generation,
		})
		return
	}

	// Redis-backed caching is enabled: /cache/ping actively pings Redis and
	// performs a test write, giving us a genuine connectivity verdict.
	ping, err := api.Health().CachePing(ctx)
	connected := err == nil && (ping.PingResponse || strings.EqualFold(ping.Status, "healthy"))

	instance.Status.Redis = &litellmv1alpha1.RedisStatus{Connected: connected}
	if connected {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionRedisReady,
			Status:             metav1.ConditionTrue,
			Reason:             "RedisConnected",
			Message:            "Redis connection is healthy",
			ObservedGeneration: instance.Generation,
		})
		if !prevConnected {
			emitEvent(r.Recorder, instance, corev1.EventTypeNormal, EventReasonRedisConnected,
				"Redis connection restored")
		}
		return
	}

	msg := "Redis connection is not healthy"
	if err != nil {
		msg = fmt.Sprintf("Redis cache ping failed: %v", err)
	}
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionRedisReady,
		Status:             metav1.ConditionFalse,
		Reason:             "RedisDisconnected",
		Message:            msg,
		ObservedGeneration: instance.Generation,
	})
	if prevConnected {
		emitEvent(r.Recorder, instance, corev1.EventTypeWarning, EventReasonRedisDisconnected,
			"Redis connection lost")
	}
}

// isRedisBackedCaching reports whether response caching is enabled with a
// Redis backend — the only configuration in which LiteLLM's /cache/ping can
// verify Redis connectivity.
func isRedisBackedCaching(instance *litellmv1alpha1.LiteLLMInstance) bool {
	c := instance.Spec.Caching
	if c == nil || !c.Enabled {
		return false
	}
	switch c.Type {
	// Empty defaults to "redis" (see CachingSpec.Type kubebuilder default).
	case "", "redis", "redis-semantic":
		return true
	default:
		return false
	}
}

// createOrUpdate creates a resource if it doesn't exist, or updates it if it does.
func (r *LiteLLMInstanceReconciler) createOrUpdate(ctx context.Context, desired client.Object, existing client.Object) error {
	key := types.NamespacedName{
		Name:      desired.GetName(),
		Namespace: desired.GetNamespace(),
	}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, desired)
}

func generateRandomToken(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// findInstanceForGuardrail maps a LiteLLMGuardrail event to the LiteLLMInstance
// it references, so guardrail CRUD triggers an instance reconcile that rewrites
// the ConfigMap and Deployment.
func (r *LiteLLMInstanceReconciler) findInstanceForGuardrail(_ context.Context, obj client.Object) []reconcile.Request {
	g, ok := obj.(*litellmv1alpha1.LiteLLMGuardrail)
	if !ok || g.Spec.InstanceRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      g.Spec.InstanceRef.Name,
			Namespace: g.Namespace,
		},
	}}
}

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMInstance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		// Watch license Secrets (not owned, so use Watches instead of Owns)
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findInstanceForLicenseSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		// Guardrails are config-level and must trigger an instance
		// reconcile whenever any CR changes so the `guardrails` config
		// section and guardrail env vars stay in sync with the Deployment.
		Watches(
			&litellmv1alpha1.LiteLLMGuardrail{},
			handler.EnqueueRequestsFromMapFunc(r.findInstanceForGuardrail),
		).
		Named("litellminstance").
		Complete(r)
}

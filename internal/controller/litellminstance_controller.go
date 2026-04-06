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
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/resources"
)

// LiteLLMInstanceReconciler reconciles a LiteLLMInstance object.
type LiteLLMInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.palena.ai,resources=litellminstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;services;secrets;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

func (r *LiteLLMInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

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
	var reconcileErr error

	// 1. Reconcile auto-generated secrets
	if err := r.reconcileSecrets(ctx, &instance); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile secrets")
	}

	// 2. ConfigMap
	if err := r.reconcileConfigMap(ctx, &instance, labels); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile ConfigMap")
	}

	// 3. ServiceAccount
	if err := r.reconcileServiceAccount(ctx, &instance, labels); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile ServiceAccount")
	}

	// 4. Database migration job (if enabled)
	// Migration is best-effort and must never block Deployment creation.
	if instance.Spec.Database.Migration != nil && instance.Spec.Database.Migration.Enabled {
		migrationDone, err := r.reconcileMigrationJob(ctx, &instance, labels)
		if err != nil {
			log.Error(err, "failed to reconcile migration Job")
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:               ConditionDatabaseReady,
				Status:             metav1.ConditionFalse,
				Reason:             "MigrationFailed",
				Message:            err.Error(),
				ObservedGeneration: instance.Generation,
			})
		} else if migrationDone {
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
	} else {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionDatabaseReady,
			Status:             metav1.ConditionTrue,
			Reason:             "MigrationSkipped",
			Message:            "Database migration not enabled; LiteLLM handles migrations on startup",
			ObservedGeneration: instance.Generation,
		})
	}

	// 5. Deployment
	if err := r.reconcileDeployment(ctx, &instance, labels); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile Deployment")
	}

	// 6. Service
	if err := r.reconcileService(ctx, &instance, labels); err != nil {
		reconcileErr = err
		log.Error(err, "failed to reconcile Service")
	}

	// 7. Ingress (conditional)
	if instance.Spec.Ingress != nil && instance.Spec.Ingress.Enabled {
		if err := r.reconcileIngress(ctx, &instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile Ingress")
		}
	}

	// 8. OpenShift Route (conditional)
	if instance.Spec.Route != nil && instance.Spec.Route.Enabled {
		if err := r.reconcileRoute(ctx, &instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile Route")
		}
	}

	// 9. Gateway API HTTPRoute (conditional)
	if instance.Spec.GatewayHTTPRoute != nil && instance.Spec.GatewayHTTPRoute.Enabled {
		if err := r.reconcileHTTPRoute(ctx, &instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile HTTPRoute")
		}
	}

	// 10. HPA (conditional)
	if instance.Spec.Autoscaling != nil && instance.Spec.Autoscaling.Enabled {
		if err := r.reconcileHPA(ctx, &instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile HPA")
		}
	}

	// 11. PDB (conditional)
	if instance.Spec.PodDisruptionBudget != nil && instance.Spec.PodDisruptionBudget.Enabled {
		if err := r.reconcilePDB(ctx, &instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile PDB")
		}
	}

	// 12. NetworkPolicy (conditional)
	if instance.Spec.Security != nil && instance.Spec.Security.NetworkPolicy != nil && instance.Spec.Security.NetworkPolicy.Enabled {
		if err := r.reconcileNetworkPolicy(ctx, &instance, labels); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile NetworkPolicy")
		}
	}

	// 13. SCIM token
	if instance.Spec.SCIM != nil && instance.Spec.SCIM.Enabled {
		if err := r.reconcileSCIMToken(ctx, &instance); err != nil {
			reconcileErr = err
			log.Error(err, "failed to reconcile SCIM token")
		}
	}

	// Update status
	r.updateInstanceStatus(ctx, &instance, reconcileErr)

	if reconcileErr != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
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

func (r *LiteLLMInstanceReconciler) reconcileConfigMap(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired, err := resources.BuildConfigMap(instance, labels)
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

func (r *LiteLLMInstanceReconciler) reconcileDeployment(ctx context.Context, instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) error {
	desired := resources.BuildDeployment(instance, labels)
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
		instance.Status.Version = "main-latest"
	}

	// SSO status
	if instance.Spec.SSO != nil && instance.Spec.SSO.Enabled {
		instance.Status.SSO = &litellmv1alpha1.SSOStatus{
			Configured: true,
			Provider:   instance.Spec.SSO.Provider,
		}
	}

	// Ready condition
	if reconcileErr != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ReconcileError",
			Message:            reconcileErr.Error(),
			ObservedGeneration: instance.Generation,
		})
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

	_ = r.Status().Update(ctx, instance)
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

// SetupWithManager sets up the controller with the Manager.
func (r *LiteLLMInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMInstance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		Named("litellminstance").
		Complete(r)
}

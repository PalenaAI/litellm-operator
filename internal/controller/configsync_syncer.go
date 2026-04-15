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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
	"github.com/PalenaAI/litellm-operator/internal/litellm"
)

// syncResult holds the outcome of a config sync cycle.
type syncResult struct {
	SyncedModels           int
	SyncedTeams            int
	SyncedUsers            int
	SyncedKeys             int
	SyncedOrganizations    int
	SyncedCustomers        int
	UnmanagedModels        int
	UnmanagedTeams         int
	UnmanagedUsers         int
	UnmanagedKeys          int
	UnmanagedOrganizations int
	UnmanagedCustomers     int
	DriftDetected          int
	PrunedResources        int
	Errors                 []string
}

func (r *syncResult) addError(msg string) {
	r.Errors = append(r.Errors, msg)
}

// configSyncer performs bidirectional config sync for a single LiteLLMInstance.
// It compares CRD state with LiteLLM API state, detects drift, handles
// unmanaged resources, and reports status.
type configSyncer struct {
	kClient   client.Client
	apiClient litellm.Client
	recorder  record.EventRecorder
	instance  *litellmv1alpha1.LiteLLMInstance
	config    *litellmv1alpha1.ConfigSyncSpec
}

// sync runs a full config sync cycle across all resource types.
func (s *configSyncer) sync(ctx context.Context) *syncResult {
	result := &syncResult{}
	s.syncModels(ctx, result)
	s.syncTeams(ctx, result)
	s.syncUsers(ctx, result)
	s.syncKeys(ctx, result)
	s.syncOrganizations(ctx, result)
	s.syncCustomers(ctx, result)
	return result
}

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

func (s *configSyncer) syncModels(ctx context.Context, result *syncResult) {
	log := logf.FromContext(ctx).WithValues("resource", "models")

	var crdList litellmv1alpha1.LiteLLMModelList
	if err := s.kClient.List(ctx, &crdList, client.InNamespace(s.instance.Namespace)); err != nil {
		log.Error(err, "failed to list model CRDs")
		result.addError(fmt.Sprintf("list model CRDs: %v", err))
		return
	}

	managedIDs := make(map[string]*litellmv1alpha1.LiteLLMModel)
	for i := range crdList.Items {
		m := &crdList.Items[i]
		if m.Spec.InstanceRef.Name != s.instance.Name || !m.DeletionTimestamp.IsZero() {
			continue
		}
		if m.Status.LiteLLMModelID != "" {
			managedIDs[m.Status.LiteLLMModelID] = m
		}
	}

	apiModels, err := s.apiClient.Models().List(ctx)
	if err != nil {
		log.Error(err, "failed to list API models")
		result.addError(fmt.Sprintf("list API models: %v", err))
		return
	}

	seenIDs := make(map[string]bool)
	for _, apiModel := range apiModels {
		if crd, ok := managedIDs[apiModel.ModelID]; ok {
			result.SyncedModels++
			seenIDs[apiModel.ModelID] = true
			if s.isModelDrifted(crd, &apiModel) {
				s.handleDrift(ctx, crd, "model", crd.Spec.ModelName, result)
			}
		} else {
			result.UnmanagedModels++
			s.handleUnmanaged(ctx, "model", apiModel.ModelName, result, func() error {
				return s.apiClient.Models().Delete(ctx, apiModel.ModelID)
			})
		}
	}

	for apiID, crd := range managedIDs {
		if !seenIDs[apiID] {
			log.Info("managed model missing from API", "model", crd.Spec.ModelName, "apiId", apiID)
			s.handleDeletedFromAPI(ctx, crd, "model", crd.Spec.ModelName, result, func() {
				crd.Status.LiteLLMModelID = ""
				crd.Status.Synced = false
			})
		}
	}
}

func (s *configSyncer) isModelDrifted(crd *litellmv1alpha1.LiteLLMModel, api *litellm.ModelInfoResponse) bool {
	if crd.Spec.ModelName != api.ModelName {
		return true
	}
	if crd.Spec.LiteLLMParams.Model != api.Params.Model {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Teams
// ---------------------------------------------------------------------------

func (s *configSyncer) syncTeams(ctx context.Context, result *syncResult) {
	log := logf.FromContext(ctx).WithValues("resource", "teams")

	var crdList litellmv1alpha1.LiteLLMTeamList
	if err := s.kClient.List(ctx, &crdList, client.InNamespace(s.instance.Namespace)); err != nil {
		log.Error(err, "failed to list team CRDs")
		result.addError(fmt.Sprintf("list team CRDs: %v", err))
		return
	}

	managedIDs := make(map[string]*litellmv1alpha1.LiteLLMTeam)
	for i := range crdList.Items {
		t := &crdList.Items[i]
		if t.Spec.InstanceRef.Name != s.instance.Name || !t.DeletionTimestamp.IsZero() {
			continue
		}
		if t.Status.LiteLLMTeamID != "" {
			managedIDs[t.Status.LiteLLMTeamID] = t
		}
	}

	apiTeams, err := s.apiClient.Teams().List(ctx)
	if err != nil {
		log.Error(err, "failed to list API teams")
		result.addError(fmt.Sprintf("list API teams: %v", err))
		return
	}

	seenIDs := make(map[string]bool)
	for _, apiTeam := range apiTeams {
		if crd, ok := managedIDs[apiTeam.TeamID]; ok {
			result.SyncedTeams++
			seenIDs[apiTeam.TeamID] = true
			if s.isTeamDrifted(crd, &apiTeam) {
				s.handleDrift(ctx, crd, "team", crd.Spec.TeamAlias, result)
			}
		} else {
			result.UnmanagedTeams++
			teamID := apiTeam.TeamID
			s.handleUnmanaged(ctx, "team", apiTeam.TeamAlias, result, func() error {
				return s.apiClient.Teams().Delete(ctx, teamID)
			})
		}
	}

	for apiID, crd := range managedIDs {
		if !seenIDs[apiID] {
			log.Info("managed team missing from API", "team", crd.Spec.TeamAlias, "apiId", apiID)
			s.handleDeletedFromAPI(ctx, crd, "team", crd.Spec.TeamAlias, result, func() {
				crd.Status.LiteLLMTeamID = ""
				crd.Status.Synced = false
			})
		}
	}
}

func (s *configSyncer) isTeamDrifted(crd *litellmv1alpha1.LiteLLMTeam, api *litellm.TeamInfo) bool {
	return crd.Spec.TeamAlias != api.TeamAlias
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *configSyncer) syncUsers(ctx context.Context, result *syncResult) {
	log := logf.FromContext(ctx).WithValues("resource", "users")

	var crdList litellmv1alpha1.LiteLLMUserList
	if err := s.kClient.List(ctx, &crdList, client.InNamespace(s.instance.Namespace)); err != nil {
		log.Error(err, "failed to list user CRDs")
		result.addError(fmt.Sprintf("list user CRDs: %v", err))
		return
	}

	managedIDs := make(map[string]*litellmv1alpha1.LiteLLMUser)
	for i := range crdList.Items {
		u := &crdList.Items[i]
		if u.Spec.InstanceRef.Name != s.instance.Name || !u.DeletionTimestamp.IsZero() {
			continue
		}
		if u.Status.LiteLLMUserID != "" {
			managedIDs[u.Status.LiteLLMUserID] = u
		}
	}

	apiUsers, err := s.apiClient.Users().List(ctx)
	if err != nil {
		log.Error(err, "failed to list API users")
		result.addError(fmt.Sprintf("list API users: %v", err))
		return
	}

	seenIDs := make(map[string]bool)
	for _, apiUser := range apiUsers {
		if crd, ok := managedIDs[apiUser.UserID]; ok {
			result.SyncedUsers++
			seenIDs[apiUser.UserID] = true
			if s.isUserDrifted(crd, &apiUser) {
				s.handleDrift(ctx, crd, "user", crd.Spec.UserID, result)
			}
		} else {
			result.UnmanagedUsers++
			userID := apiUser.UserID
			s.handleUnmanaged(ctx, "user", apiUser.UserEmail, result, func() error {
				return s.apiClient.Users().Delete(ctx, userID)
			})
		}
	}

	for apiID, crd := range managedIDs {
		if !seenIDs[apiID] {
			log.Info("managed user missing from API", "user", crd.Spec.UserID, "apiId", apiID)
			s.handleDeletedFromAPI(ctx, crd, "user", crd.Spec.UserID, result, func() {
				crd.Status.LiteLLMUserID = ""
				crd.Status.Synced = false
			})
		}
	}
}

func (s *configSyncer) isUserDrifted(crd *litellmv1alpha1.LiteLLMUser, api *litellm.UserInfo) bool {
	if crd.Spec.UserEmail != api.UserEmail {
		return true
	}
	if crd.Spec.UserRole != api.UserRole {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Keys
// ---------------------------------------------------------------------------

func (s *configSyncer) syncKeys(ctx context.Context, result *syncResult) {
	log := logf.FromContext(ctx).WithValues("resource", "keys")

	var crdList litellmv1alpha1.LiteLLMVirtualKeyList
	if err := s.kClient.List(ctx, &crdList, client.InNamespace(s.instance.Namespace)); err != nil {
		log.Error(err, "failed to list virtual key CRDs")
		result.addError(fmt.Sprintf("list key CRDs: %v", err))
		return
	}

	managedTokens := make(map[string]*litellmv1alpha1.LiteLLMVirtualKey)
	for i := range crdList.Items {
		k := &crdList.Items[i]
		if k.Spec.InstanceRef.Name != s.instance.Name || !k.DeletionTimestamp.IsZero() {
			continue
		}
		if k.Status.LiteLLMKeyToken != "" {
			managedTokens[k.Status.LiteLLMKeyToken] = k
		}
	}

	apiKeys, err := s.apiClient.Keys().List(ctx)
	if err != nil {
		log.Error(err, "failed to list API keys")
		result.addError(fmt.Sprintf("list API keys: %v", err))
		return
	}

	seenTokens := make(map[string]bool)
	for _, apiKey := range apiKeys {
		if crd, ok := managedTokens[apiKey.Token]; ok {
			result.SyncedKeys++
			seenTokens[apiKey.Token] = true
			if s.isKeyDrifted(crd, &apiKey) {
				s.handleDrift(ctx, crd, "key", crd.Spec.KeyAlias, result)
			}
		} else {
			result.UnmanagedKeys++
			token := apiKey.Token
			s.handleUnmanaged(ctx, "key", apiKey.KeyAlias, result, func() error {
				return s.apiClient.Keys().Delete(ctx, token)
			})
		}
	}

	for token, crd := range managedTokens {
		if !seenTokens[token] {
			log.Info("managed key missing from API", "key", crd.Spec.KeyAlias, "token", token)
			s.handleDeletedFromAPI(ctx, crd, "key", crd.Spec.KeyAlias, result, func() {
				crd.Status.LiteLLMKeyToken = ""
				crd.Status.Synced = false
			})
		}
	}
}

func (s *configSyncer) isKeyDrifted(crd *litellmv1alpha1.LiteLLMVirtualKey, api *litellm.KeyInfo) bool {
	return crd.Spec.KeyAlias != api.KeyAlias
}

// ---------------------------------------------------------------------------
// Organizations
// ---------------------------------------------------------------------------

func (s *configSyncer) syncOrganizations(ctx context.Context, result *syncResult) {
	log := logf.FromContext(ctx).WithValues("resource", "organizations")

	var crdList litellmv1alpha1.LiteLLMOrganizationList
	if err := s.kClient.List(ctx, &crdList, client.InNamespace(s.instance.Namespace)); err != nil {
		log.Error(err, "failed to list organization CRDs")
		result.addError(fmt.Sprintf("list organization CRDs: %v", err))
		return
	}

	managedIDs := make(map[string]*litellmv1alpha1.LiteLLMOrganization)
	for i := range crdList.Items {
		o := &crdList.Items[i]
		if o.Spec.InstanceRef.Name != s.instance.Name || !o.DeletionTimestamp.IsZero() {
			continue
		}
		if o.Status.LiteLLMOrganizationID != "" {
			managedIDs[o.Status.LiteLLMOrganizationID] = o
		}
	}

	apiOrgs, err := s.apiClient.Organizations().List(ctx)
	if err != nil {
		log.Error(err, "failed to list API organizations")
		result.addError(fmt.Sprintf("list API organizations: %v", err))
		return
	}

	seenIDs := make(map[string]bool)
	for _, apiOrg := range apiOrgs {
		if crd, ok := managedIDs[apiOrg.OrganizationID]; ok {
			result.SyncedOrganizations++
			seenIDs[apiOrg.OrganizationID] = true
			if s.isOrganizationDrifted(crd, &apiOrg) {
				s.handleDrift(ctx, crd, "organization", crd.Spec.OrganizationAlias, result)
			}
		} else {
			result.UnmanagedOrganizations++
			orgID := apiOrg.OrganizationID
			s.handleUnmanaged(ctx, "organization", apiOrg.OrganizationAlias, result, func() error {
				return s.apiClient.Organizations().Delete(ctx, orgID)
			})
		}
	}

	for apiID, crd := range managedIDs {
		if !seenIDs[apiID] {
			log.Info("managed organization missing from API", "org", crd.Spec.OrganizationAlias, "apiId", apiID)
			s.handleDeletedFromAPI(ctx, crd, "organization", crd.Spec.OrganizationAlias, result, func() {
				crd.Status.LiteLLMOrganizationID = ""
				crd.Status.Synced = false
			})
		}
	}
}

func (s *configSyncer) isOrganizationDrifted(crd *litellmv1alpha1.LiteLLMOrganization, api *litellm.OrganizationInfo) bool {
	return crd.Spec.OrganizationAlias != api.OrganizationAlias
}

// ---------------------------------------------------------------------------
// Customers
// ---------------------------------------------------------------------------

func (s *configSyncer) syncCustomers(ctx context.Context, result *syncResult) {
	log := logf.FromContext(ctx).WithValues("resource", "customers")

	var crdList litellmv1alpha1.LiteLLMCustomerList
	if err := s.kClient.List(ctx, &crdList, client.InNamespace(s.instance.Namespace)); err != nil {
		log.Error(err, "failed to list customer CRDs")
		result.addError(fmt.Sprintf("list customer CRDs: %v", err))
		return
	}

	// Customers use spec.customerId as the API identifier (no generated ID).
	managedIDs := make(map[string]*litellmv1alpha1.LiteLLMCustomer)
	for i := range crdList.Items {
		c := &crdList.Items[i]
		if c.Spec.InstanceRef.Name != s.instance.Name || !c.DeletionTimestamp.IsZero() {
			continue
		}
		if c.Status.Synced {
			managedIDs[c.Spec.CustomerID] = c
		}
	}

	apiCustomers, err := s.apiClient.Customers().List(ctx)
	if err != nil {
		log.Error(err, "failed to list API customers")
		result.addError(fmt.Sprintf("list API customers: %v", err))
		return
	}

	seenIDs := make(map[string]bool)
	for _, apiCust := range apiCustomers {
		if crd, ok := managedIDs[apiCust.UserID]; ok {
			result.SyncedCustomers++
			seenIDs[apiCust.UserID] = true
			if s.isCustomerDrifted(crd, &apiCust) {
				s.handleDrift(ctx, crd, "customer", crd.Spec.CustomerID, result)
			}
		} else {
			result.UnmanagedCustomers++
			custID := apiCust.UserID
			s.handleUnmanaged(ctx, "customer", apiCust.Alias, result, func() error {
				return s.apiClient.Customers().Delete(ctx, custID)
			})
		}
	}

	for custID, crd := range managedIDs {
		if !seenIDs[custID] {
			log.Info("managed customer missing from API", "customer", crd.Spec.CustomerID)
			s.handleDeletedFromAPI(ctx, crd, "customer", crd.Spec.CustomerID, result, func() {
				crd.Status.Synced = false
			})
		}
	}
}

func (s *configSyncer) isCustomerDrifted(crd *litellmv1alpha1.LiteLLMCustomer, api *litellm.CustomerInfo) bool {
	if crd.Spec.Alias != api.Alias {
		return true
	}
	if crd.Spec.Blocked != nil && *crd.Spec.Blocked != api.Blocked {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// handleDrift handles a managed resource whose API state differs from the CRD spec.
func (s *configSyncer) handleDrift(ctx context.Context, obj client.Object, resourceType, displayName string, result *syncResult) {
	log := logf.FromContext(ctx)
	result.DriftDetected++

	switch s.config.ConflictResolution {
	case "crd-wins", "":
		// Clear the sync hash annotation so the per-resource controller
		// detects a "changed" spec and re-pushes CRD state to the API.
		annotations := obj.GetAnnotations()
		if annotations != nil {
			delete(annotations, AnnotationSyncHash)
			obj.SetAnnotations(annotations)
			if err := s.kClient.Update(ctx, obj); err != nil {
				log.Error(err, "failed to clear sync hash for drift remediation", "type", resourceType, "name", displayName)
				result.addError(fmt.Sprintf("clear sync hash for %s %q: %v", resourceType, displayName, err))
				return
			}
		}
		s.emitAuditEvent(EventReasonConfigSyncDriftRemediated,
			"Drift detected on %s %q, triggered re-sync (crd-wins)", resourceType, displayName)

	case "api-wins":
		s.emitAuditEvent(EventReasonConfigSyncDriftDetected,
			"Drift detected on %s %q, API state preserved (api-wins)", resourceType, displayName)

	case "manual":
		s.emitAuditEvent(EventReasonConfigSyncDriftDetected,
			"Drift detected on %s %q, manual resolution required", resourceType, displayName)
	}
}

// handleDeletedFromAPI handles a CRD whose corresponding API resource was
// deleted externally. clearStatusFn should zero the status ID so the
// per-resource controller recreates the resource.
func (s *configSyncer) handleDeletedFromAPI(
	ctx context.Context,
	obj client.Object,
	resourceType, displayName string,
	result *syncResult,
	clearStatusFn func(),
) {
	log := logf.FromContext(ctx)
	result.DriftDetected++

	switch s.config.ConflictResolution {
	case "crd-wins", "":
		// Clear the status ID and sync hash so the per-resource controller
		// sees a fresh resource and recreates it in the API.
		clearStatusFn()
		if err := s.kClient.Status().Update(ctx, obj); err != nil {
			log.Error(err, "failed to clear status for re-creation", "type", resourceType, "name", displayName)
			result.addError(fmt.Sprintf("clear status for %s %q: %v", resourceType, displayName, err))
			return
		}
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		delete(annotations, AnnotationSyncHash)
		obj.SetAnnotations(annotations)
		if err := s.kClient.Update(ctx, obj); err != nil {
			log.Error(err, "failed to clear sync hash for re-creation", "type", resourceType, "name", displayName)
			result.addError(fmt.Sprintf("clear hash for %s %q: %v", resourceType, displayName, err))
			return
		}
		s.emitAuditEvent(EventReasonConfigSyncRecreated,
			"API resource for %s %q was deleted externally, triggered recreation (crd-wins)", resourceType, displayName)

	case "api-wins":
		s.emitAuditEvent(EventReasonConfigSyncDriftDetected,
			"API resource for %s %q was deleted externally, API state accepted (api-wins)", resourceType, displayName)

	case "manual":
		s.emitAuditEvent(EventReasonConfigSyncDriftDetected,
			"API resource for %s %q is missing from API, manual resolution required", resourceType, displayName)
	}
}

// handleUnmanaged handles an API resource that has no corresponding CRD.
func (s *configSyncer) handleUnmanaged(
	ctx context.Context,
	resourceType, displayName string,
	result *syncResult,
	deleteFn func() error,
) {
	log := logf.FromContext(ctx)

	switch s.config.UnmanagedResourcePolicy {
	case "prune":
		if err := deleteFn(); err != nil {
			log.Error(err, "failed to prune unmanaged resource", "type", resourceType, "name", displayName)
			result.addError(fmt.Sprintf("prune %s %q: %v", resourceType, displayName, err))
		} else {
			result.PrunedResources++
			s.emitAuditEvent(EventReasonConfigSyncPruned,
				"Pruned unmanaged %s %q", resourceType, displayName)
		}

	case "adopt":
		s.emitAuditEvent(EventReasonConfigSyncUnmanaged,
			"Detected unmanaged %s %q (adopt policy — reporting only)", resourceType, displayName)

	case "preserve", "":
		// Default: leave unmanaged resources alone, no event.
	}
}

// emitAuditEvent emits a Kubernetes Event on the instance if audit changes
// are enabled, or always for significant actions like prune/recreate.
func (s *configSyncer) emitAuditEvent(reason, messageFmt string, args ...interface{}) {
	if !s.config.AuditChanges {
		// Always emit for prune and recreate even when audit is off.
		switch reason {
		case EventReasonConfigSyncPruned, EventReasonConfigSyncRecreated:
			// fall through
		default:
			return
		}
	}
	emitEvent(s.recorder, s.instance, corev1.EventTypeNormal, reason, messageFmt, args...)
}

// instanceAsRuntimeObject returns the instance cast to runtime.Object.
// This is a compile-time assertion that LiteLLMInstance implements runtime.Object.
var _ runtime.Object = (*litellmv1alpha1.LiteLLMInstance)(nil)

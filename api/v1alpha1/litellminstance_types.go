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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMInstanceSpec defines the desired state of LiteLLMInstance.
type LiteLLMInstanceSpec struct {
	// Image configuration for the LiteLLM proxy.
	Image ImageSpec `json:"image,omitempty"`

	// Number of LiteLLM proxy replicas.
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// Autoscaling configuration.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Master key configuration for LiteLLM admin access.
	MasterKey MasterKeySpec `json:"masterKey"`

	// Database configuration for LiteLLM state storage.
	Database DatabaseSpec `json:"database"`

	// Redis configuration for caching and routing.
	// +optional
	Redis *RedisSpec `json:"redis,omitempty"`

	// Salt key for hashing.
	// +optional
	SaltKey *SaltKeySpec `json:"saltKey,omitempty"`

	// General settings for the LiteLLM proxy.
	// +optional
	GeneralSettings *GeneralSettingsSpec `json:"generalSettings,omitempty"`

	// Router settings for model routing.
	// +optional
	RouterSettings *RouterSettingsSpec `json:"routerSettings,omitempty"`

	// Config sync settings for bidirectional synchronization.
	// +optional
	ConfigSync *ConfigSyncSpec `json:"configSync,omitempty"`

	// Service configuration.
	Service ServiceSpec `json:"service,omitempty"`

	// Ingress configuration.
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`

	// OpenShift Route configuration.
	// +optional
	Route *RouteSpec `json:"route,omitempty"`

	// Gateway API HTTPRoute configuration.
	// +optional
	GatewayHTTPRoute *GatewayHTTPRouteSpec `json:"gatewayHTTPRoute,omitempty"`

	// Security settings.
	// +optional
	Security *SecuritySpec `json:"security,omitempty"`

	// Health check configuration.
	// +optional
	HealthCheck *HealthCheckSpec `json:"healthCheck,omitempty"`

	// Resource requirements for the LiteLLM container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Pod disruption budget configuration.
	// +optional
	PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`

	// Topology spread constraints.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Extra environment variables for the LiteLLM container.
	// +optional
	ExtraEnvVars []corev1.EnvVar `json:"extraEnvVars,omitempty"`

	// Extra environment variable sources.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`

	// Callback configuration.
	// +optional
	Callbacks *CallbacksSpec `json:"callbacks,omitempty"`

	// Observability configuration.
	// +optional
	Observability *ObservabilitySpec `json:"observability,omitempty"`

	// Upgrade strategy configuration.
	// +optional
	Upgrade *UpgradeSpec `json:"upgrade,omitempty"`

	// SSO configuration.
	// +optional
	SSO *SSOSpec `json:"sso,omitempty"`

	// SCIM v2 provisioning configuration.
	// +optional
	SCIM *SCIMSpec `json:"scim,omitempty"`
}

// ImageSpec defines the container image for LiteLLM.
type ImageSpec struct {
	// Container image repository.
	// +kubebuilder:default="ghcr.io/berriai/litellm"
	Repository string `json:"repository,omitempty"`

	// Container image tag.
	// +kubebuilder:default="main-latest"
	Tag string `json:"tag,omitempty"`

	// Image pull policy.
	// +kubebuilder:default="IfNotPresent"
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// Image pull secrets.
	// +optional
	PullSecrets []SecretRef `json:"pullSecrets,omitempty"`
}

// AutoscalingSpec defines horizontal pod autoscaling settings.
type AutoscalingSpec struct {
	// Enable autoscaling.
	Enabled bool `json:"enabled"`

	// Minimum number of replicas.
	// +kubebuilder:default=1
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// Maximum number of replicas.
	MaxReplicas int32 `json:"maxReplicas"`

	// Target CPU utilization percentage.
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`

	// Target memory utilization percentage.
	// +optional
	TargetMemoryUtilization *int32 `json:"targetMemoryUtilization,omitempty"`
}

// MasterKeySpec defines master key configuration.
type MasterKeySpec struct {
	// Reference to Secret containing the master key.
	// +optional
	SecretRef *SecretKeyRef `json:"secretRef,omitempty"`

	// Auto-generate a master key and store it in a Secret.
	// +optional
	AutoGenerate bool `json:"autoGenerate,omitempty"`
}

// DatabaseSpec defines database configuration.
type DatabaseSpec struct {
	// CloudNativePG managed database.
	// +optional
	CloudNativePG *CloudNativePGSpec `json:"cloudnativepg,omitempty"`

	// External database connection.
	// +optional
	External *ExternalDBSpec `json:"external,omitempty"`

	// Operator-managed PostgreSQL (simple single-pod deployment).
	// +optional
	Managed *ManagedDBSpec `json:"managed,omitempty"`

	// Connection pool settings.
	// +optional
	ConnectionPool *ConnectionPoolSpec `json:"connectionPool,omitempty"`

	// Migration settings.
	// +optional
	Migration *MigrationSpec `json:"migration,omitempty"`
}

// CloudNativePGSpec defines CloudNativePG configuration.
type CloudNativePGSpec struct {
	// Name of the CloudNativePG Cluster CR.
	ClusterName string `json:"clusterName"`

	// Backup configuration using CloudNativePG ScheduledBackup.
	// Requires CloudNativePG operator to be installed.
	// +optional
	Backup *CNPGBackupSpec `json:"backup,omitempty"`
}

// CNPGBackupSpec defines backup configuration via CloudNativePG.
type CNPGBackupSpec struct {
	// Enable scheduled backups.
	Enabled bool `json:"enabled"`

	// Cron schedule for backups (e.g., "0 0 * * *" for daily at midnight).
	// +kubebuilder:default="0 0 * * *"
	Schedule string `json:"schedule,omitempty"`

	// Number of backups to retain.
	// +kubebuilder:default=7
	// +kubebuilder:validation:Minimum=1
	Retention int `json:"retention,omitempty"`

	// Suspend scheduled backups without deleting the schedule.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// Backup method: snapshot or barmanObjectStore.
	// +kubebuilder:validation:Enum=snapshot;barmanObjectStore
	// +kubebuilder:default="snapshot"
	Method string `json:"method,omitempty"`
}

// ExternalDBSpec defines external database configuration.
type ExternalDBSpec struct {
	// Reference to Secret containing the database connection URL.
	ConnectionSecretRef SecretKeyRef `json:"connectionSecretRef"`
}

// ManagedDBSpec defines operator-managed database.
type ManagedDBSpec struct {
	// Enable operator-managed PostgreSQL.
	Enabled bool `json:"enabled"`

	// Storage size for the database PVC.
	// +kubebuilder:default="10Gi"
	StorageSize string `json:"storageSize,omitempty"`

	// Storage class name.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// ConnectionPoolSpec defines database connection pool settings.
type ConnectionPoolSpec struct {
	// Maximum number of connections.
	// +kubebuilder:default=10
	MaxConnections int `json:"maxConnections,omitempty"`
}

// MigrationSpec defines database migration settings.
type MigrationSpec struct {
	// Run database migration before starting.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Timeout for migration job.
	// +kubebuilder:default="300s"
	Timeout string `json:"timeout,omitempty"`
}

// RedisSpec defines Redis configuration.
type RedisSpec struct {
	// Enable Redis.
	Enabled bool `json:"enabled"`

	// Redis connection URL Secret reference.
	// +optional
	ConnectionSecretRef *SecretKeyRef `json:"connectionSecretRef,omitempty"`

	// Redis host.
	// +optional
	Host string `json:"host,omitempty"`

	// Redis port.
	// +kubebuilder:default=6379
	Port int `json:"port,omitempty"`

	// Redis password Secret reference.
	// +optional
	PasswordSecretRef *SecretKeyRef `json:"passwordSecretRef,omitempty"`
}

// SaltKeySpec defines salt key configuration.
type SaltKeySpec struct {
	// Reference to Secret containing the salt key.
	// +optional
	SecretRef *SecretKeyRef `json:"secretRef,omitempty"`

	// Auto-generate a salt key.
	// +optional
	AutoGenerate bool `json:"autoGenerate,omitempty"`
}

// GeneralSettingsSpec defines LiteLLM general settings.
type GeneralSettingsSpec struct {
	// Batch write interval in seconds.
	// +optional
	ProxyBatchWriteAt int `json:"proxyBatchWriteAt,omitempty"`

	// Enable/disable master key requirement.
	// +optional
	MasterKeyRequired *bool `json:"masterKeyRequired,omitempty"`

	// Alert types for notifications.
	// +optional
	AlertTypes []string `json:"alertTypes,omitempty"`

	// Custom key generation function.
	// +optional
	CustomKeyGenerate string `json:"customKeyGenerate,omitempty"`

	// Allow requests with no key.
	// +optional
	AllowUserAuth *bool `json:"allowUserAuth,omitempty"`
}

// RouterSettingsSpec defines LiteLLM router settings.
type RouterSettingsSpec struct {
	// Routing strategy.
	// +kubebuilder:validation:Enum=simple-shuffle;least-busy;latency-based-routing;usage-based-routing
	// +optional
	RoutingStrategy string `json:"routingStrategy,omitempty"`

	// Number of retries.
	// +optional
	NumRetries *int `json:"numRetries,omitempty"`

	// Timeout in seconds.
	// +optional
	Timeout *int `json:"timeout,omitempty"`

	// Retry after seconds.
	// +optional
	RetryAfter *int `json:"retryAfter,omitempty"`

	// Allowed fails before cooldown.
	// +optional
	AllowedFails *int `json:"allowedFails,omitempty"`

	// Cooldown time in seconds.
	// +optional
	CooldownTime *int `json:"cooldownTime,omitempty"`
}

// ConfigSyncSpec defines bidirectional config sync settings.
type ConfigSyncSpec struct {
	// Enable config sync.
	Enabled bool `json:"enabled"`

	// Sync interval (e.g., "30s", "1m").
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`

	// Policy for resources found in API but not in CRDs.
	// +kubebuilder:validation:Enum=preserve;prune;adopt
	// +kubebuilder:default="preserve"
	UnmanagedResourcePolicy string `json:"unmanagedResourcePolicy,omitempty"`

	// Conflict resolution strategy.
	// +kubebuilder:validation:Enum=crd-wins;api-wins;manual
	// +kubebuilder:default="crd-wins"
	ConflictResolution string `json:"conflictResolution,omitempty"`

	// Log config sync changes as events.
	// +optional
	AuditChanges bool `json:"auditChanges,omitempty"`
}

// ServiceSpec defines Kubernetes Service configuration.
type ServiceSpec struct {
	// Service type.
	// +kubebuilder:default="ClusterIP"
	Type corev1.ServiceType `json:"type,omitempty"`

	// Service port.
	// +kubebuilder:default=4000
	Port int32 `json:"port,omitempty"`
}

// IngressSpec defines Ingress configuration.
type IngressSpec struct {
	// Enable Ingress.
	Enabled bool `json:"enabled"`

	// Ingress class name.
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Hostname for the Ingress.
	// +optional
	Host string `json:"host,omitempty"`

	// TLS configuration.
	// +optional
	TLS *IngressTLSSpec `json:"tls,omitempty"`

	// Annotations for the Ingress.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressTLSSpec defines Ingress TLS configuration.
type IngressTLSSpec struct {
	// Enable TLS.
	Enabled bool `json:"enabled"`

	// Secret name containing TLS certificate.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// RouteSpec defines OpenShift Route configuration.
type RouteSpec struct {
	// Enable Route.
	Enabled bool `json:"enabled"`

	// Hostname for the Route.
	// +optional
	Host string `json:"host,omitempty"`

	// TLS termination type.
	// +kubebuilder:validation:Enum=edge;passthrough;reencrypt
	// +kubebuilder:default="edge"
	TLSTermination string `json:"tlsTermination,omitempty"`
}

// GatewayHTTPRouteSpec defines Gateway API HTTPRoute configuration.
type GatewayHTTPRouteSpec struct {
	// Enable HTTPRoute.
	Enabled bool `json:"enabled"`

	// Hostname for the HTTPRoute.
	// +optional
	Host string `json:"host,omitempty"`

	// ParentRefs are references to the Gateway(s) to attach the route to.
	// +kubebuilder:validation:MinItems=1
	ParentRefs []GatewayParentRef `json:"parentRefs"`

	// Annotations for the HTTPRoute.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewayParentRef identifies a Gateway to attach the HTTPRoute to.
type GatewayParentRef struct {
	// Name of the Gateway.
	Name string `json:"name"`

	// Namespace of the Gateway. Defaults to the route's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// SectionName is the name of a specific listener on the Gateway to attach to.
	// +optional
	SectionName string `json:"sectionName,omitempty"`
}

// SecuritySpec defines security settings.
type SecuritySpec struct {
	// NetworkPolicy configuration.
	// +optional
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`

	// RunAsNonRoot runs the LiteLLM container as a non-root user.
	// Required for OpenShift and clusters enforcing Pod Security Standards.
	// When enabled, the operator uses the official litellm-non_root image
	// (runs as nobody, UID 65534) and applies a restricted security context.
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`
}

// NetworkPolicySpec defines NetworkPolicy configuration.
type NetworkPolicySpec struct {
	// Enable NetworkPolicy.
	Enabled bool `json:"enabled"`

	// Allowed namespaces for ingress.
	// +optional
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`
}

// HealthCheckSpec defines health check configuration.
type HealthCheckSpec struct {
	// Liveness probe initial delay in seconds.
	// +kubebuilder:default=15
	LivenessInitialDelay int32 `json:"livenessInitialDelay,omitempty"`

	// Readiness probe initial delay in seconds.
	// +kubebuilder:default=10
	ReadinessInitialDelay int32 `json:"readinessInitialDelay,omitempty"`

	// Startup probe failure threshold.
	// +kubebuilder:default=30
	StartupFailureThreshold int32 `json:"startupFailureThreshold,omitempty"`
}

// PDBSpec defines PodDisruptionBudget configuration.
type PDBSpec struct {
	// Enable PDB.
	Enabled bool `json:"enabled"`

	// Minimum available pods.
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`

	// Maximum unavailable pods.
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// CallbacksSpec defines callback configuration.
type CallbacksSpec struct {
	// Callback types to enable (e.g., "langfuse", "otel", "custom").
	// +optional
	Types []string `json:"types,omitempty"`

	// Environment variables for callback configuration.
	// +optional
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`
}

// ObservabilitySpec defines observability configuration.
type ObservabilitySpec struct {
	// ServiceMonitor configuration for Prometheus.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`

	// PrometheusRule configuration for alerting.
	// +optional
	PrometheusRule *PrometheusRuleSpec `json:"prometheusRule,omitempty"`

	// Grafana dashboard configuration.
	// +optional
	GrafanaDashboard *GrafanaDashboardSpec `json:"grafanaDashboard,omitempty"`
}

// ServiceMonitorSpec defines ServiceMonitor configuration.
type ServiceMonitorSpec struct {
	// Enable ServiceMonitor creation.
	Enabled bool `json:"enabled"`

	// Scrape interval.
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`

	// Additional labels for the ServiceMonitor.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// PrometheusRuleSpec defines PrometheusRule configuration for alerting.
type PrometheusRuleSpec struct {
	// Enable PrometheusRule creation with default alerts.
	Enabled bool `json:"enabled"`

	// Additional labels for the PrometheusRule (e.g., for Prometheus rule selection).
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Disable specific default alerts by name.
	// +optional
	DisabledAlerts []string `json:"disabledAlerts,omitempty"`
}

// GrafanaDashboardSpec defines Grafana dashboard ConfigMap configuration.
type GrafanaDashboardSpec struct {
	// Enable Grafana dashboard ConfigMap creation.
	Enabled bool `json:"enabled"`

	// Additional labels for the dashboard ConfigMap.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Grafana folder to place the dashboard in.
	// +kubebuilder:default="LiteLLM"
	Folder string `json:"folder,omitempty"`
}

// UpgradeSpec defines upgrade strategy.
type UpgradeSpec struct {
	// Upgrade strategy.
	// +kubebuilder:validation:Enum=rolling;recreate
	// +kubebuilder:default="rolling"
	Strategy string `json:"strategy,omitempty"`

	// Maximum unavailable pods during rolling update.
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`

	// Maximum surge pods during rolling update.
	// +optional
	MaxSurge *int32 `json:"maxSurge,omitempty"`

	// Health check timeout after upgrade.
	// +kubebuilder:default="300s"
	HealthCheckTimeout string `json:"healthCheckTimeout,omitempty"`

	// Auto-rollback on failed health check.
	// +optional
	AutoRollback bool `json:"autoRollback,omitempty"`
}

// SSOSpec defines SSO authentication configuration.
type SSOSpec struct {
	// Enable SSO authentication.
	Enabled bool `json:"enabled"`

	// SSO provider type.
	// +kubebuilder:validation:Enum=azure-entra;okta;google;generic-oidc
	Provider string `json:"provider"`

	// Client ID for the SSO application.
	ClientID SecretKeyRef `json:"clientId"`

	// Client secret for the SSO application.
	ClientSecret SecretKeyRef `json:"clientSecret"`

	// Tenant ID (for Azure Entra).
	// +optional
	TenantID string `json:"tenantId,omitempty"`

	// Authorization endpoint URL (required for generic-oidc and okta).
	// +optional
	AuthorizationEndpoint string `json:"authorizationEndpoint,omitempty"`

	// Token endpoint URL (required for generic-oidc and okta).
	// +optional
	TokenEndpoint string `json:"tokenEndpoint,omitempty"`

	// UserInfo endpoint URL (required for generic-oidc and okta).
	// +optional
	UserinfoEndpoint string `json:"userinfoEndpoint,omitempty"`

	// OAuth scopes to request.
	// +kubebuilder:default={"openid","profile","email"}
	Scopes []string `json:"scopes,omitempty"`

	// JWT field that contains team/group IDs.
	// +kubebuilder:default="groups"
	TeamIDsJWTField string `json:"teamIdsJwtField,omitempty"`

	// User attribute mappings.
	// +optional
	UserAttributeMappings *UserAttributeMappings `json:"userAttributeMappings,omitempty"`

	// Default parameters for auto-created SSO users.
	// +optional
	DefaultUserParams *DefaultUserParams `json:"defaultUserParams,omitempty"`

	// Default parameters for auto-created teams from SSO groups.
	// +optional
	DefaultTeamParams *DefaultTeamParams `json:"defaultTeamParams,omitempty"`

	// Custom SSO handler module path (Python module).
	// +optional
	CustomSSOHandler string `json:"customSsoHandler,omitempty"`

	// Logout redirect URL.
	// +optional
	LogoutURL string `json:"logoutUrl,omitempty"`
}

// UserAttributeMappings defines SSO user attribute mappings.
type UserAttributeMappings struct {
	UserID      string `json:"userId,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	Role        string `json:"role,omitempty"`
}

// DefaultUserParams defines default parameters for SSO-created users.
type DefaultUserParams struct {
	// Maximum budget for new SSO users in USD.
	// +optional
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Budget reset duration (e.g., "30d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Models available to new SSO users.
	// +optional
	Models []string `json:"models,omitempty"`

	// Default role for new SSO users.
	// +kubebuilder:default="internal_user"
	// +kubebuilder:validation:Enum=internal_user;internal_user_viewer;proxy_admin;proxy_admin_viewer
	UserRole string `json:"userRole,omitempty"`

	// Teams to auto-assign new SSO users to.
	// +optional
	Teams []DefaultUserTeam `json:"teams,omitempty"`
}

// DefaultUserTeam defines team auto-assignment for SSO users.
type DefaultUserTeam struct {
	// Team ID.
	TeamID string `json:"teamId"`

	// Maximum budget within the team.
	// +optional
	MaxBudgetInTeam *float64 `json:"maxBudgetInTeam,omitempty"`

	// Role within the team.
	// +kubebuilder:default="user"
	Role string `json:"role,omitempty"`
}

// DefaultTeamParams defines default parameters for SSO-created teams.
type DefaultTeamParams struct {
	// Maximum budget in USD.
	// +optional
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Budget reset duration.
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Available models.
	// +optional
	Models []string `json:"models,omitempty"`

	// TPM limit.
	// +optional
	TPMLimit *int `json:"tpmLimit,omitempty"`

	// RPM limit.
	// +optional
	RPMLimit *int `json:"rpmLimit,omitempty"`
}

// SCIMSpec defines SCIM v2 provisioning configuration.
type SCIMSpec struct {
	// Enable SCIM v2 provisioning endpoints.
	Enabled bool `json:"enabled"`

	// Reference to Secret containing the SCIM bearer token.
	// If not specified, operator auto-generates a token and stores it.
	// +optional
	TokenSecretRef *SecretKeyRef `json:"tokenSecretRef,omitempty"`

	// Name for the auto-generated SCIM token Secret.
	// +kubebuilder:default="litellm-scim-token"
	GeneratedTokenSecretName string `json:"generatedTokenSecretName,omitempty"`
}

// LiteLLMInstanceStatus defines the observed state of LiteLLMInstance.
type LiteLLMInstanceStatus struct {
	// Whether the instance is fully ready.
	Ready bool `json:"ready,omitempty"`

	// Current replica count.
	Replicas int32 `json:"replicas,omitempty"`

	// Ready replica count.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Internal cluster endpoint URL.
	Endpoint string `json:"endpoint,omitempty"`

	// Current LiteLLM version.
	Version string `json:"version,omitempty"`

	// Database connection status.
	Database DatabaseStatus `json:"database,omitempty"`

	// Redis connection status.
	// +optional
	Redis *RedisStatus `json:"redis,omitempty"`

	// Config sync status.
	// +optional
	ConfigSync *ConfigSyncStatus `json:"configSync,omitempty"`

	// SSO configuration status.
	// +optional
	SSO *SSOStatus `json:"sso,omitempty"`

	// SCIM configuration status.
	// +optional
	SCIM *SCIMStatus `json:"scim,omitempty"`

	// Backup status (CloudNativePG).
	// +optional
	Backup *BackupStatus `json:"backup,omitempty"`

	// Last successful deployment revision for auto-rollback.
	// +optional
	LastSuccessfulRevision string `json:"lastSuccessfulRevision,omitempty"`

	// Standard Kubernetes conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// BackupStatus defines backup status for CNPG.
type BackupStatus struct {
	// Whether scheduled backups are configured.
	Configured bool `json:"configured,omitempty"`

	// Last backup time.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// Last backup status.
	// +optional
	LastBackupStatus string `json:"lastBackupStatus,omitempty"`
}

// DatabaseStatus defines database status.
type DatabaseStatus struct {
	// Whether the database is connected.
	Connected bool `json:"connected,omitempty"`

	// Current migration version.
	MigrationVersion string `json:"migrationVersion,omitempty"`
}

// RedisStatus defines Redis status.
type RedisStatus struct {
	// Whether Redis is connected.
	Connected bool `json:"connected,omitempty"`
}

// ConfigSyncStatus defines config sync status.
type ConfigSyncStatus struct {
	// Last sync time.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Count of synced models.
	SyncedModels int `json:"syncedModels,omitempty"`

	// Count of synced teams.
	SyncedTeams int `json:"syncedTeams,omitempty"`

	// Count of synced users.
	SyncedUsers int `json:"syncedUsers,omitempty"`

	// Count of synced keys.
	SyncedKeys int `json:"syncedKeys,omitempty"`

	// Count of unmanaged models.
	UnmanagedModels int `json:"unmanagedModels,omitempty"`

	// Count of unmanaged teams.
	UnmanagedTeams int `json:"unmanagedTeams,omitempty"`

	// Count of unmanaged users.
	UnmanagedUsers int `json:"unmanagedUsers,omitempty"`

	// Count of unmanaged keys.
	UnmanagedKeys int `json:"unmanagedKeys,omitempty"`

	// Sync errors.
	// +optional
	SyncErrors []string `json:"syncErrors,omitempty"`
}

// SSOStatus defines SSO status.
type SSOStatus struct {
	// Whether SSO is configured.
	Configured bool `json:"configured,omitempty"`

	// SSO provider type.
	Provider string `json:"provider,omitempty"`
}

// SCIMStatus defines SCIM status.
type SCIMStatus struct {
	// Whether SCIM is configured.
	Configured bool `json:"configured,omitempty"`

	// Name of the Secret containing the SCIM token.
	TokenSecretName string `json:"tokenSecretName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=li
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LiteLLMInstance is the Schema for the litellminstances API.
type LiteLLMInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMInstanceSpec   `json:"spec,omitempty"`
	Status LiteLLMInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMInstanceList contains a list of LiteLLMInstance.
type LiteLLMInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiteLLMInstance{}, &LiteLLMInstanceList{})
}

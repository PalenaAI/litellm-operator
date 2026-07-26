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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LiteLLMInstanceSpec defines the desired state of LiteLLMInstance.
type LiteLLMInstanceSpec struct {
	// Image configuration for the LiteLLM proxy.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image"
	Image ImageSpec `json:"image,omitempty"`

	// Number of LiteLLM proxy replicas.
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Replicas"
	Replicas int32 `json:"replicas,omitempty"`

	// Autoscaling configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Autoscaling"
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Master key configuration for LiteLLM admin access.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Master Key"
	MasterKey MasterKeySpec `json:"masterKey"`

	// Database configuration for LiteLLM state storage.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Database"
	Database DatabaseSpec `json:"database"`

	// Redis configuration for caching and routing.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis"
	Redis *RedisSpec `json:"redis,omitempty"`

	// Salt key for hashing.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Salt Key"
	SaltKey *SaltKeySpec `json:"saltKey,omitempty"`

	// General settings for the LiteLLM proxy.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="General Settings"
	GeneralSettings *GeneralSettingsSpec `json:"generalSettings,omitempty"`

	// Router settings for model routing.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Router Settings"
	RouterSettings *RouterSettingsSpec `json:"routerSettings,omitempty"`

	// LiteLLM library settings written to litellm_settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="LiteLLM Settings"
	LiteLLMSettings *LiteLLMSettingsSpec `json:"litellmSettings,omitempty"`

	// Fallback configuration for model routing.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Fallbacks"
	Fallbacks *FallbackSpec `json:"fallbacks,omitempty"`

	// Config sync settings for bidirectional synchronization.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Sync"
	ConfigSync *ConfigSyncSpec `json:"configSync,omitempty"`

	// Service configuration.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Service"
	Service ServiceSpec `json:"service,omitempty"`

	// Ingress configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Ingress"
	Ingress *IngressSpec `json:"ingress,omitempty"`

	// OpenShift Route configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Route"
	Route *RouteSpec `json:"route,omitempty"`

	// Gateway API HTTPRoute configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Gateway HTTP Route"
	GatewayHTTPRoute *GatewayHTTPRouteSpec `json:"gatewayHTTPRoute,omitempty"`

	// Security settings.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Security"
	Security *SecuritySpec `json:"security,omitempty"`

	// Health check configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Health Check"
	HealthCheck *HealthCheckSpec `json:"healthCheck,omitempty"`

	// Resource requirements for the LiteLLM container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resources"
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Pod disruption budget configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Pod Disruption Budget"
	PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`

	// Topology spread constraints.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Topology Spread Constraints"
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Extra environment variables for the LiteLLM container.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Extra Env Vars"
	ExtraEnvVars []corev1.EnvVar `json:"extraEnvVars,omitempty"`

	// Extra environment variable sources.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Extra Env From"
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`

	// Callback configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Callbacks"
	Callbacks *CallbacksSpec `json:"callbacks,omitempty"`

	// Observability configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Observability"
	Observability *ObservabilitySpec `json:"observability,omitempty"`

	// Upgrade strategy configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Upgrade"
	Upgrade *UpgradeSpec `json:"upgrade,omitempty"`

	// SSO configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="SSO"
	SSO *SSOSpec `json:"sso,omitempty"`

	// SCIM v2 provisioning configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="SCIM"
	SCIM *SCIMSpec `json:"scim,omitempty"`

	// JWT authentication configuration (enterprise).
	// Enables API-level authentication via JWT tokens from identity providers.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="JWT Auth"
	JWTAuth *JWTAuthSpec `json:"jwtAuth,omitempty"`

	// OAuth2 machine-to-machine authentication configuration (enterprise).
	// Maps JWT fields to LiteLLM attributes for service-to-service auth.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="OAuth2 Auth"
	OAuth2Auth *OAuth2AuthSpec `json:"oauth2Auth,omitempty"`

	// Response caching configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Caching"
	Caching *CachingSpec `json:"caching,omitempty"`

	// Pass-through endpoint definitions.
	// Allows proxying arbitrary API requests to upstream services through LiteLLM.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Pass Through Endpoints"
	PassThroughEndpoints []PassThroughEndpoint `json:"passThroughEndpoints,omitempty"`

	// Default budget settings applied to all end-users/customers.
	// Written to litellm_settings.max_end_user_budget / max_end_user_budget_id.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Default Customer Budget"
	DefaultCustomerBudget *DefaultCustomerBudgetSpec `json:"defaultCustomerBudget,omitempty"`

	// External secret manager configuration.
	// When configured, LiteLLM fetches secrets from the provider at runtime
	// instead of reading them from Kubernetes Secrets.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secret Manager"
	SecretManager *SecretManagerSpec `json:"secretManager,omitempty"`

	// Role-based access control configuration.
	// Controls route restrictions, key generation permissions, and per-role access.
	// Some features (key_generation_settings, role_permissions) require a LiteLLM Enterprise license.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RBAC"
	RBAC *RBACSpec `json:"rbac,omitempty"`

	// Logging configuration for the instance.
	// Controls audit logs, global message logging, and spend log retention.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Logging"
	Logging *InstanceLoggingSpec `json:"logging,omitempty"`

	// Admin UI configuration.
	// Controls UI availability, access restrictions, model persistence, and personal key creation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Admin UI"
	AdminUI *AdminUISpec `json:"adminUI,omitempty"`

	// TLS configuration for the LiteLLM proxy pod: serving HTTPS, trusting a
	// custom CA on outbound calls (provider + callback traffic), and presenting
	// a client certificate for outbound mTLS.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLS"
	TLS *TLSSpec `json:"tls,omitempty"`

	// ExtraVolumes are additional volumes attached to the proxy pod. Escape
	// hatch for mounting arbitrary Secrets/ConfigMaps the typed fields do not
	// cover. Pair with extraVolumeMounts.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Extra Volumes"
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts are additional volume mounts on the LiteLLM container.
	// Escape hatch paired with extraVolumes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Extra Volume Mounts"
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}

// TLSSpec configures TLS for the LiteLLM proxy pod.
//
// All references are Kubernetes Secrets, designed to accept cert-manager's
// standard keys (tls.crt/tls.key for certificate Secrets, ca.crt for CA
// bundles). The operator mounts each referenced Secret read-only and wires the
// corresponding LiteLLM environment variable to the mounted path.
type TLSSpec struct {
	// ServerCertSecretRef references a TLS Secret (kubernetes.io/tls, with
	// tls.crt and tls.key) the proxy serves HTTPS with. When set the operator
	// mounts the Secret and sets both SSL_KEYFILE_PATH and SSL_CERTFILE_PATH,
	// so uvicorn serves HTTPS on the proxy port. Clients must then use https://
	// (including the Service/Ingress in front of the proxy).
	// +optional
	ServerCertSecretRef *SecretRef `json:"serverCertSecretRef,omitempty"`

	// TrustedCASecretRef references a Secret containing a CA bundle the proxy
	// trusts on outbound HTTPS calls (model providers and logging callbacks
	// such as Langfuse). The operator mounts it and sets SSL_CERT_FILE to the
	// mounted path — the documented LiteLLM knob for a custom outbound CA
	// (LiteLLM is Python/httpx; SSL_CERT_FILE, not REQUESTS_CA_BUNDLE).
	// +optional
	TrustedCASecretRef *CASecretRef `json:"trustedCASecretRef,omitempty"`

	// ClientCertSecretRef references a TLS Secret (tls.crt/tls.key) the proxy
	// presents as a client certificate for outbound mTLS. When set the operator
	// mounts it and sets SSL_CERTIFICATE to the mounted certificate path.
	// +optional
	ClientCertSecretRef *SecretRef `json:"clientCertSecretRef,omitempty"`
}

// CASecretRef references a CA bundle within a Kubernetes Secret. The key
// defaults to cert-manager's standard "ca.crt".
type CASecretRef struct {
	// Name of the Secret.
	Name string `json:"name"`

	// Key within the Secret holding the PEM CA bundle.
	// +kubebuilder:default="ca.crt"
	// +optional
	Key string `json:"key,omitempty"`
}

// DefaultCustomerBudgetSpec sets platform-wide defaults for new end-users.
type DefaultCustomerBudgetSpec struct {
	// Default maximum budget (USD) applied to new customers.
	// Written to litellm_settings.max_end_user_budget.
	// +optional
	MaxBudget *float64 `json:"maxBudget,omitempty"`

	// Default predefined budget tier ID.
	// Written to litellm_settings.max_end_user_budget_id.
	// +optional
	BudgetID string `json:"budgetId,omitempty"`
}

// SecretManagerSpec defines external secret manager integration.
// LiteLLM connects to the provider at runtime to fetch API keys and
// optionally store generated virtual keys.
type SecretManagerSpec struct {
	// Secret manager provider.
	// +kubebuilder:validation:Enum=aws_secret_manager;aws_kms;azure_key_vault;google_secret_manager;google_kms;hashicorp_vault
	Provider string `json:"provider"`

	// Provider credentials. Reference to a Secret containing
	// provider-specific authentication fields (e.g., AWS_ACCESS_KEY_ID,
	// AZURE_CLIENT_ID). May be omitted when using workload identity
	// (e.g., IRSA on EKS, Workload Identity on GKE).
	// +optional
	CredentialsSecretRef *SecretRef `json:"credentialsSecretRef,omitempty"`

	// List of environment variable names that LiteLLM should
	// resolve from the secret manager instead of the pod environment.
	// +optional
	HostedKeys []string `json:"hostedKeys,omitempty"`

	// Store generated virtual keys in the secret manager.
	// +optional
	StoreVirtualKeys *bool `json:"storeVirtualKeys,omitempty"`

	// Prefix for stored virtual keys (e.g., "litellm/").
	// +optional
	PrefixForStoredVirtualKeys string `json:"prefixForStoredVirtualKeys,omitempty"`

	// Access mode for the secret manager.
	// +kubebuilder:validation:Enum=read_only;write_only;read_and_write
	// +kubebuilder:default="read_only"
	// +optional
	AccessMode string `json:"accessMode,omitempty"`

	// Name of a single secret containing multiple key-value pairs as JSON.
	// +optional
	PrimarySecretName string `json:"primarySecretName,omitempty"`

	// AWS-specific configuration (for aws_secret_manager and aws_kms providers).
	// +optional
	AWS *AWSSecretManagerConfig `json:"aws,omitempty"`

	// Azure-specific configuration (for azure_key_vault provider).
	// +optional
	Azure *AzureKeyVaultConfig `json:"azure,omitempty"`

	// HashiCorp Vault-specific configuration (for hashicorp_vault provider).
	// +optional
	Vault *VaultConfig `json:"vault,omitempty"`
}

// AWSSecretManagerConfig defines AWS-specific secret manager settings.
type AWSSecretManagerConfig struct {
	// AWS region.
	Region string `json:"region"`

	// IAM role ARN for role assumption (alternative to static credentials).
	// +optional
	RoleARN string `json:"roleARN,omitempty"`

	// Session name for role assumption.
	// +optional
	SessionName string `json:"sessionName,omitempty"`

	// Path to web identity token file (for IRSA on EKS).
	// When set, the operator mounts the projected token as a volume.
	// +optional
	WebIdentityTokenPath string `json:"webIdentityTokenPath,omitempty"`

	// Custom STS endpoint (for VPC environments).
	// +optional
	STSEndpoint string `json:"stsEndpoint,omitempty"`
}

// AzureKeyVaultConfig defines Azure Key Vault-specific settings.
type AzureKeyVaultConfig struct {
	// Azure Key Vault URI.
	VaultURI string `json:"vaultURI"`

	// Azure tenant ID.
	TenantID string `json:"tenantID"`
}

// VaultConfig defines HashiCorp Vault-specific settings.
type VaultConfig struct {
	// Vault server address.
	Address string `json:"address"`

	// Vault namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Auth method: approle, tls, or token.
	// +kubebuilder:validation:Enum=approle;tls;token
	// +kubebuilder:default="approle"
	// +optional
	AuthMethod string `json:"authMethod,omitempty"`

	// AppRole mount path (defaults to "approle").
	// +optional
	AppRoleMountPath string `json:"appRoleMountPath,omitempty"`

	// KV engine mount name (defaults to "secret").
	// +optional
	MountName string `json:"mountName,omitempty"`

	// Path prefix for secrets.
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// Cache refresh interval in seconds.
	// +optional
	RefreshInterval *int `json:"refreshInterval,omitempty"`
}

// RBACSpec defines role-based access control configuration.
// Controls route restrictions, key generation permissions, and per-role access.
type RBACSpec struct {
	// Enable role-based access control enforcement.
	Enabled bool `json:"enabled"`

	// Routes restricted to proxy_admin only.
	// +optional
	AdminOnlyRoutes []string `json:"adminOnlyRoutes,omitempty"`

	// Routes accessible to all authenticated users.
	// If set, only these routes are allowed; all others are blocked.
	// +optional
	AllowedRoutes []string `json:"allowedRoutes,omitempty"`

	// Key generation restrictions.
	// +optional
	KeyGeneration *KeyGenerationSettings `json:"keyGeneration,omitempty"`

	// Per-role permission definitions.
	// +optional
	RolePermissions map[string]RolePermission `json:"rolePermissions,omitempty"`

	// Prevent users from creating personal keys (force team-based keys).
	// +optional
	DefaultTeamDisabled *bool `json:"defaultTeamDisabled,omitempty"`
}

// KeyGenerationSettings defines restrictions on who can generate API keys.
type KeyGenerationSettings struct {
	// Team key generation restrictions.
	// +optional
	TeamKeyGeneration *TeamKeyGenerationSettings `json:"teamKeyGeneration,omitempty"`

	// Personal key generation restrictions.
	// +optional
	PersonalKeyGeneration *PersonalKeyGenerationSettings `json:"personalKeyGeneration,omitempty"`
}

// TeamKeyGenerationSettings defines which team member roles can generate team keys.
type TeamKeyGenerationSettings struct {
	// Team member roles allowed to generate team keys.
	// +kubebuilder:validation:Items:Enum=admin;user
	AllowedTeamMemberRoles []string `json:"allowedTeamMemberRoles"`
}

// PersonalKeyGenerationSettings defines which user roles can generate personal keys.
type PersonalKeyGenerationSettings struct {
	// User roles allowed to generate personal keys.
	// +kubebuilder:validation:Items:Enum=proxy_admin;proxy_admin_viewer;internal_user;internal_user_viewer
	AllowedUserRoles []string `json:"allowedUserRoles"`
}

// RolePermission defines the routes and models accessible to a specific role.
type RolePermission struct {
	// API routes this role can access.
	// +optional
	Routes []string `json:"routes,omitempty"`

	// Models this role can use.
	// +optional
	Models []string `json:"models,omitempty"`
}

// InstanceLoggingSpec configures instance-level logging behavior.
type InstanceLoggingSpec struct {
	// Enable audit logs (enterprise).
	// Stores admin actions (key creation, team changes, etc.) in the database.
	// +optional
	AuditLogs *AuditLogSpec `json:"auditLogs,omitempty"`

	// Disable logging of request/response message content.
	// Only metadata (tokens, cost, model) is logged.
	// +optional
	TurnOffMessageLogging *bool `json:"turnOffMessageLogging,omitempty"`

	// Redact user API key information from logs.
	// +optional
	RedactUserAPIKeyInfo *bool `json:"redactUserApiKeyInfo,omitempty"`

	// Spend log retention configuration.
	// +optional
	SpendLogRetention *SpendLogRetentionSpec `json:"spendLogRetention,omitempty"`
}

// AuditLogSpec configures audit logging.
type AuditLogSpec struct {
	// Enable audit logging.
	Enabled bool `json:"enabled"`

	// Retention period in days.
	// +optional
	RetentionDays *int `json:"retentionDays,omitempty"`
}

// SpendLogRetentionSpec configures spend log retention and cleanup.
type SpendLogRetentionSpec struct {
	// Maximum retention period (e.g., "90d", "1y").
	// Written to general_settings.maximum_spend_logs_retention_period.
	// +optional
	MaxRetentionPeriod string `json:"maxRetentionPeriod,omitempty"`

	// Cleanup interval (e.g., "1d", "1h").
	// Written to general_settings.maximum_spend_logs_retention_interval.
	// +optional
	CleanupInterval string `json:"cleanupInterval,omitempty"`
}

// AdminUISpec defines Admin UI configuration for a LiteLLM instance.
// +kubebuilder:object:generate=true
type AdminUISpec struct {
	// Disable the Admin UI entirely. When true, the /ui endpoint returns 404.
	// Injected as DISABLE_ADMIN_UI environment variable.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// Restrict UI access to admin users only (proxy_admin and proxy_admin_viewer).
	// When true, sets ui_access_mode: "admin_only" in general_settings.
	// +optional
	AdminOnly *bool `json:"adminOnly,omitempty"`

	// Store model definitions in the database instead of the config file.
	// Enables adding/editing models from the Admin UI without proxy restart.
	// Written to general_settings.store_model_in_db.
	// +optional
	StoreModelInDB *bool `json:"storeModelInDB,omitempty"`

	// Prevent users from creating personal API keys.
	// When true, keys can only be created under an assigned team.
	// Written to general_settings.default_team_disabled.
	// +optional
	DefaultTeamDisabled *bool `json:"defaultTeamDisabled,omitempty"`

	// Custom base URL for the API reference documentation shown in the UI.
	// Useful when the Admin UI is served from a different host than the proxy.
	// Injected as LITELLM_UI_API_DOC_BASE_URL environment variable.
	// +optional
	APIDocBaseURL string `json:"apiDocBaseURL,omitempty"`

	// Custom path for the docs endpoint (default: "/").
	// Injected as DOCS_URL environment variable.
	// +optional
	DocsURL string `json:"docsURL,omitempty"`

	// URL to redirect to when the root path is accessed and docsURL is changed.
	// Injected as ROOT_REDIRECT_URL environment variable.
	// +optional
	RootRedirectURL string `json:"rootRedirectURL,omitempty"`

	// URL to a hosted logo image displayed in the Admin UI.
	// Injected as UI_LOGO_PATH environment variable.
	// +optional
	LogoURL string `json:"logoURL,omitempty"`

	// URL to a logo image included in email notifications (budget alerts, invitations).
	// Injected as EMAIL_LOGO_URL environment variable.
	// +optional
	EmailLogoURL string `json:"emailLogoURL,omitempty"`

	// Support email address displayed in email notifications.
	// Injected as EMAIL_SUPPORT_CONTACT environment variable.
	// +optional
	EmailSupportContact string `json:"emailSupportContact,omitempty"`

	// Reference to a ConfigMap containing a custom UI color theme.
	// The ConfigMap must have a key named "enterprise_colors.json" with a JSON
	// object defining brand colors (uses the Tremor color palette).
	// Mounted at /app/enterprise/enterprise_ui/enterprise_colors.json in the container.
	// +optional
	ColorThemeConfigMapRef *ConfigMapRef `json:"colorThemeConfigMapRef,omitempty"`
}

// ConfigMapRef references a Kubernetes ConfigMap by name.
type ConfigMapRef struct {
	// Name of the ConfigMap.
	Name string `json:"name"`
}

// ImageSpec defines the container image for LiteLLM.
type ImageSpec struct {
	// Container image repository.
	// +kubebuilder:default="ghcr.io/berriai/litellm"
	Repository string `json:"repository,omitempty"`

	// Container image tag.
	// +kubebuilder:default="latest"
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

	// TLS settings for the PostgreSQL connection.
	// +optional
	TLS *DatabaseTLSSpec `json:"tls,omitempty"`
}

// DatabaseTLSSpec configures TLS material for the PostgreSQL connection.
//
// Important: LiteLLM talks to Postgres through Prisma, whose native connector
// reads SSL parameters from the connection string, NOT from libpq PG* env vars,
// and does NOT accept libpq's verify-full / sslrootcert=system spellings — it
// uses sslmode=require together with sslaccept=strict plus sslrootcert/sslcert/
// sslkey paths. Because DATABASE_URL is supplied via a Secret the operator does
// not rebuild, the operator cannot inject those parameters for you. This spec
// therefore only MOUNTS the certificate material at deterministic paths; the
// caller must add the SSL parameters to the DATABASE_URL Secret value, e.g.:
//
//	postgresql://user:pass@host:5432/db?sslmode=require&sslaccept=strict&sslrootcert=/etc/litellm/db-tls/ca/ca.crt
//
// The mounted paths are stable: the CA bundle at /etc/litellm/db-tls/ca/<key>
// and (for mTLS) tls.crt/tls.key at /etc/litellm/db-tls/client/. The same
// material is mounted on both the proxy Deployment and the migration Job.
type DatabaseTLSSpec struct {
	// CASecretRef references a Secret containing the PostgreSQL server CA
	// bundle. Mounted at /etc/litellm/db-tls/ca/<key>; reference it in
	// DATABASE_URL via sslrootcert=<that path>.
	// +optional
	CASecretRef *CASecretRef `json:"caSecretRef,omitempty"`

	// ClientCertSecretRef references a TLS Secret (tls.crt/tls.key) for
	// PostgreSQL mutual TLS. Mounted at /etc/litellm/db-tls/client/; reference
	// the files in DATABASE_URL via sslcert= and sslkey=.
	// +optional
	ClientCertSecretRef *SecretRef `json:"clientCertSecretRef,omitempty"`
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
//
// Since v0.12.x the operator runs `prisma migrate deploy` (applying
// LiteLLM's versioned migration files in `litellm-proxy-extras/migrations`)
// rather than `prisma db push --accept-data-loss`. This matches the command
// LiteLLM's own componentized chart uses and is mandatory for LiteLLM
// v1.86+, which disables schema updates in app pods (PR #27557).
type MigrationSpec struct {
	// Run database migration before starting.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Timeout for migration job.
	// +kubebuilder:default="300s"
	Timeout string `json:"timeout,omitempty"`

	// UseDatabaseImage switches the migration Job from running
	// `prisma migrate deploy` inside the gateway image to running LiteLLM's
	// dedicated `litellm-migrations` migrations image instead. That image's
	// entrypoint runs migrations with the componentized recovery logic
	// (P3005/P3009/P3018 retries, v2 migration resolver) — the operator
	// only injects DATABASE_URL and lets the image do the rest.
	//
	// When true, ONLY the database image runs — the operator does not also
	// invoke prisma inside the gateway image. Recommended for LiteLLM
	// v1.86+ and especially for upgrading from operator versions that
	// previously used `prisma db push` (the database image auto-recovers
	// from the resulting `_prisma_migrations` / schema drift).
	//
	// Tag availability caveat: as of June 2026, ghcr.io/berriai/litellm-
	// migrations only publishes release-candidate tags (e.g.
	// v1.87.0-rc.1, v1.88.0-rc.1). If your gateway tag is not yet
	// published as a migrations tag, override via DatabaseImage.Tag or
	// stay on the gateway-image path (which works on every LiteLLM v1.85+
	// gateway tag — the operator runs the same setup_database recovery
	// logic inside the gateway image).
	// +optional
	UseDatabaseImage bool `json:"useDatabaseImage,omitempty"`

	// DatabaseImage overrides the migration image used when
	// UseDatabaseImage is true. Repository defaults to
	// "ghcr.io/berriai/litellm-migrations" and Tag defaults to spec.image.tag
	// so the migrations image stays version-aligned with the gateway. The
	// gateway image's pullSecrets are reused for this image too. Only
	// consulted when UseDatabaseImage is true.
	// +optional
	DatabaseImage *DatabaseImageSpec `json:"databaseImage,omitempty"`
}

// DatabaseImageSpec overrides the dedicated LiteLLM migrations image used
// when MigrationSpec.UseDatabaseImage is true.
type DatabaseImageSpec struct {
	// Repository for the database migration image.
	// Defaults to "ghcr.io/berriai/litellm-migrations".
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag for the database migration image. Defaults to spec.image.tag so
	// the migrations image stays version-aligned with the gateway image.
	// +optional
	Tag string `json:"tag,omitempty"`

	// Image pull policy.
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
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

	// Alerting enables the alert delivery channels (e.g. ["slack"]). Without
	// this, configuring alertTypes alone does not deliver any alerts. For
	// Slack, also set the SLACK_WEBHOOK_URL env var (via extraEnvVars).
	// +optional
	Alerting []string `json:"alerting,omitempty"`

	// AlertingThreshold is the number of seconds a request may hang before a
	// hanging-request alert fires.
	// +optional
	AlertingThreshold *int `json:"alertingThreshold,omitempty"`

	// AlertToWebhookURL maps an alert type to a specific webhook URL, overriding
	// the default channel for that alert type.
	// +optional
	AlertToWebhookURL map[string]string `json:"alertToWebhookUrl,omitempty"`

	// BackgroundHealthChecks enables periodic background health checks so
	// GET /health returns cached results instead of probing on each call.
	// +optional
	BackgroundHealthChecks *bool `json:"backgroundHealthChecks,omitempty"`

	// HealthCheckInterval is the seconds between background health checks
	// (default 300). Only applies when backgroundHealthChecks is true.
	// +optional
	HealthCheckInterval *int `json:"healthCheckInterval,omitempty"`

	// HealthCheckDetails toggles whether GET /health exposes endpoint URLs and
	// error details. Set false to hide them.
	// +optional
	HealthCheckDetails *bool `json:"healthCheckDetails,omitempty"`

	// HealthCheckSkipDisabledModels excludes models that set
	// modelInfo.healthCheck.disableBackgroundHealthCheck: true from the
	// on-demand GET /health probe as well (not just the background loop),
	// rendering health_check_skip_disabled_background_models. Use this to stop
	// on-demand /health from live-probing (and billing for) models you have
	// opted out of health checking.
	// +optional
	HealthCheckSkipDisabledModels *bool `json:"healthCheckSkipDisabledModels,omitempty"`

	// Custom key generation function.
	// +optional
	CustomKeyGenerate string `json:"customKeyGenerate,omitempty"`

	// Allow requests with no key.
	// +optional
	AllowUserAuth *bool `json:"allowUserAuth,omitempty"`

	// Global proxy budget in USD.
	// +optional
	MaxBudget *string `json:"maxBudget,omitempty"`

	// Global budget reset duration (e.g., "1d", "7d", "30d").
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// Maximum parallel requests across the entire proxy.
	// +optional
	GlobalMaxParallelRequests *int `json:"globalMaxParallelRequests,omitempty"`

	// Minimum interval in seconds between budget reset checks.
	// +optional
	BudgetReschedulerMinTime *int `json:"budgetReschedulerMinTime,omitempty"`

	// Maximum interval in seconds between budget reset checks.
	// +optional
	BudgetReschedulerMaxTime *int `json:"budgetReschedulerMaxTime,omitempty"`
}

// FallbackSpec defines fallback chain configuration for model routing.
type FallbackSpec struct {
	// Default fallback models applied on any error.
	// List of model names to try in order.
	// +optional
	DefaultFallbacks []string `json:"defaultFallbacks,omitempty"`

	// Per-model fallback chains for general errors.
	// +optional
	ModelFallbacks []ModelFallbackEntry `json:"modelFallbacks,omitempty"`

	// Fallback models for content policy violations.
	// +optional
	ContentPolicyFallbacks []ModelFallbackEntry `json:"contentPolicyFallbacks,omitempty"`

	// Fallback models for context window exceeded errors.
	// +optional
	ContextWindowFallbacks []ModelFallbackEntry `json:"contextWindowFallbacks,omitempty"`

	// Maximum number of fallback attempts.
	// +kubebuilder:default=3
	// +optional
	MaxFallbacks *int `json:"maxFallbacks,omitempty"`
}

// ModelFallbackEntry maps a primary model to an ordered list of fallback models.
type ModelFallbackEntry struct {
	// Primary model name.
	Model string `json:"model"`

	// Ordered list of fallback model names.
	Fallbacks []string `json:"fallbacks"`
}

// RouterSettingsSpec defines LiteLLM router settings.
type RouterSettingsSpec struct {
	// Routing strategy.
	// +kubebuilder:validation:Enum=simple-shuffle;least-busy;latency-based-routing;usage-based-routing;usage-based-routing-v2;cost-based-routing
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

	// Global retry policy by error type.
	// Keys: TimeoutError, RateLimitError, ContentPolicyViolationError, etc.
	// Values: number of retries.
	// +optional
	RetryPolicy map[string]int `json:"retryPolicy,omitempty"`

	// Per-model-group retry policies.
	// +optional
	ModelGroupRetryPolicy map[string]map[string]int `json:"modelGroupRetryPolicy,omitempty"`

	// Enable tag-based routing. When enabled, requests with matching tags
	// are routed to model deployments that share those tags.
	// +optional
	EnableTagFiltering *bool `json:"enableTagFiltering,omitempty"`

	// If true, match requests having ANY of the specified tags (OR logic).
	// If false (default), ALL tags must match (AND logic).
	// +optional
	TagFilteringMatchAny *bool `json:"tagFilteringMatchAny,omitempty"`

	// Default max parallel requests per model deployment.
	// +optional
	DefaultMaxParallelRequests *int `json:"defaultMaxParallelRequests,omitempty"`

	// Per-provider budget limits.
	// +optional
	ProviderBudgetConfig map[string]ProviderBudget `json:"providerBudgetConfig,omitempty"`

	// Stream timeout in seconds for streaming responses. Distinct from
	// `timeout`, which applies to the full (non-streaming) request.
	// +optional
	StreamTimeout *int `json:"streamTimeout,omitempty"`

	// EnablePreCallChecks filters deployments by context-window size and
	// region before making a call, so a request that would exceed a
	// deployment's context window is routed to one that fits.
	// +optional
	EnablePreCallChecks *bool `json:"enablePreCallChecks,omitempty"`

	// ModelGroupAlias maps an alias model-group name to a real one, so clients
	// can request the alias and be routed to the underlying model group.
	// +optional
	ModelGroupAlias map[string]string `json:"modelGroupAlias,omitempty"`
}

// LiteLLMSettingsSpec configures the LiteLLM library settings rendered under
// `litellm_settings` in proxy_server_config.yaml.
type LiteLLMSettingsSpec struct {
	// JSONLogs emits structured JSON logs instead of plaintext — recommended
	// for log aggregation in Kubernetes.
	// +optional
	JSONLogs *bool `json:"jsonLogs,omitempty"`
}

// ProviderBudget defines a spending limit for a single LLM provider.
type ProviderBudget struct {
	// Budget limit in USD.
	BudgetLimit string `json:"budgetLimit"`

	// Time period for the budget (e.g., "1d", "7d", "30d").
	TimePeriod string `json:"timePeriod"`
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

	// IP allowlist configuration (enterprise).
	// Restricts API access to specific IP addresses or CIDR ranges.
	// +optional
	IPAllowlist *IPAllowlistSpec `json:"ipAllowlist,omitempty"`
}

// IPAllowlistSpec defines IP address filtering configuration.
// This is a LiteLLM enterprise feature.
type IPAllowlistSpec struct {
	// Enable IP address filtering.
	Enabled bool `json:"enabled"`

	// List of allowed IP addresses or CIDR ranges (e.g. "10.0.0.1", "192.168.1.0/24").
	// +kubebuilder:validation:MinItems=1
	AllowedIPs []string `json:"allowedIPs"`

	// Use X-Forwarded-For header for client IP detection.
	// Enable when LiteLLM is behind a load balancer or reverse proxy.
	// +optional
	UseXForwardedFor *bool `json:"useXForwardedFor,omitempty"`

	// Maximum request size in MB (enterprise).
	// +optional
	MaxRequestSizeMB *int `json:"maxRequestSizeMB,omitempty"`

	// Maximum response size in MB (enterprise).
	// +optional
	MaxResponseSizeMB *int `json:"maxResponseSizeMB,omitempty"`
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

	// Path is the HTTP path to scrape metrics from.
	// Defaults to "/metrics".
	// +optional
	Path string `json:"path,omitempty"`

	// Authorization configures the Authorization header Prometheus sends when
	// scraping the metrics endpoint. This is required when the LiteLLM proxy is
	// protected with a master key: the /metrics endpoint then rejects
	// unauthenticated scrapes with "Malformed API Key passed in. Ensure Key has
	// `Bearer ` prefix." When Authorization is set but Credentials is omitted,
	// the operator defaults to the instance's master key Secret.
	// +optional
	Authorization *ServiceMonitorAuthorization `json:"authorization,omitempty"`
}

// ServiceMonitorAuthorization configures endpoint authorization for the
// generated ServiceMonitor, mirroring the Prometheus Operator's native
// `endpoints[].authorization` block.
type ServiceMonitorAuthorization struct {
	// Type is the authorization type, e.g. "Bearer".
	// +kubebuilder:default="Bearer"
	// +optional
	Type string `json:"type,omitempty"`

	// Credentials selects the key of a Secret holding the credentials Prometheus
	// sends in the Authorization header. When omitted, the operator defaults to
	// the instance's master key Secret (the referenced Secret for a
	// user-supplied master key, or the auto-generated "<instance>-master-key"
	// Secret otherwise).
	// +optional
	Credentials *SecretKeyRef `json:"credentials,omitempty"`
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

	// Custom SSO handler invoked by LiteLLM after a successful SSO login.
	// Written to general_settings.custom_sso as a dotted Python module path.
	// Provide either an inline module path (handler baked into the image)
	// or a ConfigMap reference (operator mounts the code at runtime).
	// +optional
	CustomSSOHandler *CustomSSOHandlerSpec `json:"customSsoHandler,omitempty"`

	// Logout redirect URL applied to the Admin UI logout action.
	// Written to the PROXY_LOGOUT_URL env var on the Deployment.
	// +optional
	LogoutURL string `json:"logoutUrl,omitempty"`
}

// CustomSSOHandlerSpec configures the LiteLLM custom SSO handler.
// Exactly one of module or configMapRef must be set.
type CustomSSOHandlerSpec struct {
	// Dotted Python module path to the handler function
	// (e.g. "my_package.my_handler"). Use when the handler is baked
	// into a custom LiteLLM image. Mutually exclusive with configMapRef.
	// +optional
	Module string `json:"module,omitempty"`

	// Reference to a ConfigMap containing the handler's Python source.
	// The operator mounts the ConfigMap at /app/custom_sso_handlers/ and
	// sets custom_sso to "custom_sso_handlers.<stem>.<functionName>",
	// where <stem> is fileName with the .py suffix removed.
	// Mutually exclusive with module.
	// +optional
	ConfigMapRef *CustomSSOHandlerConfigMapRef `json:"configMapRef,omitempty"`
}

// CustomSSOHandlerConfigMapRef references a ConfigMap key holding Python source
// and the function name to call within it.
type CustomSSOHandlerConfigMapRef struct {
	// Name of the ConfigMap in the same namespace as the LiteLLMInstance.
	Name string `json:"name"`

	// Key in the ConfigMap whose value is the Python source. The key
	// becomes the filename in the mounted directory. Must end with ".py".
	// +kubebuilder:default="handler.py"
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*\.py$`
	FileName string `json:"fileName,omitempty"`

	// Name of the Python function within the handler file. Appended to
	// the derived module path when writing custom_sso.
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	FunctionName string `json:"functionName"`
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

// CachingSpec defines response caching configuration.
type CachingSpec struct {
	// Enable response caching.
	Enabled bool `json:"enabled"`

	// Cache backend type.
	// +kubebuilder:validation:Enum=redis;s3;gcs;local;qdrant;redis-semantic
	// +kubebuilder:default="redis"
	Type string `json:"type,omitempty"`

	// Redis cache configuration (when type is "redis" or "redis-semantic").
	// +optional
	Redis *CacheRedisSpec `json:"redis,omitempty"`

	// S3 cache configuration (when type is "s3").
	// +optional
	S3 *CacheS3Spec `json:"s3,omitempty"`

	// GCS cache configuration (when type is "gcs").
	// +optional
	GCS *CacheGCSSpec `json:"gcs,omitempty"`

	// Qdrant semantic cache configuration (when type is "qdrant").
	// +optional
	Qdrant *CacheQdrantSpec `json:"qdrant,omitempty"`

	// Cache TTL in seconds.
	// +kubebuilder:default=600
	// +optional
	TTL *int `json:"ttl,omitempty"`

	// Namespace for cache key isolation.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Restrict caching to specific call types.
	// +optional
	SupportedCallTypes []string `json:"supportedCallTypes,omitempty"`

	// Cache mode: "default_on" (cache everything) or "default_off" (require explicit opt-in).
	// +kubebuilder:validation:Enum=default_on;default_off
	// +kubebuilder:default="default_on"
	// +optional
	Mode string `json:"mode,omitempty"`
}

// CacheRedisSpec defines Redis cache backend configuration.
type CacheRedisSpec struct {
	// Redis host. If empty, uses the instance's Redis config.
	// +optional
	Host string `json:"host,omitempty"`

	// Redis port.
	// +optional
	Port *int `json:"port,omitempty"`

	// Reference to Secret containing the Redis password.
	// +optional
	PasswordSecretRef *SecretKeyRef `json:"passwordSecretRef,omitempty"`

	// Enable SSL/TLS.
	// +optional
	SSL bool `json:"ssl,omitempty"`
}

// CacheS3Spec defines S3 cache backend configuration.
type CacheS3Spec struct {
	// S3 bucket name.
	BucketName string `json:"bucketName"`

	// AWS region.
	// +optional
	Region string `json:"region,omitempty"`

	// Reference to Secret containing AWS credentials.
	// +optional
	CredentialsSecretRef *SecretKeyRef `json:"credentialsSecretRef,omitempty"`
}

// CacheGCSSpec defines GCS cache backend configuration.
type CacheGCSSpec struct {
	// GCS bucket name.
	BucketName string `json:"bucketName"`

	// Reference to Secret containing GCS service account JSON.
	// +optional
	CredentialsSecretRef *SecretKeyRef `json:"credentialsSecretRef,omitempty"`
}

// CacheQdrantSpec defines Qdrant semantic cache backend configuration.
type CacheQdrantSpec struct {
	// Qdrant server URL.
	URL string `json:"url"`

	// Reference to Secret containing Qdrant API key.
	// +optional
	APIKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`

	// Collection name for cached embeddings.
	// +optional
	CollectionName string `json:"collectionName,omitempty"`
}

// PassThroughEndpoint defines a pass-through endpoint that proxies requests to an upstream service.
type PassThroughEndpoint struct {
	// Route path on the LiteLLM proxy (e.g., "/bria", "/api/v1/custom").
	Path string `json:"path"`

	// Target URL to forward requests to.
	Target string `json:"target"`

	// Enable LiteLLM authentication for this endpoint (enterprise).
	// +optional
	Auth *bool `json:"auth,omitempty"`

	// Forward incoming client headers to the target.
	// +optional
	ForwardHeaders *bool `json:"forwardHeaders,omitempty"`

	// Forward requests to sub-paths (e.g., /path/sub/route → target/sub/route).
	// +optional
	IncludeSubpath *bool `json:"includeSubpath,omitempty"`

	// HTTP methods to allow. If empty, all methods are allowed.
	// +optional
	Methods []string `json:"methods,omitempty"`

	// Custom headers to add to forwarded requests.
	// For headers containing secrets, use headerSecrets instead.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Headers sourced from Kubernetes Secrets.
	// +optional
	HeaderSecrets []HeaderSecretRef `json:"headerSecrets,omitempty"`

	// Default query parameters added to all forwarded requests.
	// +optional
	DefaultQueryParams map[string]string `json:"defaultQueryParams,omitempty"`
}

// HeaderSecretRef references a Secret value to use as an HTTP header.
type HeaderSecretRef struct {
	// HTTP header name (e.g., "Authorization").
	HeaderName string `json:"headerName"`

	// Prefix prepended to the secret value (e.g., "Bearer ").
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// Reference to Secret containing the header value.
	SecretRef SecretKeyRef `json:"secretRef"`
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

// JWTAuthSpec defines JWT-based authentication configuration (enterprise).
// When enabled, LiteLLM validates JWT tokens from identity providers and
// maps claims to roles, teams, and organizations.
type JWTAuthSpec struct {
	// Enable JWT-based authentication.
	Enabled bool `json:"enabled"`

	// PublicKeyURL is the JWKS endpoint the proxy fetches token-signing keys
	// from (e.g. an Entra/Keycloak `.../discovery/v2.0/keys` URL). Rendered as
	// the JWT_PUBLIC_KEY_URL env var (comma-separate multiple URLs). Without it
	// the proxy cannot validate any token even when litellm_jwtauth is set.
	// +optional
	PublicKeyURL string `json:"publicKeyUrl,omitempty"`

	// Issuer is the expected `iss` claim; rendered as the JWT_ISSUER env var.
	// +optional
	Issuer string `json:"issuer,omitempty"`

	// Audience is the expected `aud` claim; rendered as the JWT_AUDIENCE env var.
	// +optional
	Audience string `json:"audience,omitempty"`

	// JWT scope value that grants proxy admin access.
	// +optional
	AdminJWTScope string `json:"adminJwtScope,omitempty"`

	// Routes accessible to admin JWT holders.
	// +optional
	AdminAllowedRoutes []string `json:"adminAllowedRoutes,omitempty"`

	// JWT field containing the team ID.
	// +optional
	TeamIDJWTField string `json:"teamIdJwtField,omitempty"`

	// JWT field containing team IDs (array).
	// +optional
	TeamIDsJWTField string `json:"teamIdsJwtField,omitempty"`

	// JWT field containing the organization ID.
	// +optional
	OrgIDJWTField string `json:"orgIdJwtField,omitempty"`

	// JWT field containing the user ID.
	// +optional
	UserIDJWTField string `json:"userIdJwtField,omitempty"`

	// UserIDUpsert auto-creates (upserts) a LiteLLM user record the first time
	// a JWT presents a user id that does not yet exist in the DB, instead of
	// rejecting the request. Renders litellm_jwtauth.user_id_upsert.
	// +optional
	UserIDUpsert *bool `json:"userIdUpsert,omitempty"`

	// JWT field containing the user email.
	// +optional
	UserEmailJWTField string `json:"userEmailJwtField,omitempty"`

	// JWT field containing the user role (a single role value).
	// +optional
	UserRoleJWTField string `json:"userRoleJwtField,omitempty"`

	// UserRolesJWTField is the JWT claim containing a LIST of the user's roles
	// (e.g. "roles"). Distinct from userRoleJwtField (a single role). Combined
	// with userAllowedRoles and enforceRbac this drives JWT role-based model
	// access (LiteLLM's role_permissions).
	// +optional
	UserRolesJWTField string `json:"userRolesJwtField,omitempty"`

	// UserAllowedRoles are the role values (read from userRolesJwtField) that
	// map to an internal_user on LiteLLM (e.g. ["basic_user"]). When
	// enforceRbac is true, callers whose roles are not listed are denied.
	// +optional
	UserAllowedRoles []string `json:"userAllowedRoles,omitempty"`

	// EnforceRBAC turns on JWT role-based access control: LiteLLM checks the
	// caller's role (from the JWT) against general_settings.role_permissions
	// before allowing model access. Rendered under litellm_jwtauth — distinct
	// from spec.rbac.enforceRbac, which is the general_settings-level toggle.
	// +optional
	EnforceRBAC *bool `json:"enforceRbac,omitempty"`

	// ObjectIDJWTField is the JWT claim holding an object id — either a user or
	// a team id, inferred from the role mapping.
	// +optional
	ObjectIDJWTField string `json:"objectIdJwtField,omitempty"`

	// JWT field containing the end-user ID.
	// +optional
	EndUserIDJWTField string `json:"endUserIdJwtField,omitempty"`

	// TTL in seconds for caching the public key.
	// +optional
	PublicKeyTTL *int `json:"publicKeyTtl,omitempty"`

	// ScopeModelMappings maps a JWT scope to a list of allowed model names.
	// Rendered into litellm_jwtauth.scope_mappings (the shape LiteLLM reads).
	// Prefer scopeMappings for new configs (it also supports per-scope routes);
	// both are merged into scope_mappings.
	// +optional
	ScopeModelMappings map[string][]string `json:"scopeModelMappings,omitempty"`

	// ScopeMappings restricts models (and optionally routes) per JWT scope.
	// Rendered into litellm_jwtauth.scope_mappings.
	// +optional
	ScopeMappings []JWTScopeMapping `json:"scopeMappings,omitempty"`

	// RolesJWTField is the JWT claim containing the caller's roles (used by
	// roleMappings). Rendered as roles_jwt_field.
	// +optional
	RolesJWTField string `json:"rolesJwtField,omitempty"`

	// RoleMappings map a JWT role value to a LiteLLM internal role.
	// +optional
	RoleMappings []JWTRoleMapping `json:"roleMappings,omitempty"`

	// JWTLiteLLMRoleMap maps a JWT role to a LiteLLM user role (proxy_admin,
	// internal_user, ...), with wildcard support. Rendered as
	// jwt_litellm_role_map.
	// +optional
	JWTLiteLLMRoleMap []JWTLiteLLMRoleMapEntry `json:"jwtLitellmRoleMap,omitempty"`

	// TeamAliasJWTField / OrgAliasJWTField look up a team / organization by its
	// name (alias) claim instead of its id.
	// +optional
	TeamAliasJWTField string `json:"teamAliasJwtField,omitempty"`
	// +optional
	OrgAliasJWTField string `json:"orgAliasJwtField,omitempty"`

	// TeamIDDefault is the fallback team id used when the JWT team claim does
	// not resolve.
	// +optional
	TeamIDDefault string `json:"teamIdDefault,omitempty"`

	// TeamAllowedRoutes restricts which routes team JWT holders may call.
	// +optional
	TeamAllowedRoutes []string `json:"teamAllowedRoutes,omitempty"`

	// TeamIDUpsert auto-creates a team on first sight of an unknown JWT team id.
	// +optional
	TeamIDUpsert *bool `json:"teamIdUpsert,omitempty"`

	// TeamClaimFallback defers to the user's single DB team when the JWT team
	// claim is unresolved.
	// +optional
	TeamClaimFallback *bool `json:"teamClaimFallback,omitempty"`

	// UserAllowedEmailDomain grants proxy access to any caller whose JWT email
	// belongs to this domain (e.g. "my-co.com").
	// +optional
	UserAllowedEmailDomain string `json:"userAllowedEmailDomain,omitempty"`

	// EnforceTeamBasedModelAccess denies model access unless the caller's team
	// has access to the model.
	// +optional
	EnforceTeamBasedModelAccess *bool `json:"enforceTeamBasedModelAccess,omitempty"`

	// EnforceScopeBasedAccess enforces model access via scopeMappings.
	// +optional
	EnforceScopeBasedAccess *bool `json:"enforceScopeBasedAccess,omitempty"`

	// SyncUserRoleAndTeams keeps the user's LiteLLM role and team memberships in
	// sync with the IdP on each request.
	// +optional
	SyncUserRoleAndTeams *bool `json:"syncUserRoleAndTeams,omitempty"`

	// CustomValidate is the dotted import path to a custom JWT-validation
	// function that must be present in the proxy image (same packaging pattern
	// as spec.sso.customSsoHandler in module mode). Rendered as custom_validate.
	// +optional
	CustomValidate string `json:"customValidate,omitempty"`

	// VirtualKeyClaimField is the JWT claim to look up in the virtual-key
	// mappings (JWT → virtual key). VirtualKeyMappingCacheTTL caches that lookup.
	// +optional
	VirtualKeyClaimField string `json:"virtualKeyClaimField,omitempty"`
	// +optional
	VirtualKeyMappingCacheTTL *int `json:"virtualKeyMappingCacheTtl,omitempty"`

	// RoutingOverrides route JWT-shaped tokens to OAuth2 handling based on
	// claims. Rendered as routing_overrides.
	// +optional
	RoutingOverrides []JWTRoutingOverride `json:"routingOverrides,omitempty"`

	// OIDCUserInfoEnabled fetches additional claims from the IdP UserInfo
	// endpoint (OIDCUserInfoEndpoint) and caches them for OIDCUserInfoCacheTTL
	// seconds.
	// +optional
	OIDCUserInfoEnabled *bool `json:"oidcUserinfoEnabled,omitempty"`
	// +optional
	OIDCUserInfoEndpoint string `json:"oidcUserinfoEndpoint,omitempty"`
	// +optional
	OIDCUserInfoCacheTTL *int `json:"oidcUserinfoCacheTtl,omitempty"`
}

// JWTScopeMapping restricts the models (and optionally routes) a JWT scope may
// access. Rendered as an entry in litellm_jwtauth.scope_mappings.
type JWTScopeMapping struct {
	// JWT scope value.
	Scope string `json:"scope"`
	// Allowed model names for this scope.
	// +optional
	Models []string `json:"models,omitempty"`
	// Allowed routes for this scope.
	// +optional
	Routes []string `json:"routes,omitempty"`
}

// JWTRoleMapping maps a JWT role value to a LiteLLM internal role
// (e.g. "team", "internal_user").
type JWTRoleMapping struct {
	// JWT role value.
	Role string `json:"role"`
	// LiteLLM internal role this JWT role maps to.
	InternalRole string `json:"internalRole"`
}

// JWTLiteLLMRoleMapEntry maps a JWT role to a LiteLLM user role.
type JWTLiteLLMRoleMapEntry struct {
	// JWT role value (e.g. "ADMIN").
	JWTRole string `json:"jwtRole"`
	// LiteLLM user role (e.g. "proxy_admin", "internal_user").
	LiteLLMRole string `json:"litellmRole"`
}

// JWTRoutingOverride routes JWT-shaped tokens to OAuth2 handling based on
// matching claims. Rendered as an entry in litellm_jwtauth.routing_overrides.
type JWTRoutingOverride struct {
	// Issuer claim to match (required).
	Iss string `json:"iss"`
	// Client id claim to match.
	// +optional
	ClientID string `json:"clientId,omitempty"`
	// Scope claim to match.
	// +optional
	Scope string `json:"scope,omitempty"`
	// Audience claim to match.
	// +optional
	Aud string `json:"aud,omitempty"`
	// Handling path (e.g. "oauth2").
	// +optional
	Path string `json:"path,omitempty"`
}

// OAuth2AuthSpec defines OAuth2 machine-to-machine authentication configuration (enterprise).
// Maps JWT fields to LiteLLM attributes for service-to-service authentication.
type OAuth2AuthSpec struct {
	// Enable OAuth2 machine-to-machine authentication.
	Enabled bool `json:"enabled"`

	// Mappings from JWT fields to LiteLLM attributes.
	// +optional
	ConfigMappings []OAuth2Mapping `json:"configMappings,omitempty"`
}

// OAuth2Mapping defines a single mapping from a JWT field to a LiteLLM attribute.
type OAuth2Mapping struct {
	// Identifier for this mapping.
	Name string `json:"name"`

	// JWT field to read from.
	JWTField string `json:"jwtField"`

	// LiteLLM attribute to map to (e.g., "team_id", "user_id").
	LiteLLMAttribute string `json:"litellmAttribute"`
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

	// Secret manager configuration status.
	// +optional
	SecretManager *SecretManagerStatus `json:"secretManager,omitempty"`

	// License activation status.
	// +optional
	License *LicenseStatus `json:"license,omitempty"`

	// Backup status (CloudNativePG).
	// +optional
	Backup *BackupStatus `json:"backup,omitempty"`

	// Last successful deployment revision for auto-rollback.
	// +optional
	LastSuccessfulRevision string `json:"lastSuccessfulRevision,omitempty"`

	// UnhealthyPods explains why proxy pods are not running — crash loops, image
	// pull failures, OOM kills, unschedulable pods — so the cause is visible from
	// the instance itself instead of requiring a dig through pod logs. Populated
	// only while the instance is not ready, and capped to keep the object small.
	// +optional
	// +kubebuilder:validation:MaxItems=3
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Unhealthy Pods"
	UnhealthyPods []UnhealthyPod `json:"unhealthyPods,omitempty"`

	// Standard Kubernetes conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// UnhealthyPod summarizes why a single proxy pod is not healthy.
type UnhealthyPod struct {
	// Pod name.
	Name string `json:"name"`

	// Pod phase (Pending, Running, Failed, ...).
	// +optional
	Phase string `json:"phase,omitempty"`

	// Machine-readable cause, e.g. CrashLoopBackOff, ImagePullBackOff,
	// OOMKilled, CreateContainerConfigError, Unschedulable.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human-readable detail (truncated).
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Message string `json:"message,omitempty"`

	// Container restart count — a climbing value is the signature of a crash loop.
	// +optional
	RestartCount int32 `json:"restartCount,omitempty"`
}

// SecretManagerStatus defines the observed state of the secret manager integration.
type SecretManagerStatus struct {
	// Whether the secret manager is configured.
	Configured bool `json:"configured"`

	// The configured provider.
	// +optional
	Provider string `json:"provider,omitempty"`
}

// LicenseStatus reflects license Secret detection.
type LicenseStatus struct {
	// Whether a license Secret was detected.
	Active bool `json:"active"`

	// Name of the Secret providing the license.
	// +optional
	SecretName string `json:"secretName,omitempty"`
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

	// Count of synced organizations.
	SyncedOrganizations int `json:"syncedOrganizations,omitempty"`

	// Count of synced customers.
	SyncedCustomers int `json:"syncedCustomers,omitempty"`

	// Count of unmanaged models.
	UnmanagedModels int `json:"unmanagedModels,omitempty"`

	// Count of unmanaged teams.
	UnmanagedTeams int `json:"unmanagedTeams,omitempty"`

	// Count of unmanaged users.
	UnmanagedUsers int `json:"unmanagedUsers,omitempty"`

	// Count of unmanaged keys.
	UnmanagedKeys int `json:"unmanagedKeys,omitempty"`

	// Count of unmanaged organizations.
	UnmanagedOrganizations int `json:"unmanagedOrganizations,omitempty"`

	// Count of unmanaged customers.
	UnmanagedCustomers int `json:"unmanagedCustomers,omitempty"`

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
// +operator-sdk:csv:customresourcedefinitions:displayName="LiteLLM Instance",resources={{ConfigMap,v1,""},{Deployment,apps/v1,""},{HorizontalPodAutoscaler,autoscaling/v2,""},{Ingress,networking.k8s.io/v1,""},{Job,batch/v1,""},{NetworkPolicy,networking.k8s.io/v1,""},{PodDisruptionBudget,policy/v1,""},{Secret,v1,""},{Service,v1,""},{ServiceAccount,v1,""}}

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

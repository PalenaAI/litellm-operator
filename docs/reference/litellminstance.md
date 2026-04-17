# LiteLLMInstance

The primary CRD. Deploys a LiteLLM proxy with all infrastructure dependencies.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMInstance`
**Short Name:** `li`

## Minimal Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: litellm-db
        key: DATABASE_URL
```

## Full Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  image:
    repository: ghcr.io/berriai/litellm
    tag: main-v1.60.0
    pullPolicy: IfNotPresent

  replicas: 3

  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilization: 70

  masterKey:
    autoGenerate: true

  database:
    external:
      connectionSecretRef:
        name: litellm-db
        key: DATABASE_URL
    connectionPool:
      maxConnections: 20
    migration:
      enabled: true
      timeout: "300s"

  redis:
    enabled: true
    host: redis.default.svc
    port: 6379
    passwordSecretRef:
      name: redis-credentials
      key: password

  saltKey:
    autoGenerate: true

  service:
    type: ClusterIP
    port: 4000

  ingress:
    enabled: true
    host: litellm.example.com
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt

  configSync:
    enabled: true
    interval: "30s"
    unmanagedResourcePolicy: preserve
    conflictResolution: crd-wins
    auditChanges: true

  caching:
    enabled: true
    type: redis
    ttl: 600
    namespace: "prod"
    mode: default_on

  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: "2"
      memory: 1Gi

  podDisruptionBudget:
    enabled: true
    minAvailable: 1

  security:
    networkPolicy:
      enabled: true
      allowedNamespaces:
        - default
        - production
    ipAllowlist:
      enabled: true
      allowedIPs:
        - "10.0.0.0/8"
        - "192.168.1.0/24"
      useXForwardedFor: true

  generalSettings:
    masterKeyRequired: true
    maxBudget: "10000.00"
    budgetDuration: "30d"
    globalMaxParallelRequests: 100
    budgetReschedulerMinTime: 300
    budgetReschedulerMaxTime: 600

  routerSettings:
    routingStrategy: least-busy
    numRetries: 3
    enableTagFiltering: true
    defaultMaxParallelRequests: 10
    retryPolicy:
      TimeoutError: 2
      RateLimitError: 3
    modelGroupRetryPolicy:
      gpt-4:
        TimeoutError: 1
    providerBudgetConfig:
      openai:
        budgetLimit: "500.00"
        timePeriod: "1d"
      anthropic:
        budgetLimit: "300.00"
        timePeriod: "1d"

  fallbacks:
    defaultFallbacks: ["gpt-4-mini", "claude-3-haiku"]
    modelFallbacks:
      - model: gpt-4
        fallbacks: ["gpt-4-mini", "claude-3-haiku"]
    contentPolicyFallbacks:
      - model: gpt-4
        fallbacks: ["claude-3-sonnet"]
    contextWindowFallbacks:
      - model: gpt-4
        fallbacks: ["gpt-4-32k", "claude-3-sonnet"]
    maxFallbacks: 3

  passThroughEndpoints:
    - path: /bria
      target: https://engine.prod.bria-api.com
      headers:
        content-type: application/json
      headerSecrets:
        - headerName: Authorization
          prefix: "Bearer "
          secretRef:
            name: bria-credentials
            key: api-key
    - path: /langfuse
      target: https://us.cloud.langfuse.com
      auth: true
      forwardHeaders: true
      includeSubpath: true
      methods: ["GET", "POST"]
      defaultQueryParams:
        version: "2"

  jwtAuth:
    enabled: true
    adminJwtScope: "litellm_proxy_admin"
    adminAllowedRoutes:
      - openai_routes
      - info_routes
    teamIdJwtField: "client_id"
    teamIdsJwtField: "groups"
    orgIdJwtField: "org_id"
    userIdJwtField: "sub"
    userEmailJwtField: "email"
    publicKeyTtl: 600

  oauth2Auth:
    enabled: true
    configMappings:
      - name: clientId
        jwtField: "client_id"
        litellmAttribute: "team_id"
      - name: userId
        jwtField: "sub"
        litellmAttribute: "user_id"

  secretManager:
    provider: aws_secret_manager
    credentialsSecretRef:
      name: litellm-aws-credentials
    hostedKeys:
      - OPENAI_API_KEY
      - ANTHROPIC_API_KEY
    storeVirtualKeys: true
    prefixForStoredVirtualKeys: "litellm/"
    accessMode: read_and_write
    aws:
      region: us-east-1

  rbac:
    enabled: true
    adminOnlyRoutes:
      - /model/new
      - /model/delete
      - /organization/new
    allowedRoutes:
      - /chat/completions
      - /embeddings
      - /key/info
      - /user/info
    defaultTeamDisabled: true
    keyGeneration:
      teamKeyGeneration:
        allowedTeamMemberRoles: ["admin"]
      personalKeyGeneration:
        allowedUserRoles: ["proxy_admin"]
    rolePermissions:
      internal_user:
        routes:
          - /key/generate
          - /key/delete
          - /key/info
        models:
          - gpt-4
          - claude-3-haiku

  logging:
    auditLogs:
      enabled: true
      retentionDays: 90
    turnOffMessageLogging: false
    redactUserApiKeyInfo: true
    spendLogRetention:
      maxRetentionPeriod: "90d"
      cleanupInterval: "1d"

  adminUI:
    disabled: false
    adminOnly: true
    storeModelInDB: true
    defaultTeamDisabled: true
    apiDocBaseURL: "https://api.example.com"
    docsURL: "/docs"
    rootRedirectURL: "/ui"
    logoURL: "https://example.com/logo.png"
    emailLogoURL: "https://example.com/email-logo.png"
    emailSupportContact: "support@example.com"
    colorThemeConfigMapRef:
      name: litellm-colors
```

## Spec Fields

### `image`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `repository` | string | `ghcr.io/berriai/litellm` | Container image repository |
| `tag` | string | `main-latest` | Image tag |
| `pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `pullSecrets` | []SecretRef | — | Image pull secrets |

### `replicas`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `replicas` | int32 | `1` | Number of proxy replicas |

### `autoscaling`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable HPA |
| `minReplicas` | int32 | `1` | Minimum replicas |
| `maxReplicas` | int32 | — | Maximum replicas (required if enabled) |
| `targetCPUUtilization` | *int32 | — | Target CPU percentage |
| `targetMemoryUtilization` | *int32 | — | Target memory percentage |

### `masterKey`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `secretRef` | *SecretKeyRef | — | Reference to existing master key Secret |
| `autoGenerate` | bool | `false` | Auto-generate and store in a Secret |

### `database`

| Field | Type | Description |
| --- | --- | --- |
| `external` | *ExternalDBSpec | External PostgreSQL connection |
| `cloudnativepg` | *CloudNativePGSpec | CloudNativePG cluster reference |
| `managed` | *ManagedDBSpec | Operator-managed single-pod PostgreSQL |
| `connectionPool` | *ConnectionPoolSpec | Connection pool settings |
| `migration` | *MigrationSpec | Database migration settings |

### `redis`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable Redis |
| `connectionSecretRef` | *SecretKeyRef | — | Redis connection URL Secret |
| `host` | string | — | Redis host |
| `port` | int | `6379` | Redis port |
| `passwordSecretRef` | *SecretKeyRef | — | Redis password Secret |

### `configSync`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable bidirectional config sync |
| `interval` | string | `30s` | Sync interval |
| `unmanagedResourcePolicy` | string | `preserve` | Policy for unmanaged resources: `preserve`, `prune`, `adopt` |
| `conflictResolution` | string | `crd-wins` | Conflict strategy: `crd-wins`, `api-wins`, `manual` |
| `auditChanges` | bool | `false` | Emit Kubernetes Events for sync changes |

### `service`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `type` | string | `ClusterIP` | Service type |
| `port` | int32 | `4000` | Service port |

### `generalSettings`

| Field | Type | Description |
| --- | --- | --- |
| `masterKeyRequired` | *bool | Require master key for all requests |
| `proxyBatchWriteAt` | int | Batch write interval in seconds |
| `alertTypes` | []string | Alert types for notifications |
| `allowUserAuth` | *bool | Allow requests without a key |
| `maxBudget` | *string | Global proxy budget in USD |
| `budgetDuration` | string | Global budget reset duration (e.g., `1d`, `7d`, `30d`) |
| `globalMaxParallelRequests` | *int | Maximum parallel requests across the entire proxy |
| `budgetReschedulerMinTime` | *int | Minimum interval (seconds) between budget reset checks |
| `budgetReschedulerMaxTime` | *int | Maximum interval (seconds) between budget reset checks |

### `routerSettings`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `routingStrategy` | string | — | `simple-shuffle`, `least-busy`, `latency-based-routing`, `usage-based-routing` |
| `numRetries` | *int | — | Number of retries |
| `timeout` | *int | — | Timeout in seconds |
| `cooldownTime` | *int | — | Cooldown time in seconds |
| `retryPolicy` | map[string]int | — | Per-error-type retry counts (e.g., `TimeoutError: 2`, `RateLimitError: 3`) |
| `modelGroupRetryPolicy` | map[string]map[string]int | — | Per-model-group retry overrides (e.g., `gpt-4: {TimeoutError: 1}`) |
| `enableTagFiltering` | *bool | — | Enable tag-based routing. Requests with matching tags route to tagged model deployments |
| `tagFilteringMatchAny` | *bool | — | If true, match ANY tag (OR logic). If false (default), ALL tags must match (AND logic) |
| `defaultMaxParallelRequests` | *int | — | Default max parallel requests per model deployment |
| `providerBudgetConfig` | map[string]ProviderBudget | — | Per-provider spending limits |

**ProviderBudget:**

| Field | Type | Description |
| --- | --- | --- |
| `budgetLimit` | string | Budget limit in USD |
| `timePeriod` | string | Time period for the budget (e.g., `1d`, `7d`, `30d`) |

### `fallbacks`

Configure model fallback chains. Fallback model names must match `model_name` values from your `LiteLLMModel` resources.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `defaultFallbacks` | []string | — | Global fallback models applied on any error |
| `modelFallbacks` | []ModelFallbackEntry | — | Per-model fallback chains for general errors |
| `contentPolicyFallbacks` | []ModelFallbackEntry | — | Fallbacks for content policy violations |
| `contextWindowFallbacks` | []ModelFallbackEntry | — | Fallbacks for context window exceeded errors |
| `maxFallbacks` | *int | `3` | Maximum number of fallback attempts |

**ModelFallbackEntry:**

| Field | Type | Description |
| --- | --- | --- |
| `model` | string | Primary model name |
| `fallbacks` | []string | Ordered list of fallback model names |

### `caching`

Response caching configuration. See the [Caching guide](/guide/caching) for detailed examples.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable response caching |
| `type` | string | `redis` | Cache backend: `redis`, `redis-semantic`, `s3`, `gcs`, `qdrant`, `local` |
| `ttl` | *int | `600` | Cache TTL in seconds |
| `namespace` | string | — | Cache key namespace for isolation |
| `supportedCallTypes` | []string | — | Restrict caching to specific call types |
| `mode` | string | `default_on` | `default_on` (cache all) or `default_off` (require opt-in) |
| `redis` | *CacheRedisSpec | — | Redis-specific config (omit to reuse `spec.redis`) |
| `s3` | *CacheS3Spec | — | S3 backend config |
| `gcs` | *CacheGCSSpec | — | GCS backend config |
| `qdrant` | *CacheQdrantSpec | — | Qdrant semantic cache config |

**CacheRedisSpec:**

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `host` | string | — | Redis host |
| `port` | *int | — | Redis port |
| `passwordSecretRef` | *SecretKeyRef | — | Redis password Secret reference |
| `ssl` | bool | `false` | Enable SSL/TLS |

**CacheS3Spec:**

| Field | Type | Description |
| --- | --- | --- |
| `bucketName` | string | S3 bucket name (required) |
| `region` | string | AWS region |
| `credentialsSecretRef` | *SecretKeyRef | Secret with `aws_access_key_id` and `aws_secret_access_key` keys |

**CacheGCSSpec:**

| Field | Type | Description |
| --- | --- | --- |
| `bucketName` | string | GCS bucket name (required) |
| `credentialsSecretRef` | *SecretKeyRef | Secret with GCS service account JSON |

**CacheQdrantSpec:**

| Field | Type | Description |
| --- | --- | --- |
| `url` | string | Qdrant server URL (required) |
| `apiKeySecretRef` | *SecretKeyRef | Qdrant API key Secret reference |
| `collectionName` | string | Collection name for cached embeddings |

### `passThroughEndpoints`

Configure pass-through endpoints to proxy arbitrary API requests to upstream services through LiteLLM. Useful for provider-specific APIs (image generation, fine-tuning, embeddings) that aren't covered by the standard chat/completion routing.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `path` | string | — | Route path on the LiteLLM proxy (e.g., `/bria`) (required) |
| `target` | string | — | Target URL to forward requests to (required) |
| `auth` | *bool | — | Enable LiteLLM authentication for this endpoint (enterprise) |
| `forwardHeaders` | *bool | — | Forward incoming client headers to the target |
| `includeSubpath` | *bool | — | Forward requests to sub-paths (e.g., `/path/sub` → `target/sub`) |
| `methods` | []string | — | HTTP methods to allow. If empty, all methods are allowed |
| `headers` | map[string]string | — | Static headers to add to forwarded requests |
| `headerSecrets` | []HeaderSecretRef | — | Headers sourced from Kubernetes Secrets |
| `defaultQueryParams` | map[string]string | — | Default query parameters added to all forwarded requests |

**HeaderSecretRef:**

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `headerName` | string | — | HTTP header name (e.g., `Authorization`) (required) |
| `prefix` | string | — | Prefix prepended to the secret value (e.g., "Bearer ") |
| `secretRef` | SecretKeyRef | — | Reference to Secret containing the header value (required) |

Secret-backed headers are injected as environment variables named `PASSTHROUGH_{PATH}_{HEADER}` (uppercase, special characters replaced with `_`). In the generated config, they appear as `os.environ/PASSTHROUGH_...` references that LiteLLM resolves at runtime.

### `secretManager`

External secret manager integration. When configured, LiteLLM fetches API keys from the provider at runtime instead of reading them from Kubernetes Secrets.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `provider` | string | — | Provider name (required). One of: `aws_secret_manager`, `aws_kms`, `azure_key_vault`, `google_secret_manager`, `google_kms`, `hashicorp_vault` |
| `credentialsSecretRef` | *SecretRef | — | Reference to a Secret containing provider authentication fields. Omit when using workload identity (IRSA, GKE WI) |
| `hostedKeys` | []string | — | Env var names LiteLLM should resolve from the secret manager |
| `storeVirtualKeys` | *bool | — | Store generated virtual keys in the secret manager |
| `prefixForStoredVirtualKeys` | string | — | Prefix for stored virtual keys (e.g., `litellm/`) |
| `accessMode` | string | `read_only` | Access mode: `read_only`, `write_only`, `read_and_write` |
| `primarySecretName` | string | — | Single secret containing multiple key-value pairs as JSON |
| `aws` | *AWSSecretManagerConfig | — | AWS-specific settings |
| `azure` | *AzureKeyVaultConfig | — | Azure-specific settings |
| `vault` | *VaultConfig | — | HashiCorp Vault-specific settings |

**AWSSecretManagerConfig:**

| Field | Type | Description |
| --- | --- | --- |
| `region` | string | AWS region (required) |
| `roleARN` | string | IAM role ARN for role assumption |
| `sessionName` | string | Session name for role assumption |
| `webIdentityTokenPath` | string | Path to web identity token file (for IRSA on EKS) |
| `stsEndpoint` | string | Custom STS endpoint (for VPC environments) |

**AzureKeyVaultConfig:**

| Field | Type | Description |
| --- | --- | --- |
| `vaultURI` | string | Azure Key Vault URI (required) |
| `tenantID` | string | Azure tenant ID (required) |

**VaultConfig:**

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `address` | string | — | Vault server address (required) |
| `namespace` | string | — | Vault namespace |
| `authMethod` | string | `approle` | Auth method: `approle`, `tls`, or `token` |
| `appRoleMountPath` | string | — | AppRole mount path |
| `mountName` | string | — | KV engine mount name |
| `pathPrefix` | string | — | Path prefix for secrets |
| `refreshInterval` | *int | — | Cache refresh interval in seconds |

See the [Secret Managers guide](/guide/secret-managers) for full usage examples per provider.

### `jwtAuth`

JWT-based authentication configuration (enterprise). When enabled, LiteLLM validates JWT tokens from identity providers and maps claims to roles, teams, and organizations. Public keys are fetched automatically from the IdP's JWKS endpoint. See the [JWT/OAuth2 Auth guide](/guide/jwt-oauth2-auth) for detailed examples.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable JWT-based authentication |
| `adminJwtScope` | string | — | JWT scope value that grants proxy admin access |
| `adminAllowedRoutes` | []string | — | Routes accessible to admin JWT holders (e.g., `openai_routes`, `info_routes`) |
| `teamIdJwtField` | string | — | JWT field containing the team ID |
| `teamIdsJwtField` | string | — | JWT field containing team IDs (array) |
| `orgIdJwtField` | string | — | JWT field containing the organization ID |
| `userIdJwtField` | string | — | JWT field containing the user ID |
| `userEmailJwtField` | string | — | JWT field containing the user email |
| `userRoleJwtField` | string | — | JWT field containing the user role |
| `endUserIdJwtField` | string | — | JWT field containing the end-user ID |
| `publicKeyTtl` | *int | — | TTL in seconds for caching the public key |
| `scopeModelMappings` | map[string][]string | — | Scope-to-model mappings (key: JWT scope, value: allowed model names) |

### `oauth2Auth`

OAuth2 machine-to-machine authentication configuration (enterprise). Maps JWT fields to LiteLLM attributes for service-to-service authentication. See the [JWT/OAuth2 Auth guide](/guide/jwt-oauth2-auth) for detailed examples.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable OAuth2 machine-to-machine authentication |
| `configMappings` | []OAuth2Mapping | — | Mappings from JWT fields to LiteLLM attributes |

**OAuth2Mapping:**

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Identifier for this mapping (required) |
| `jwtField` | string | JWT field to read from (required) |
| `litellmAttribute` | string | LiteLLM attribute to map to, e.g., `team_id`, `user_id` (required) |

### `security`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `networkPolicy.enabled` | bool | `false` | Enable NetworkPolicy |
| `networkPolicy.allowedNamespaces` | []string | — | Namespaces allowed ingress access |
| `runAsNonRoot` | *bool | — | Run as non-root user (OpenShift compatible) |
| `ipAllowlist` | *IPAllowlistSpec | — | IP address filtering (enterprise) |

**IPAllowlistSpec:**

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable IP address filtering |
| `allowedIPs` | []string | — | Allowed IP addresses or CIDR ranges (required, min 1) |
| `useXForwardedFor` | *bool | — | Use `X-Forwarded-For` header for client IP detection. Enable when behind a load balancer |
| `maxRequestSizeMB` | *int | — | Maximum request body size in MB (enterprise) |
| `maxResponseSizeMB` | *int | — | Maximum response body size in MB (enterprise) |

### `rbac`

Role-based access control configuration. Controls route restrictions, key generation permissions, and per-role access. Some features require a LiteLLM Enterprise license. See the [RBAC guide](/guide/rbac) for detailed examples.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable RBAC enforcement (`enforce_rbac: true` in config) |
| `adminOnlyRoutes` | []string | — | Routes restricted to `proxy_admin` only |
| `allowedRoutes` | []string | — | Routes accessible to all authenticated users. If set, all other routes are blocked |
| `defaultTeamDisabled` | *bool | — | Prevent personal key creation (force team-based keys) |
| `keyGeneration` | *KeyGenerationSettings | — | Key generation restrictions (enterprise) |
| `rolePermissions` | map[string]RolePermission | — | Per-role permission definitions (enterprise) |

**KeyGenerationSettings:**

| Field | Type | Description |
| --- | --- | --- |
| `teamKeyGeneration` | *TeamKeyGenerationSettings | Team key generation restrictions |
| `personalKeyGeneration` | *PersonalKeyGenerationSettings | Personal key generation restrictions |

**TeamKeyGenerationSettings:**

| Field | Type | Description |
| --- | --- | --- |
| `allowedTeamMemberRoles` | []string | Team member roles allowed to generate team keys. Values: `admin`, `user` |

**PersonalKeyGenerationSettings:**

| Field | Type | Description |
| --- | --- | --- |
| `allowedUserRoles` | []string | User roles allowed to generate personal keys. Values: `proxy_admin`, `proxy_admin_viewer`, `internal_user`, `internal_user_viewer` |

**RolePermission:**

| Field | Type | Description |
| --- | --- | --- |
| `routes` | []string | API routes this role can access |
| `models` | []string | Models this role can use |

### `logging`

Instance-level logging configuration. Controls audit logs, message content logging, API key redaction, and spend log retention.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `auditLogs` | *AuditLogSpec | — | Audit log configuration (enterprise) |
| `turnOffMessageLogging` | *bool | — | Disable logging of request/response message content. Only metadata (tokens, cost, model) is logged |
| `redactUserApiKeyInfo` | *bool | — | Redact user API key information from logs |
| `spendLogRetention` | *SpendLogRetentionSpec | — | Spend log retention and cleanup configuration |

**AuditLogSpec:**

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enable audit logging (enterprise). Writes `store_audit_logs: true` to `general_settings` |
| `retentionDays` | *int | — | Audit log retention period in days. Written to `litellm_settings.audit_log_retention_days` |

**SpendLogRetentionSpec:**

| Field | Type | Description |
| --- | --- | --- |
| `maxRetentionPeriod` | string | Maximum retention period (e.g., `90d`, `1y`). Written to `general_settings.maximum_spend_logs_retention_period` |
| `cleanupInterval` | string | Cleanup interval (e.g., `1d`, `1h`). Written to `general_settings.maximum_spend_logs_retention_interval` |

### `extraEnvVars` / `extraEnvFrom`

Inject additional environment variables into the LiteLLM container.

| Field | Type | Description |
| --- | --- | --- |
| `extraEnvVars` | []corev1.EnvVar | Extra env vars appended to the container. Entries override operator-set env vars of the same name (e.g. `PROXY_BASE_URL`) |
| `extraEnvFrom` | []corev1.EnvFromSource | Extra `envFrom` sources (Secret or ConfigMap) mounted into the container |

::: tip
`extraEnvVars` is an escape hatch for env vars the operator derives automatically. A common case is `PROXY_BASE_URL`: the operator derives it from `spec.ingress.host`, falling back to the in-cluster Service DNS when ingress is not used. If you expose the gateway through an OpenShift Route, a Gateway API HTTPRoute, or an external load balancer, set `PROXY_BASE_URL` here to your public URL so SSO redirects land on the correct host.

```yaml
spec:
  extraEnvVars:
    - name: PROXY_BASE_URL
      value: https://gateway.example.com
```

:::

### `adminUI`

Admin UI configuration. Controls UI availability, access restrictions, model persistence, and personal key creation policy.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `disabled` | *bool | — | Disable the Admin UI entirely. When `true`, the `/ui` endpoint returns 404. Injected as `DISABLE_ADMIN_UI` env var |
| `adminOnly` | *bool | — | Restrict UI access to admin users only (`proxy_admin` and `proxy_admin_viewer`). Sets `ui_access_mode: "admin_only"` in `general_settings` |
| `storeModelInDB` | *bool | — | Store model definitions in the database instead of the config file. Enables adding/editing models from the UI without proxy restart. Written to `general_settings.store_model_in_db` |
| `defaultTeamDisabled` | *bool | — | Prevent users from creating personal API keys. Keys can only be created under an assigned team. Written to `general_settings.default_team_disabled` |
| `apiDocBaseURL` | string | — | Custom base URL for the API reference documentation shown in the UI. Useful when the Admin UI is served from a different host. Injected as `LITELLM_UI_API_DOC_BASE_URL` env var |
| `docsURL` | string | — | Custom path for the docs endpoint (default: `/`). Injected as `DOCS_URL` env var |
| `rootRedirectURL` | string | — | URL to redirect to when the root path is accessed and `docsURL` is changed. Injected as `ROOT_REDIRECT_URL` env var |
| `logoURL` | string | — | URL to a hosted logo image displayed in the Admin UI. Injected as `UI_LOGO_PATH` env var |
| `emailLogoURL` | string | — | URL to a logo image included in email notifications (budget alerts, invitations). Injected as `EMAIL_LOGO_URL` env var |
| `emailSupportContact` | string | — | Support email address displayed in email notifications. Injected as `EMAIL_SUPPORT_CONTACT` env var |
| `colorThemeConfigMapRef` | *ConfigMapRef | — | Reference to a ConfigMap containing `enterprise_colors.json` with custom brand colors ([Tremor palette](https://www.tremor.so/docs/layout/color-palette#default-colors)). Mounted at `/app/enterprise/enterprise_ui/enterprise_colors.json` |

::: tip
`storeModelInDB: true` is recommended for production multi-replica deployments where models should persist across restarts and be shared across instances.

`defaultTeamDisabled: true` pairs well with the operator's `LiteLLMTeam` CRD — it enforces that all keys are team-scoped, making spend tracking and access control more predictable.
:::

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `ready` | bool | Whether the instance is fully ready |
| `replicas` | int32 | Current replica count |
| `readyReplicas` | int32 | Ready replica count |
| `endpoint` | string | Internal cluster endpoint URL |
| `version` | string | Current LiteLLM version |
| `database` | DatabaseStatus | Database connection status |
| `redis` | *RedisStatus | Redis connection status |
| `configSync` | *ConfigSyncStatus | Config sync status and counts |
| `secretManager` | *SecretManagerStatus | Secret manager status (`configured`, `provider`) |
| `license` | *LicenseStatus | License activation status (`active`, `secretName`) |
| `sso` | *SSOStatus | SSO configuration status |
| `scim` | *SCIMStatus | SCIM configuration status |
| `conditions` | []Condition | Standard Kubernetes conditions |

## Print Columns

```bash
kubectl get li
NAME         READY   ENDPOINT                              VERSION          AGE
my-gateway   True    http://my-gateway.default.svc:4000    main-v1.60.0     5d
```

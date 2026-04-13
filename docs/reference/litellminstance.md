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

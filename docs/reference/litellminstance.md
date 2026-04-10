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

  routerSettings:
    routingStrategy: least-busy
    numRetries: 3
    enableTagFiltering: true
    retryPolicy:
      TimeoutError: 2
      RateLimitError: 3
    modelGroupRetryPolicy:
      gpt-4:
        TimeoutError: 1

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

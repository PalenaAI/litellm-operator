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

  generalSettings:
    masterKeyRequired: true

  routerSettings:
    routingStrategy: least-busy
    numRetries: 3
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

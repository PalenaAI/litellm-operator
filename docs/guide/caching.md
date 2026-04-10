# Response Caching

LiteLLM supports response caching to reduce latency and costs. The operator configures caching via the `spec.caching` field on `LiteLLMInstance`, which translates to `litellm_settings.cache` and `litellm_settings.cache_params` in the proxy config.

## Supported Backends

| Backend | `type` value | Description |
| --- | --- | --- |
| Redis | `redis` | Redis key-value cache (recommended for production) |
| Redis Semantic | `redis-semantic` | Semantic similarity cache using Redis + embeddings |
| S3 | `s3` | Amazon S3 object storage |
| GCS | `gcs` | Google Cloud Storage |
| Qdrant | `qdrant` | Semantic cache using Qdrant vector database |
| Local | `local` | In-memory cache (single-pod only, not shared across replicas) |

## Basic Setup (Redis)

The simplest setup uses the same Redis instance already configured for rate limiting:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  redis:
    enabled: true
    host: redis.default.svc
    port: 6379
    passwordSecretRef:
      name: redis-credentials
      key: password
  caching:
    enabled: true
    type: redis
    ttl: 600
  # ... rest of spec
```

When `type: redis` and no `caching.redis` block is provided, the operator reuses the connection details from `spec.redis`. No need to duplicate host/port/password.

## Dedicated Cache Redis

To use a separate Redis instance for caching:

```yaml
spec:
  caching:
    enabled: true
    type: redis
    ttl: 300
    namespace: "my-app"
    redis:
      host: cache-redis.default.svc
      port: 6380
      passwordSecretRef:
        name: cache-redis-secret
        key: password
      ssl: true
```

## S3 Backend

```yaml
spec:
  caching:
    enabled: true
    type: s3
    ttl: 3600
    s3:
      bucketName: my-llm-cache
      region: us-east-1
      credentialsSecretRef:
        name: aws-credentials
        key: credentials
```

The Secret must contain keys `aws_access_key_id` and `aws_secret_access_key`.

## GCS Backend

```yaml
spec:
  caching:
    enabled: true
    type: gcs
    gcs:
      bucketName: my-llm-cache
      credentialsSecretRef:
        name: gcs-credentials
        key: service-account.json
```

## Qdrant Semantic Cache

Semantic caching matches requests by embedding similarity rather than exact match. Requires an embedding model to be configured in LiteLLM.

```yaml
spec:
  caching:
    enabled: true
    type: qdrant
    qdrant:
      url: http://qdrant.default.svc:6333
      collectionName: llm-cache
      apiKeySecretRef:
        name: qdrant-secret
        key: api-key
```

## Local (In-Memory) Cache

Useful for development or single-replica deployments. Not shared across pods.

```yaml
spec:
  caching:
    enabled: true
    type: local
    ttl: 120
```

## Cache Options

### TTL

Cache time-to-live in seconds. Default: `600` (10 minutes).

```yaml
spec:
  caching:
    enabled: true
    type: redis
    ttl: 1800  # 30 minutes
```

### Namespace Isolation

Isolate cache keys with a namespace prefix. Useful when multiple LiteLLM instances share a Redis cluster:

```yaml
spec:
  caching:
    enabled: true
    type: redis
    namespace: "team-a"
```

### Call Type Filtering

Restrict caching to specific LiteLLM call types:

```yaml
spec:
  caching:
    enabled: true
    type: redis
    supportedCallTypes:
      - acompletion
      - aembedding
      - atranscription
```

### Default-Off Mode

Require explicit opt-in per request (via `cache` parameter in the API call):

```yaml
spec:
  caching:
    enabled: true
    type: redis
    mode: default_off
```

In `default_on` mode (the default), all matching requests are cached automatically.

## Secret Handling

Cache backend credentials are never stored in the ConfigMap. The operator:

1. Writes `os.environ/CACHE_*` references in `proxy_server_config.yaml`
2. Injects the actual values as environment variables from Kubernetes Secrets

| Backend | Env Var | Source |
| --- | --- | --- |
| Redis (dedicated) | `CACHE_REDIS_PASSWORD` | `caching.redis.passwordSecretRef` |
| Redis (reused) | `REDIS_PASSWORD` | `redis.passwordSecretRef` |
| S3 | `CACHE_S3_ACCESS_KEY_ID`, `CACHE_S3_SECRET_ACCESS_KEY` | `caching.s3.credentialsSecretRef` |
| GCS | `CACHE_GCS_SERVICE_ACCOUNT_JSON` | `caching.gcs.credentialsSecretRef` |
| Qdrant | `CACHE_QDRANT_API_KEY` | `caching.qdrant.apiKeySecretRef` |

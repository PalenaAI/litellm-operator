# LiteLLM Operator

A Kubernetes operator for deploying and managing production-ready [LiteLLM](https://github.com/BerriAI/litellm) AI Gateway instances. Built with [Operator SDK](https://sdk.operatorframework.io/) for OLM integration, OperatorHub distribution, and first-class OpenShift support.

Replaces manual Helm-based deployments with a declarative, reconciliation-based approach that keeps CRD state and the LiteLLM API in sync.

## Features

- **Declarative LiteLLM deployment** — manage proxy instances, models, teams, users, and API keys as Kubernetes custom resources
- **Bidirectional config sync** — reconciles CRD state with the LiteLLM REST API on every sync interval
- **Team member management** — three modes: `crd` (CRD authoritative), `sso` (IdP authoritative), `mixed` (additive)
- **VirtualKey secret management** — generated API keys are stored in Kubernetes Secrets with owner references for automatic cleanup
- **SSO/SCIM support** — configure Azure Entra ID, Okta, Google, or generic OIDC providers declaratively
- **Flexible ingress** — Kubernetes Ingress, OpenShift Route, and Gateway API HTTPRoute support
- **Production-ready** — HPA, PDB, NetworkPolicy, health checks, resource limits, security contexts
- **OpenShift / non-root support** — `spec.security.runAsNonRoot: true` automatically uses the official non-root image and applies restricted security contexts
- **Multiple install methods** — OLM bundles (OperatorHub/OpenShift) or Helm chart
- **CloudNativePG backup/restore** — scheduled backups via CNPG `ScheduledBackup` CRs with configurable schedule, retention, and method
- **Enterprise license activation** — convention-based license Secret detection (`{instance}-license` or `litellm-license`) with automatic `LITELLM_LICENSE` env var injection
- **Auto-rollback** — automatically rolls back failed deployments when `spec.upgrade.autoRollback: true`
- **Response caching** — 6 cache backends (Redis, S3, GCS, Qdrant semantic, Redis semantic, local) with TTL, namespace isolation, call-type filtering, and default-off mode
- **Tag-based routing** — route requests to model deployments by tags, assign tags to teams for team-scoped routing
- **Fallback chains** — default fallbacks, per-model fallbacks, content policy fallbacks, context window fallbacks, and per-error-type retry policies
- **Pass-through endpoints** — proxy arbitrary API requests to upstream services (image generation, fine-tuning, etc.) with static and secret-backed headers, sub-path routing, and optional LiteLLM authentication
- **IP allowlisting (enterprise)** — restrict API access to specific IP addresses or CIDR ranges via `spec.security.ipAllowlist`, with `X-Forwarded-For` support and max request/response size limits
- **Prometheus integration** — ServiceMonitor and PrometheusRule with six built-in alerts (instance down, degraded, pod restarts, not ready, high memory, high CPU) and runbooks
- **Grafana dashboard** — auto-provisioned dashboard via ConfigMap with replica status, resource usage, and deployment condition panels

## Custom Resource Definitions

| CRD | Short Name | Description |
| --- | ---------- | ----------- |
| `LiteLLMInstance` | `li` | Deploys a LiteLLM proxy with database, Redis, networking, and SSO |
| `LiteLLMModel` | `lm` | Registers a model (e.g., `openai/gpt-4o`) with the proxy |
| `LiteLLMTeam` | `lt` | Creates a team with budget limits and member management |
| `LiteLLMUser` | `lu` | Creates a user (service accounts, bot users, non-SSO environments) |
| `LiteLLMVirtualKey` | `lk` | Generates an API key scoped to a team/user with budget and rate limits |

All secondary resources (`LiteLLMModel`, `LiteLLMTeam`, `LiteLLMUser`, `LiteLLMVirtualKey`) reference a `LiteLLMInstance` via `spec.instanceRef`.

## Prerequisites

- Go 1.22+
- Docker 17.03+
- kubectl v1.28+
- Access to a Kubernetes v1.28+ cluster
- A PostgreSQL database for LiteLLM state storage

## Quick Start

### 1. Install CRDs

```sh
make install
```

### 2. Deploy the operator

```sh
make deploy IMG=ghcr.io/palenaai/litellm-operator:latest
```

### 3. Create a database secret

```sh
kubectl create secret generic litellm-db-credentials \
  --from-literal=DATABASE_URL='postgresql://user:pass@host:5432/litellm'
```

### 4. Deploy a LiteLLM instance

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  replicas: 2
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: litellm-db-credentials
        key: DATABASE_URL
  service:
    type: ClusterIP
    port: 4000
```

### 5. Register a model

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMModel
metadata:
  name: gpt4o
spec:
  instanceRef:
    name: my-gateway
  modelName: gpt-4o
  litellmParams:
    model: openai/gpt-4o
    apiKeySecretRef:
      name: openai-credentials
      key: OPENAI_API_KEY
```

### 6. Create a team and API key

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: engineering
spec:
  instanceRef:
    name: my-gateway
  teamAlias: engineering
  models: [gpt-4o]
  maxBudgetMonthly: 1000
  budgetDuration: "30d"
  members:
    - email: dev@example.com
      role: user
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: eng-ci-key
spec:
  instanceRef:
    name: my-gateway
  keyAlias: eng-ci-key
  teamRef:
    name: engineering
  models: [gpt-4o]
  maxBudget: "100"
```

### OpenShift / Non-Root Environments

For OpenShift or clusters enforcing Pod Security Standards, enable non-root mode:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  security:
    runAsNonRoot: true
  # ... rest of spec
```

This automatically switches to the official `litellm-non_root` image (runs as `nobody`, UID 65534) and applies a restricted pod security context compatible with OpenShift's restricted SCC.

### IP Allowlisting (Enterprise)

Restrict API access to specific IP addresses or CIDR ranges:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  security:
    ipAllowlist:
      enabled: true
      allowedIPs:
        - "10.0.0.0/8"
        - "192.168.1.0/24"
        - "203.0.113.50"
      useXForwardedFor: true  # required behind load balancers
  # ... rest of spec
```

### OpenShift Route

For OpenShift clusters, create a Route instead of an Ingress:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  route:
    enabled: true
    host: litellm.apps.example.com
    tlsTermination: edge   # edge | passthrough | reencrypt
  # ... rest of spec
```

### Gateway API HTTPRoute

For clusters using the [Gateway API](https://gateway-api.sigs.k8s.io/) (Istio, Envoy Gateway, Cilium, etc.):

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  gatewayHTTPRoute:
    enabled: true
    host: litellm.example.com
    parentRefs:
      - name: my-gateway       # Name of the Gateway resource
        namespace: istio-system # Optional: namespace of the Gateway
        sectionName: https     # Optional: specific listener on the Gateway
  # ... rest of spec
```

### Observability (Prometheus + Grafana)

Enable ServiceMonitor, alerting rules, and a Grafana dashboard:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  observability:
    serviceMonitor:
      enabled: true
      interval: "30s"
    prometheusRule:
      enabled: true
      # disabledAlerts: ["LiteLLMHighCPUUsage"]  # optionally disable specific alerts
    grafanaDashboard:
      enabled: true
      folder: "LiteLLM"
  # ... rest of spec
```

Built-in alerts: `LiteLLMInstanceDown` (critical), `LiteLLMInstanceDegraded`, `LiteLLMPodRestarting`, `LiteLLMPodNotReady`, `LiteLLMHighMemoryUsage`, `LiteLLMHighCPUUsage`. Each alert includes a runbook annotation with troubleshooting commands.

### CloudNativePG Backups

When using CloudNativePG for the database, enable scheduled backups:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  database:
    cloudnativepg:
      clusterName: litellm-db
      backup:
        enabled: true
        schedule: "0 2 * * *"   # daily at 2am
        retention: 7
        method: snapshot        # snapshot or barmanObjectStore
  # ... rest of spec
```

### Tag-Based Routing

Route requests to different model deployments based on tags. Useful for free/paid tiers or team-specific model access:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  routerSettings:
    enableTagFiltering: true
  # ... rest of spec
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMModel
metadata:
  name: gpt4-paid
spec:
  instanceRef:
    name: my-gateway
  modelName: gpt-4
  litellmParams:
    model: openai/gpt-4
    apiKeySecretRef:
      name: openai-credentials
      key: OPENAI_API_KEY
  tags: ["paid"]
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: paid-tier
spec:
  instanceRef:
    name: my-gateway
  teamAlias: paid-tier
  tags: ["paid"]
```

Requests from the `paid-tier` team are routed to model deployments tagged `paid`. Use `tagFilteringMatchAny: true` in `routerSettings` to match requests having ANY of the specified tags (default is ALL must match).

### Fallback Chains

Configure model fallback routing so requests automatically try alternative models on failure:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  fallbacks:
    # Global fallbacks applied on any error
    defaultFallbacks: ["gpt-4-mini", "claude-3-haiku"]

    # Per-model fallbacks for general errors
    modelFallbacks:
      - model: gpt-4
        fallbacks: ["gpt-4-mini", "claude-3-haiku"]

    # Fallbacks for content policy violations
    contentPolicyFallbacks:
      - model: gpt-4
        fallbacks: ["claude-3-sonnet"]

    # Fallbacks for context window exceeded
    contextWindowFallbacks:
      - model: gpt-4
        fallbacks: ["gpt-4-32k", "claude-3-sonnet"]

    maxFallbacks: 3

  routerSettings:
    # Retry policy by error type (retries on same model before fallback)
    retryPolicy:
      TimeoutError: 2
      RateLimitError: 3
      ContentPolicyViolationError: 0
    # Per-model-group retry overrides
    modelGroupRetryPolicy:
      gpt-4:
        TimeoutError: 1
        RateLimitError: 0
  # ... rest of spec
```

### Response Caching

Configure response caching to reduce latency and costs:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  caching:
    enabled: true
    type: redis             # redis, redis-semantic, s3, gcs, qdrant, local
    ttl: 600                # cache TTL in seconds
    namespace: "my-app"     # key isolation namespace
    mode: default_on        # default_on or default_off
    supportedCallTypes:     # restrict to specific call types
      - acompletion
      - aembedding
    redis:
      host: redis.example.com
      port: 6379
      passwordSecretRef:
        name: redis-secret
        key: password
      ssl: true
  # ... rest of spec
```

When `type: redis` and no `caching.redis` block is provided, the operator reuses the instance's existing `spec.redis` connection — no need to duplicate Redis details.

Other backends: `s3` (with bucket, region, AWS credentials), `gcs` (with bucket, GCS service account), `qdrant` (semantic caching with embeddings), `local` (in-memory, no external dependencies).

### Auto-Rollback

Automatically rollback failed deployments:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  upgrade:
    strategy: rolling
    autoRollback: true
    healthCheckTimeout: "300s"
  # ... rest of spec
```

When enabled, the operator tracks the last successful deployment revision. If a new deployment exceeds the progress deadline, the operator triggers a rollback and sets a status condition.

### Enterprise License

To activate LiteLLM Enterprise features, create a Secret with your license key. The operator detects it automatically and injects the `LITELLM_LICENSE` environment variable into the proxy Deployment.

**Per-instance license** (takes precedence):

```sh
kubectl create secret generic my-gateway-license \
  --from-literal=license-key='your-litellm-enterprise-license-key'
```

**Namespace-wide license** (fallback for all instances in the namespace):

```sh
kubectl create secret generic litellm-license \
  --from-literal=license-key='your-litellm-enterprise-license-key'
```

The operator checks for `{instance-name}-license` first, then falls back to `litellm-license`. License status is reported in `.status.license`:

```sh
kubectl get litellminstance my-gateway -o jsonpath='{.status.license}'
# {"active":true,"secretName":"my-gateway-license"}
```

If a downstream resource (Model, Team, User, VirtualKey) requires an enterprise feature and no license is present, the operator sets `Reason: EnterpriseLicenseRequired` on the resource's status condition without retrying.

### Namespace-Scoped Watching

By default, the operator watches all namespaces. To restrict it to specific namespaces:

**Helm:**

```bash
helm install litellm-operator deploy/charts/litellm-operator/ \
  --set watchNamespaces="team-a,team-b"
```

**Flag:**

```bash
/manager --watch-namespaces=team-a,team-b
```

**Environment variable (set automatically by OLM for OwnNamespace/SingleNamespace install modes):**

```bash
WATCH_NAMESPACE=team-a,team-b
```

### 7. Retrieve a generated API key

The generated API key is stored in a Secret (default name: `{name}-key`):

```sh
kubectl get secret eng-ci-key-key -o jsonpath='{.data.api_key}' | base64 -d
```

## Installation Methods

### Direct (Makefile)

```sh
make install       # Install CRDs
make deploy        # Deploy operator
```

### OLM (OpenShift / clusters with OLM)

```sh
operator-sdk run bundle ghcr.io/palenaai/litellm-operator-bundle:v0.7.0
```

### Helm

```sh
helm install litellm-operator deploy/charts/litellm-operator/
```

## Development

### Build

```sh
make build                    # Build operator binary
make docker-build IMG=...     # Build container image
```

### Test

```sh
make test          # Unit + integration tests (envtest)
make test-e2e      # End-to-end tests (requires cluster)
```

### Generate

```sh
make generate      # DeepCopy functions
make manifests     # CRD YAMLs, RBAC, webhooks
```

### Run locally (against current kubeconfig cluster)

```sh
make install       # Install CRDs first
make run           # Run operator outside the cluster
```

## Architecture

Key design points:

- **LiteLLMInstance** controller manages Deployment, ConfigMap, Service, Secrets, Ingress, HPA, PDB, NetworkPolicy, migration Jobs, ServiceMonitor, PrometheusRule, Grafana dashboard ConfigMaps, and CNPG ScheduledBackups
- **Secondary controllers** (Model, Team, User, VirtualKey) resolve their `instanceRef` to discover the LiteLLM API endpoint and master key, then sync state via the REST API
- **Finalizers** ensure cleanup: deleting a CRD calls the corresponding LiteLLM API delete endpoint before removing the Kubernetes resource
- **Spec hash annotations** (`litellm.palena.ai/sync-hash`) enable change detection to avoid unnecessary API calls

## Project Structure

```text
api/v1alpha1/          CRD type definitions
internal/controller/   Reconciliation controllers
internal/litellm/      LiteLLM REST API client
internal/resources/    Kubernetes resource generators
config/crd/bases/      Generated CRD manifests
config/samples/        Example custom resources
bundle/                OLM bundle manifests
deploy/charts/         Helm chart
```

## License

Copyright 2026. Licensed under the Apache License, Version 2.0.

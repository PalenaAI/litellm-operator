<p align="center">
  <img src="docs/public/logo.png" width="120" alt="LiteLLM Operator" />
</p>

<h1 align="center">LiteLLM Operator</h1>

<p align="center">
  Production-grade Kubernetes operator for <a href="https://github.com/BerriAI/litellm">LiteLLM</a> — declarative AI gateway deployments, bidirectional config sync, and first-class OpenShift support.
</p>

<p align="center">
  <a href="https://github.com/PalenaAI/litellm-operator/releases"><img src="https://img.shields.io/github/v/release/PalenaAI/litellm-operator?include_prereleases&sort=semver&label=release" alt="Release" /></a>
  <a href="https://github.com/PalenaAI/litellm-operator/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/PalenaAI/litellm-operator/ci.yml?branch=main&label=ci" alt="CI" /></a>
  <a href="https://github.com/PalenaAI/litellm-operator/blob/main/LICENSE"><img src="https://img.shields.io/github/license/PalenaAI/litellm-operator" alt="License" /></a>
  <a href="https://github.com/PalenaAI/litellm-operator"><img src="https://img.shields.io/github/go-mod/go-version/PalenaAI/litellm-operator" alt="Go version" /></a>
  <img src="https://img.shields.io/badge/kubernetes-%E2%89%A5%201.28-blue" alt="Kubernetes" />
  <img src="https://img.shields.io/badge/openshift-supported-ee0000" alt="OpenShift" />
  <img src="https://img.shields.io/badge/operator--sdk-v1.38+-5f87af" alt="Operator SDK" />
</p>

---

## Why this operator?

The community LiteLLM Helm chart deploys the proxy, but leaves you with a hard trade-off: manage models and keys through the **Admin UI** (convenient, but not GitOps-friendly) or through `proxy_server_config.yaml` (reproducible, but no UI). Pick one and you lose the other.

This operator dissolves that trade-off. Every resource — instances, organizations, models, teams, users, keys, customers, credentials, guardrails, budget tiers — is a first-class Kubernetes CRD, reconciled continuously against the LiteLLM REST API. Git is the source of truth; Admin-UI drift is detected on every sync interval and resolved per your policy (`crd-wins`, `api-wins`, or `manual`). You get GitOps **and** the Admin UI, backed by the same state.

It also handles the parts a Helm chart can't: finalizer-based cleanup that deletes upstream API objects, generated virtual keys stored as garbage-collected Kubernetes Secrets, enterprise license activation, rollback-on-failure, OpenShift-native routing, six-backend response caching, and external secret-manager integration so provider API keys never touch etcd.

## Architecture at a glance

```text
                        kubectl apply
                             │
                             ▼
 ┌─────────────────────────────────────────────────────────────┐
 │                        Kubernetes API                       │
 │                                                             │
 │   LiteLLMInstance     LiteLLMOrganization   LiteLLMModel    │
 │   LiteLLMTeam         LiteLLMUser           LiteLLMCustomer │
 │   LiteLLMVirtualKey   LiteLLMCredential     LiteLLMGuardrail│
 │   LiteLLMBudget                                             │
 └──────────────────────────────┬──────────────────────────────┘
                                │   watches / reconciles
                                ▼
                     ┌────────────────────┐
                     │  LiteLLM Operator  │
                     └─────────┬──────────┘
         ┌───────────────────┬─┴─┬───────────────────────┐
         │ Deployment        │   │  LiteLLM REST API     │
         │ ConfigMap         │   │  (bidirectional sync) │
         │ Secrets / HPA     │   │  crd-wins · api-wins  │
         │ Ingress / Route / │   │  preserve · prune     │
         │ HTTPRoute         │   │  adopt                │
         │ ServiceMonitor    │   │                       │
         │ PrometheusRule    │   │                       │
         │ Grafana dashboard │   │                       │
         └─────────┬─────────┘   └───────────┬───────────┘
                   ▼                         ▼
         ┌──────────────────────────────────────────┐
         │  LiteLLM Proxy  +  Postgres  +  Redis    │
         └──────────────────────────────────────────┘
```

## Features

| Area | What you get |
| --- | --- |
| **Infrastructure** | Declarative Deployment, ConfigMap, Service, Secrets, HPA v2, PDB, NetworkPolicy; migration Jobs per image tag; auto-rollback on `ProgressDeadlineExceeded`; topology spread constraints; `runAsNonRoot` mode using the official non-root image |
| **Networking** | Kubernetes Ingress, OpenShift Route, Gateway API HTTPRoute — pick one declaratively per instance |
| **Multi-tenancy** | Full Organization → Team → User → Key hierarchy with budgets, member management (`crd` / `sso` / `mixed` modes), and org-scoped model access |
| **API-managed CRDs** | `LiteLLMOrganization`, `LiteLLMModel`, `LiteLLMTeam`, `LiteLLMUser`, `LiteLLMCustomer`, `LiteLLMVirtualKey`, `LiteLLMBudget` — reconciled via the LiteLLM REST API with finalizer-based cleanup and spec-hash change detection |
| **Config-managed CRDs** | `LiteLLMCredential` (reusable provider API keys via `credentialRef`) and `LiteLLMGuardrail` (Aporia, Lakera, Presidio, Bedrock, LLM Guard, Guardrails AI, Azure, Google Text Moderation, custom) — materialized into `proxy_server_config.yaml`, keys injected via `secretKeyRef` (never read into operator memory) |
| **Bidirectional sync** | Periodic drift detection with `crd-wins` / `api-wins` / `manual` resolution and `preserve` / `prune` / `adopt` policies for unmanaged resources |
| **VirtualKey lifecycle** | Generated API keys stored in owner-referenced Kubernetes Secrets; rotation and revocation follow CRD deletion |
| **Authentication** | SSO for Azure Entra, Okta, Google, generic OIDC; SCIM v2 provisioning; JWT and OAuth2 auth for M2M flows; custom SSO handlers via ConfigMap or image |
| **Security** | IP allowlisting with `X-Forwarded-For` support, RBAC via `spec.rbac`, external secret managers (AWS Secrets Manager / KMS, Azure Key Vault, Google Secret Manager / KMS, HashiCorp Vault) with IRSA and workload-identity support |
| **Reliability** | 6-backend response caching (Redis / S3 / GCS / Qdrant / Redis-semantic / local), fallback chains (default, per-model, content-policy, context-window), per-error-type retry policies, tag-based routing, per-provider budget caps |
| **Observability** | ServiceMonitor + PrometheusRule with 6 built-in alerts and runbook annotations; auto-provisioned Grafana dashboard ConfigMap |
| **Data** | Optional CloudNativePG integration with `ScheduledBackup` (snapshot or `barmanObjectStore`) |
| **Admin UI** | Disable, admin-only mode, DB-backed model management, personal-key gating, custom docs URL, logo, email branding, color themes via ConfigMap |
| **Distribution** | OLM bundle for OperatorHub / OpenShift Catalog **and** a Helm chart for clusters without OLM |
| **Enterprise** | Convention-based license Secret detection (`{instance}-license` or `litellm-license`) with `EnterpriseLicenseRequired` status conditions when unlicensed enterprise features are requested |

> **Full documentation:** see the [`docs/`](docs/) folder — guides for [SSO](docs/guide/sso.md), [config sync](docs/guide/config-sync.md), [caching](docs/guide/caching.md), [RBAC](docs/guide/rbac.md), [observability](docs/guide/observability.md), [secret managers](docs/guide/secret-managers.md), and per-CRD reference pages under [`docs/reference/`](docs/reference/).

## Custom Resource Definitions

| CRD | Short Name | Description |
| --- | ---------- | ----------- |
| `LiteLLMInstance` | `li` | Deploys a LiteLLM proxy with database, Redis, networking, and SSO |
| `LiteLLMOrganization` | `lo` | Creates an organization for multi-tenant isolation with budget and model access |
| `LiteLLMModel` | `lm` | Registers a model (e.g., `openai/gpt-4o`) with the proxy |
| `LiteLLMTeam` | `lt` | Creates a team with budget limits and member management |
| `LiteLLMUser` | `lu` | Creates a user (service accounts, bot users, non-SSO environments) |
| `LiteLLMCustomer` | `lcust` | Manages an external end-user (SaaS customer) with budgets and rate limits |
| `LiteLLMCredential` | `lc` | Defines a reusable provider credential (API key + optional base URL) shared across models |
| `LiteLLMGuardrail` | `lg` | Defines a content moderation / safety integration (Aporia, Lakera, Presidio, Bedrock, etc.) |
| `LiteLLMVirtualKey` | `lk` | Generates an API key scoped to a team/user with budget and rate limits |
| `LiteLLMBudget` | `lb` | Declares a reusable budget / rate-limit tier (via `/budget/*`) referenced by `budgetId` from keys, customers, and the instance default |

All secondary resources reference a `LiteLLMInstance` via `spec.instanceRef`. Teams can optionally reference a `LiteLLMOrganization` via `spec.organizationRef`.

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

### Reusable Credentials

When many models share the same provider API key (e.g., several OpenAI deployments), define the credential once and reference it from each `LiteLLMModel` via `credentialRef`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openai-credentials
type: Opaque
stringData:
  api-key: sk-...
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMCredential
metadata:
  name: openai-prod
spec:
  instanceRef:
    name: my-gateway
  # The name used under `credential_list` in the generated proxy config.
  # Models reference this via `litellm_params.litellm_credential_name`.
  credentialName: openai-prod
  apiKeySecretRef:
    name: openai-credentials
    key: api-key
  # Optional extras merged into credential_info (api_base / api_version /
  # free-form params are supported — params cannot override reserved keys).
  apiBase: https://api.openai.com/v1
---
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
    credentialRef:
      name: openai-prod   # takes precedence over inline apiKeySecretRef/apiBase
```

The operator injects the API key into the proxy pod via a `secretKeyRef`-backed env var (`CREDENTIAL_OPENAI_PROD_API_KEY`) and writes a matching `os.environ/…` reference to the generated `proxy_server_config.yaml` — the key value itself is never read into the operator's memory. Rotating the key is a Secret update: the operator rolls out a new Deployment pod to pick up the new value.

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

### Multi-Tenant Organizations

Create an organization to group teams under a shared budget and model access policy:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMOrganization
metadata:
  name: acme-corp
spec:
  instanceRef:
    name: my-gateway
  organizationAlias: acme-corp
  models: [gpt-4o, claude-4-sonnet]
  maxBudget: 5000
  budgetDuration: "30d"
  members:
    - email: admin@acme.com
      role: org_admin
    - email: user@acme.com
      role: internal_user
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: acme-engineering
spec:
  instanceRef:
    name: my-gateway
  organizationRef:
    name: acme-corp
  teamAlias: acme-engineering
  models: [gpt-4o]
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

### RBAC (Role-Based Access Control)

Enforce route restrictions and key generation controls:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  rbac:
    enabled: true
    adminOnlyRoutes:
      - /model/new
      - /model/delete
    allowedRoutes:
      - /chat/completions
      - /embeddings
      - /key/info
    defaultTeamDisabled: true   # force team-based keys
    keyGeneration:              # enterprise
      teamKeyGeneration:
        allowedTeamMemberRoles: ["admin"]
    rolePermissions:            # enterprise
      internal_user:
        routes: ["/key/generate", "/key/info"]
        models: ["gpt-4", "claude-3-haiku"]
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
operator-sdk run bundle ghcr.io/palenaai/litellm-operator-bundle:latest
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
- **Secondary controllers** (Organization, Model, Team, User, VirtualKey) resolve their `instanceRef` to discover the LiteLLM API endpoint and master key, then sync state via the REST API
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

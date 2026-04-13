# Architecture

## Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     LiteLLM Operator                            │
│                                                                 │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐  │
│  │    Instance       │  │  Organization    │  │    Team      │  │
│  │   Controller      │  │   Controller     │  │  Controller  │  │
│  │                   │  │                  │  │              │  │
│  │ - Deployment      │  │ - POST /org/*    │  │ - POST /team │  │
│  │ - ConfigMap       │  │ - Members        │  │ - Members    │  │
│  │ - Service         │  │ - Budget mgmt   │  │ - Budgets    │  │
│  │ - Ingress/Route   │  │                  │  │              │  │
│  │ - HPA, PDB        │  └────────┬─────────┘  └──────┬───────┘  │
│  │ - Migration Job   │           │                    │          │
│  │ - SSO/SCIM config │  ┌────────┴────────┐  ┌───────┴───────┐  │
│  │ - License Secret  │  │     Model       │  │     User      │  │
│  │                   │  │   Controller    │  │   Controller  │  │
│  └────────┬──────────┘  │                 │  │               │  │
│           │             │ - POST /model/* │  │ - POST /user/*│  │
│           │             │ - Health check  │  │ - Budget mgmt │  │
│           │             └────────┬────────┘  └───────┬───────┘  │
│           │                      │                    │          │
│           │             ┌────────┴────────────────────┘          │
│           │             │                                        │
│           │    ┌────────┴────────┐                               │
│           │    │  VirtualKey     │  ┌──────────────────┐         │
│           │    │  Controller     │  │  ConfigSync      │         │
│           │    │ - POST /key/*   │  │  Controller      │         │
│           │    │ - Secret mgmt   │  │ - Drift detect   │         │
│           │    └────────┬────────┘  │ - Prune/adopt    │         │
│           │             │           │ - Status counts  │         │
│           │             │           └────────┬─────────┘         │
│  ┌────────▼─────────────▼────────────────────▼───────────────┐  │
│  │                    LiteLLM API Client                      │  │
│  │  OrganizationService · ModelService · TeamService           │  │
│  │  UserService · KeyService · CustomerService · HealthService│  │
│  └────────────────────────┬───────────────────────────────────┘  │
│                           │                                      │
└───────────────────────────┼──────────────────────────────────────┘
                            │ HTTP (within cluster)
                            ▼
                 ┌─────────────────────┐
                 │   LiteLLM Proxy     │
                 │   (Deployment)      │
                 │  REST API :4000     │
                 └─────────┬───────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │PostgreSQL│ │  Redis   │ │ LLM      │
        │          │ │ (cache)  │ │Providers │
        └──────────┘ └──────────┘ └──────────┘
```

## Controllers

### Instance Controller

The most complex controller. It manages all Kubernetes infrastructure for a LiteLLM deployment:

1. **ConfigMap** — generates `proxy_server_config.yaml` from general settings (including IP allowlisting), router settings (including tag-based routing), fallback chains, retry policies, caching, SSO, and callback configuration
2. **Secrets** — master key (auto-generated or from existing Secret), salt key, SSO client credentials
3. **Migration Job** — runs database migrations before Deployment rollout
4. **Deployment** — LiteLLM container with env vars, volumes, probes, security context
5. **Service** — ClusterIP or LoadBalancer
6. **Ingress / Route** — optional external access (Ingress for vanilla K8s, Route for OpenShift)
7. **HPA** — horizontal pod autoscaling based on CPU/memory
8. **PDB** — pod disruption budget for availability
9. **NetworkPolicy** — restrict access to the LiteLLM service
10. **OpenShift Route** — optional Route for OpenShift clusters (TLS edge/passthrough/reencrypt)
11. **Gateway API HTTPRoute** — optional HTTPRoute for Gateway API implementations (Istio, Envoy Gateway, Cilium, etc.)
12. **SCIM Token** — auto-generate and store SCIM bearer token
13. **License Secret detection** — discovers `{instance}-license` or `litellm-license` Secrets and injects `LITELLM_LICENSE` env var via `secretKeyRef`

### Secondary Controllers

Most secondary controllers (Organization, Model, Team, User, Customer, VirtualKey) follow the same pattern:

```
CR created/updated/deleted
│
├── 1. Fetch CR
├── 2. Check deletion → finalizer cleanup → call LiteLLM DELETE API
├── 3. Ensure finalizer present
├── 4. Resolve instanceRef → get API endpoint + master key
├── 5. Reconcile against LiteLLM API (create or update)
└── 6. Update status (synced, IDs, conditions)
```

**Change detection** uses a spec hash stored in the `litellm.palena.ai/sync-hash` annotation. On each reconciliation, the current spec hash is compared to the stored hash — if different, an update is sent to the LiteLLM API.

### Credential Controller

The Credential controller is different: `LiteLLMCredential` is a **config-level** resource, not an API-level one. There is no `POST /credential/new` equivalent — credentials live in the proxy's `credential_list` config section. The controller validates the referenced Secret and counts consuming models, but the actual materialization happens in the Instance controller, which watches LiteLLMCredential and rebuilds the ConfigMap + rolls the Deployment whenever a credential changes. API keys are injected as `CREDENTIAL_{NAME}_API_KEY` env vars via `secretKeyRef`, and the ConfigMap uses `os.environ/...` placeholders so plaintext keys never land on disk.

### Guardrail Controller

Like the Credential controller, the Guardrail controller manages a **config-level** resource. `LiteLLMGuardrail` CRs are rendered into the `guardrails` section of `proxy_server_config.yaml` by the Instance controller — there is no API-level create/update. The Guardrail controller validates that the referenced instance and (if declared) API key Secret exist, and reports `Ready` / `SecretNotFound` / `InstanceNotFound` conditions. The Instance controller watches LiteLLMGuardrail CRs and rebuilds the ConfigMap + rolls the Deployment whenever a guardrail is created, updated, or deleted. Provider API keys are injected as `GUARDRAIL_{NAME}_API_KEY` env vars via `secretKeyRef`, referenced from the config as `os.environ/...`. Per-key/per-team guardrail assignment via `spec.guardrails []string` on `LiteLLMVirtualKey` / `LiteLLMTeam` is a LiteLLM Enterprise feature.

### ConfigSync Controller

The ConfigSync controller is a **separate controller** (not part of the Instance controller) that implements [bidirectional config sync](/guide/config-sync). It watches `LiteLLMInstance` resources and, for each instance with `spec.configSync.enabled: true`, runs a periodic sync cycle on the configured interval (default 30s).

The sync cycle covers all six API-managed resource types: models, teams, users, keys, organizations, and customers. For each type it:

1. Lists CRDs referencing this instance and builds a map of API IDs
2. Lists all resources from the LiteLLM API
3. Classifies each API resource as **managed** (has a matching CRD) or **unmanaged**
4. Detects **drift** on managed resources by comparing key spec fields against the API response
5. Handles drift per the `conflictResolution` policy and unmanaged resources per the `unmanagedResourcePolicy`
6. Updates `status.configSync` counts and the `ConfigSynced` condition

The ConfigSync controller is complementary to the per-resource controllers: per-resource controllers handle real-time CRD lifecycle (event-driven, immediate), while ConfigSync provides periodic comprehensive drift detection, unmanaged resource handling, and status aggregation.

## Reconciliation Model

The operator uses the standard Kubernetes reconciliation pattern:

- **Finalizers** ensure cleanup: deleting a CRD calls the corresponding LiteLLM API delete endpoint before removing the Kubernetes resource
- **Status conditions** report health using standard `metav1.Condition` types
- **Requeue strategies**:
  - Transient errors (network, API 5xx): `RequeueAfter: 30s`
  - Permanent errors (invalid spec, 400): set status condition, don't requeue
  - Enterprise license errors (403 + "enterprise"): set `EnterpriseLicenseRequired` condition, don't requeue
  - Healthy state: `RequeueAfter: 5m` for periodic re-sync

## Security Model

- Operator runs with a dedicated ServiceAccount and scoped RBAC
- LiteLLM pods run as **non-root** with a **read-only root filesystem**
- Secrets (master key, salt key, provider API keys, license keys) are always read from Kubernetes Secrets via `secretKeyRef` — never stored as plaintext in CRDs or read into operator memory
- NetworkPolicy restricts which namespaces can reach the LiteLLM service (network layer)
- IP allowlisting restricts API access to specific IP addresses or CIDR ranges (application layer, enterprise)
- Generated virtual keys are stored in Secrets with `ownerReferences` for automatic garbage collection

## Upgrade Strategy

When `spec.image.tag` changes on a `LiteLLMInstance`:

1. A migration Job runs with the new image
2. On success, the Deployment is updated with a rolling update
3. Health checks validate new pods
4. If auto-rollback is enabled and health checks fail, the Deployment reverts

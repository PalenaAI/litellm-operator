# Architecture

## Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     LiteLLM Operator                            │
│                                                                 │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐  │
│  │    Instance       │  │     Model        │  │    Team      │  │
│  │   Controller      │  │   Controller     │  │  Controller  │  │
│  │                   │  │                  │  │              │  │
│  │ - Deployment      │  │ - POST /model/*  │  │ - POST /team │  │
│  │ - ConfigMap       │  │ - Health check   │  │ - Members    │  │
│  │ - Service         │  │ - Status sync    │  │ - Budgets    │  │
│  │ - Ingress/Route   │  │                  │  │              │  │
│  │ - HPA, PDB        │  └────────┬─────────┘  └──────┬───────┘  │
│  │ - Migration Job   │           │                    │          │
│  │ - SSO/SCIM config │  ┌────────┴────────┐  ┌───────┴───────┐  │
│  │ - License Secret  │  │     User        │  │  VirtualKey   │  │
│  │ - Config Sync     │  │   Controller    │  │  Controller   │  │
│  └────────┬──────────┘  │                 │  │               │  │
│           │             │                 │  │               │  │
│           │             │ - POST /user/*  │  │ - POST /key/* │  │
│           │             │ - Budget mgmt   │  │ - Secret mgmt │  │
│           │             └────────┬────────┘  └───────┬───────┘  │
│           │                      │                    │          │
│  ┌────────▼──────────────────────▼────────────────────▼───────┐  │
│  │                    LiteLLM API Client                      │  │
│  │  ModelService · TeamService · UserService                  │  │
│  │  KeyService · HealthService                                │  │
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

All secondary controllers (Model, Team, User, VirtualKey) follow the same pattern:

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

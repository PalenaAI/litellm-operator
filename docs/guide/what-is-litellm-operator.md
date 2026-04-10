# What is LiteLLM Operator?

LiteLLM Operator is a Kubernetes operator that deploys and manages production-ready [LiteLLM](https://github.com/BerriAI/litellm) AI Gateway instances. It replaces manual Helm-based deployments with a declarative, reconciliation-based approach.

## The Problem

LiteLLM stores its configuration (models, teams, users, keys) in a PostgreSQL database, accessed via a REST API. The community Helm chart manages only the static `proxy_server_config.yaml` — so any changes made through the **Admin UI** are invisible to GitOps, and a Helm upgrade can overwrite them.

This creates a fundamental conflict: teams want to use the Admin UI for quick changes, but also want GitOps as the source of truth.

## The Solution

The LiteLLM Operator manages configuration through the **LiteLLM REST API** rather than the static config file. This means:

1. **CRD changes** get pushed to LiteLLM via the API
2. **Admin UI changes** go through the same API and land in the same database
3. **The config sync loop** reads back from the API and reconciles

Both the CRDs and the Admin UI write to the same backing store, so they coexist naturally.

## CRD Hierarchy

The operator introduces six Custom Resource Definitions:

```text
LiteLLMInstance (primary — deploys infrastructure)
├── LiteLLMOrganization (multi-tenant isolation with budgets)
│   └── LiteLLMTeam (organizationRef — scoped to an org)
├── LiteLLMModel (registers AI models)
├── LiteLLMTeam (creates teams with budgets and members)
│   └── members (inline, managed by memberManagement policy)
├── LiteLLMUser (manages users for non-SSO environments)
└── LiteLLMVirtualKey (generates scoped API keys)
```

All secondary CRDs reference a `LiteLLMInstance` via `spec.instanceRef`. Teams can optionally reference a `LiteLLMOrganization` via `spec.organizationRef`. The operator resolves these to find the LiteLLM API endpoint, master key, and organization ID.

## Key Features

### Bidirectional Config Sync

The operator's distinguishing feature. A reconciliation loop keeps CRD state and LiteLLM's runtime state in sync, with configurable policies for handling resources created outside of CRDs. [Learn more](/guide/config-sync).

### SSO-Aware Team Management

Three member management modes prevent the operator from destroying SSO-provisioned memberships when `spec.members` is empty. [Learn more](/guide/team-members).

### Virtual Key Secret Management

When a `LiteLLMVirtualKey` is created, the operator generates an API key via the LiteLLM API and stores it in a Kubernetes Secret with owner references for automatic garbage collection. [Learn more](/guide/virtual-keys).

### Enterprise License Activation

Convention-based license management. Drop a Secret named `{instance}-license` or `litellm-license` into the namespace and the operator injects the `LITELLM_LICENSE` env var automatically — no CRD fields required. [Learn more](/guide/enterprise-license).

### Production Infrastructure

The Instance controller manages the full production stack:
- Deployment with security contexts and health probes
- ConfigMap with `proxy_server_config.yaml`
- Service, Ingress, and OpenShift Route
- HPA and PDB for availability
- NetworkPolicy and IP allowlisting for security
- Database migration Jobs
- SSO and SCIM configuration

## Distribution

| Method | Use Case |
| --- | --- |
| **OLM Bundle** | OpenShift, clusters with OLM installed |
| **Helm Chart** | Vanilla Kubernetes, k3s, RKE2 |
| **Makefile** | Development, CI/CD |

Both OLM and Helm install the exact same operator binary and CRDs — only the lifecycle management differs.

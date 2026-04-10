# CRD Reference

The LiteLLM Operator defines six Custom Resource Definitions in the `litellm.palena.ai/v1alpha1` API group.

## Overview

| CRD | Short Name | Scope | Description |
| --- | --- | --- | --- |
| [LiteLLMInstance](/reference/litellminstance) | `li` | Namespaced | Primary CRD. Deploys LiteLLM proxy infrastructure |
| [LiteLLMOrganization](/reference/litellmorganization) | `lo` | Namespaced | Creates an organization for multi-tenant isolation |
| [LiteLLMModel](/reference/litellmmodel) | `lm` | Namespaced | Registers an AI model with the proxy |
| [LiteLLMTeam](/reference/litellmteam) | `lt` | Namespaced | Creates a team with budget and member management |
| [LiteLLMUser](/reference/litellmuser) | `lu` | Namespaced | Creates a user (non-SSO environments) |
| [LiteLLMVirtualKey](/reference/litellmvirtualkey) | `lk` | Namespaced | Generates a scoped API key |

## Relationship Diagram

```text
LiteLLMInstance
├── LiteLLMOrganization (instanceRef → LiteLLMInstance)
│   └── LiteLLMTeam     (organizationRef → LiteLLMOrganization)
├── LiteLLMModel        (instanceRef → LiteLLMInstance)
├── LiteLLMTeam         (instanceRef → LiteLLMInstance)
├── LiteLLMUser         (instanceRef → LiteLLMInstance, teamRef → LiteLLMTeam)
└── LiteLLMVirtualKey   (instanceRef → LiteLLMInstance, teamRef → LiteLLMTeam, userRef → LiteLLMUser)
```

All secondary CRDs reference a `LiteLLMInstance` in the same namespace via `spec.instanceRef`. Teams can optionally reference a `LiteLLMOrganization` via `spec.organizationRef`. The operator resolves these references to find the LiteLLM API endpoint, master key, and organization ID.

## Common Types

These types are shared across multiple CRDs:

### SecretKeyRef

References a specific key within a Kubernetes Secret.

```yaml
secretRef:
  name: my-secret    # Secret name
  key: my-key        # Key within the Secret
```

### InstanceRef

References a `LiteLLMInstance` in the same namespace.

```yaml
instanceRef:
  name: my-gateway   # LiteLLMInstance name
```

## Quick Reference

```bash
# List all resources
kubectl get li,lo,lm,lt,lu,lk

# Watch a specific type
kubectl get litellmmodels -w

# Describe a resource
kubectl describe litellminstance my-gateway
```

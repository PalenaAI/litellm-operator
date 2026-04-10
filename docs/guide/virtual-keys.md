# Virtual Key Secret Management

The `LiteLLMVirtualKey` CRD generates scoped API keys and automatically stores them in Kubernetes Secrets.

## How It Works

1. You create a `LiteLLMVirtualKey` CR with a key alias, budget, and scope
2. The operator calls `POST /key/generate` on the LiteLLM API
3. The returned API key is stored in a Kubernetes Secret
4. The Secret has an `ownerReference` pointing to the VirtualKey CR
5. When the VirtualKey CR is deleted, the Secret is **automatically garbage collected**

## Creating a Virtual Key

```yaml
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
  models:
    - gpt-4o
    - claude-4-sonnet
  maxBudget: "100"
  budgetDuration: "30d"
  rpmLimit: 60
```

## Retrieving the Key

The API key is stored in a Secret named `{name}-key` by default (customizable via `spec.keySecretName`):

```bash
# Get the Secret name from status
kubectl get lk eng-ci-key -o jsonpath='{.status.keySecretRef.name}'

# Retrieve the API key
kubectl get secret eng-ci-key-key -o jsonpath='{.data.api-key}' | base64 -d
```

## Using the Key in Other Pods

Reference the Secret in your application's Deployment:

```yaml
env:
  - name: LITELLM_API_KEY
    valueFrom:
      secretKeyRef:
        name: eng-ci-key-key
        key: api-key
```

## Custom Secret Name

Override the default Secret name with `spec.keySecretName`:

```yaml
spec:
  keySecretName: my-custom-secret-name
```

## Scoping Keys

Virtual keys can be scoped to a **team**, a **user**, or both:

```yaml
spec:
  # Scope to a team managed by a LiteLLMTeam CR
  teamRef:
    name: engineering

  # Scope to a user managed by a LiteLLMUser CR
  userRef:
    name: service-bot
```

The operator resolves these references to LiteLLM IDs before generating the key.

## Per-Model Budget Limits (Enterprise)

Set per-model spending limits on a key with `modelMaxBudget`:

```yaml
spec:
  instanceRef:
    name: my-gateway
  keyAlias: capped-key
  models:
    - gpt-4
    - claude-3-opus
  modelMaxBudget:
    gpt-4: "100.00"
    claude-3-opus: "50.00"
  maxParallelRequests: 5
```

This limits spending per model independently — once the GPT-4 budget is exhausted, GPT-4 requests are rejected but Claude requests continue up to their own limit. `maxParallelRequests` caps concurrency for the key.

::: tip
`modelMaxBudget` requires a LiteLLM Enterprise license. The operator will set `Reason: EnterpriseLicenseRequired` on the condition if the license is missing.
:::

## Key Updates vs Regeneration

- **Updating** key properties (budget, rate limits, models) calls `POST /key/update` — the key value itself doesn't change
- The API key is only generated **once** when the VirtualKey CR is first created
- To rotate a key, delete and recreate the VirtualKey CR

## Status

```yaml
status:
  synced: true
  keySecretRef:
    name: eng-ci-key-key
    key: api-key
  litellmKeyToken: "sk-..."  # hashed token for reference
  isActive: true
  currentSpend: "42.50"
  lastSyncTime: "2026-04-01T12:00:00Z"
```

## Garbage Collection

The Secret has an `ownerReference` pointing to the VirtualKey CR:

```yaml
metadata:
  ownerReferences:
    - apiVersion: litellm.palena.ai/v1alpha1
      kind: LiteLLMVirtualKey
      name: eng-ci-key
      controller: true
      blockOwnerDeletion: true
```

When you delete the VirtualKey CR:
1. The finalizer calls `POST /key/delete` to revoke the key in LiteLLM
2. The finalizer is removed, allowing CR deletion
3. Kubernetes garbage collects the owned Secret

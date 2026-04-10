# LiteLLMVirtualKey

Generates a scoped API key and stores it in a Kubernetes Secret.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMVirtualKey`
**Short Name:** `lk`

## Example

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
  maxBudget: "100"
  budgetDuration: "30d"
  rpmLimit: 60
  keySecretName: engineering-ci-api-key
```

## Spec Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | — | Reference to the LiteLLMInstance |
| `keyAlias` | string | Yes | — | Human-readable key alias |
| `teamRef` | *InstanceRef | No | — | Reference to a `LiteLLMTeam` CR |
| `userRef` | *InstanceRef | No | — | Reference to a `LiteLLMUser` CR |
| `models` | []string | No | — | Models this key can access |
| `maxBudget` | *string | No | — | Maximum budget in USD |
| `budgetDuration` | string | No | — | Budget reset period (e.g., `30d`) |
| `expiresAt` | *Time | No | — | Key expiration time |
| `tpmLimit` | *int | No | — | Tokens per minute limit |
| `rpmLimit` | *int | No | — | Requests per minute limit |
| `metadata` | map[string]string | No | — | Custom metadata |
| `modelMaxBudget` | map[string]string | No | — | Per-model spending limits in USD (enterprise) |
| `maxParallelRequests` | *int | No | — | Maximum concurrent requests for this key |
| `keySecretName` | string | No | `{name}-key` | Name for the generated Secret |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the key is synced to LiteLLM |
| `keySecretRef` | *SecretKeyRef | Reference to the Secret containing the API key |
| `litellmKeyToken` | string | Hashed token for reference |
| `isActive` | bool | Whether the key is active |
| `currentSpend` | *string | Current spend in USD |
| `expiresAt` | *Time | Key expiration time |
| `lastSyncTime` | *Time | Last successful sync time |
| `conditions` | []Condition | Standard conditions |

## Print Columns

```bash
kubectl get lk
NAME          ALIAS         ACTIVE   SYNCED   AGE
eng-ci-key    eng-ci-key    true     true     1d
```

## Retrieving the Generated Key

```bash
kubectl get secret eng-ci-key-key -o jsonpath='{.data.api-key}' | base64 -d
```

## Secret Format

The generated Secret contains:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: eng-ci-key-key
  ownerReferences:
    - apiVersion: litellm.palena.ai/v1alpha1
      kind: LiteLLMVirtualKey
      name: eng-ci-key
type: Opaque
data:
  api-key: <base64-encoded-api-key>
```

See [Virtual Key Secrets](/guide/virtual-keys) for more details on lifecycle and garbage collection.

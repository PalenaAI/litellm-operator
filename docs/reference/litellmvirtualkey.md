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
| `blocked` | *bool | No | — | Disable this key without deleting it (rejects all requests using it) |
| `softBudget` | *float64 | No | — | Alert threshold in USD below `maxBudget` (does not block) |
| `modelRpmLimit` | map[string]int | No | — | Per-model requests-per-minute caps (model name → RPM) |
| `modelTpmLimit` | map[string]int | No | — | Per-model tokens-per-minute caps (model name → TPM) |
| `objectPermission` | *ObjectPermission | No | — | Grant access to MCP servers, vector stores, agents, access groups |
| `modelMaxBudget` | map[string]string | No | — | Per-model spending limits in USD (enterprise) |
| `maxParallelRequests` | *int | No | — | Maximum concurrent requests for this key |
| `guardrails` | []string | No | — | Names of [LiteLLMGuardrail](/reference/litellmguardrail) CRs this key opts into. Each entry must match `spec.guardrailName` on a guardrail bound to the same instance (enterprise) |
| `keySecretName` | string | No | `{name}-key` | Name for the generated Secret. Only honoured before the key is minted — once `status.keySecretRef` is set the name is pinned, so editing it cannot orphan the only copy of the key material |
| `keySecretTemplate` | *KeySecretTemplateSpec | No | — | Annotations and labels to apply to the generated Secret, so third-party controllers can act on it |

### KeySecretTemplateSpec

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `annotations` | map[string]string | No | — | Annotations to apply to the key Secret |
| `labels` | map[string]string | No | — | Labels to apply to the key Secret. The operator’s own labels win on conflict |

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

## Annotating the Generated Secret

`spec.keySecretTemplate` puts annotations and labels on the generated Secret, so
that other controllers can act on it without an external mutating admission
policy. A common use is [kubernetes-reflector](https://github.com/emberstack/kubernetes-reflector),
which mirrors the Secret into the namespace where the consuming application runs:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: eng-ci-key
spec:
  instanceRef:
    name: my-gateway
  keyAlias: eng-ci-key
  keySecretTemplate:
    annotations:
      reflector.v1.k8s.emberstack.com/reflection-allowed: "true"
      reflector.v1.k8s.emberstack.com/reflection-allowed-namespaces: "apps"
      reflector.v1.k8s.emberstack.com/reflection-auto-enabled: "true"
      reflector.v1.k8s.emberstack.com/reflection-auto-namespaces: "apps"
    labels:
      app.kubernetes.io/part-of: checkout
```

Entries are merged, not replaced: annotations and labels added to the Secret by
other controllers survive reconciliation. The flip side is that removing an entry
from `keySecretTemplate` does not remove it from the Secret — delete it from the
Secret directly.

Mirroring a Secret copies a live credential across a namespace boundary. Scope
`reflection-allowed-namespaces` to the namespaces that genuinely need the key.

## If the Generated Secret Is Deleted

LiteLLM stores only a hash of the key, so the material in the Secret is the only
copy — it cannot be re-read from the API. If the Secret is deleted, the operator
deletes the now-unusable key from LiteLLM, mints a replacement, and writes a fresh
Secret, emitting a `KeySecretMissing` warning event. Consumers must re-read the
Secret to pick up the new key.

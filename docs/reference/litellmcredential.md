# LiteLLMCredential

Declares a reusable provider credential that is materialized into the proxy's `credential_list` config section. Multiple [LiteLLMModel](./litellmmodel) CRs can reference the same credential by name instead of each embedding its own API key, reducing Secret sprawl and simplifying key rotation.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMCredential`
**Short Name:** `lc`

## Why credentials?

Without a credential, every model that talks to a provider has to inline its own `apiKeySecretRef` and `apiBase`:

```yaml
# 20 models, 20 duplicated apiKeySecretRef blocks
spec:
  litellmParams:
    model: openai/gpt-4o
    apiBase: https://api.openai.com/v1
    apiKeySecretRef:
      name: openai-credentials
      key: OPENAI_API_KEY
```

`LiteLLMCredential` lets you declare provider creds *once* per instance:

```yaml
# Declared once
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMCredential
metadata:
  name: openai-prod
spec:
  instanceRef:
    name: my-gateway
  credentialName: openai-prod
  apiBase: https://api.openai.com/v1
  apiKeySecretRef:
    name: openai-credentials
    key: OPENAI_API_KEY
---
# Referenced by any number of models
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
      name: openai-prod
```

Rotating the key means updating one Secret — no model edits required.

## Example

### OpenAI

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMCredential
metadata:
  name: openai-prod
spec:
  instanceRef:
    name: my-gateway
  credentialName: openai-prod
  apiBase: https://api.openai.com/v1
  apiKeySecretRef:
    name: openai-credentials
    key: OPENAI_API_KEY
```

### Azure OpenAI

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMCredential
metadata:
  name: azure-east
spec:
  instanceRef:
    name: my-gateway
  credentialName: azure-east
  apiBase: https://my-deployment.openai.azure.com/
  apiVersion: "2024-02-01"
  apiKeySecretRef:
    name: azure-credentials
    key: AZURE_API_KEY
  params:
    deployment_id: gpt-4o-deployment
```

## Spec Fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | Reference to the LiteLLMInstance this credential belongs to |
| `credentialName` | string | Yes | Unique credential name; referenced by models via `litellmParams.credentialRef.name` |
| `apiKeySecretRef` | SecretKeyRef | Yes | Reference to a Kubernetes Secret containing the provider API key |
| `apiBase` | string | No | API base URL for the provider (e.g., `https://api.openai.com/v1`) |
| `apiVersion` | string | No | API version (e.g., `"2024-02-01"` for Azure OpenAI) |
| `params` | map[string]JSON | No | Additional provider-specific parameters merged into `credential_values`. Values are **arbitrary JSON** (strings, numbers, bools, or nested objects/arrays) — e.g. a Vertex AI service-account JSON object. Reserved keys (`api_key`, `api_base`, `api_version`) are ignored |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `configured` | bool | Whether the credential is configured in the referenced instance's proxy config |
| `referencedByModels` | int | Number of LiteLLMModel CRs in the same namespace referencing this credential via `credentialRef` |
| `lastSyncTime` | *Time | Last successful validation/reconciliation time |
| `conditions` | []Condition | Standard conditions. `Ready=True` with reason `Validated` means the referenced Secret exists and has the expected key |

### Ready condition reasons

| Reason | Meaning |
| --- | --- |
| `Validated` | Credential is valid and materialized into the instance config |
| `InstanceNotFound` | `spec.instanceRef.name` does not resolve to a LiteLLMInstance in the same namespace |
| `SecretNotFound` | `spec.apiKeySecretRef.name` does not exist |
| `SecretKeyMissing` | The referenced Secret exists but does not contain `spec.apiKeySecretRef.key` |

## Print Columns

```bash
kubectl get lc
NAME          CREDENTIAL    INSTANCE     CONFIGURED   MODELS   AGE
openai-prod   openai-prod   my-gateway   true         3        1d
azure-east    azure-east    my-gateway   true         2        1d
```

## How it works

Credentials are **config-level** resources: the operator materializes them into the `credential_list` section of `proxy_server_config.yaml` rather than calling the LiteLLM REST API. For each credential, the operator writes:

```yaml
credential_list:
  - credential_name: openai-prod
    credential_values:
      api_base: https://api.openai.com/v1
      api_key: os.environ/CREDENTIAL_OPENAI_PROD_API_KEY
    credential_info: {}
```

The API key is never written to the ConfigMap. Instead, the operator injects a `CREDENTIAL_{SANITIZED_NAME}_API_KEY` environment variable on the LiteLLM Deployment backed by the Secret reference, and LiteLLM resolves the `os.environ/...` placeholder at startup.

### Env var naming

The env var name follows `CREDENTIAL_<SANITIZED>_API_KEY` where `<SANITIZED>` is the credential name uppercased with any non-alphanumeric characters replaced by underscores. For example:

| credentialName | env var |
| --- | --- |
| `openai-prod` | `CREDENTIAL_OPENAI_PROD_API_KEY` |
| `azure.east` | `CREDENTIAL_AZURE_EAST_API_KEY` |
| `anthropic` | `CREDENTIAL_ANTHROPIC_API_KEY` |

### Watches

Both the credential controller and the LiteLLMInstance controller watch LiteLLMCredential objects:

- The **credential controller** validates the referenced Secret and counts how many models reference the credential.
- The **LiteLLMInstance controller** re-reconciles the target instance whenever a credential is created, updated, or deleted — rebuilding the ConfigMap and rolling the Deployment with the new env vars.

This means there is nothing to "apply" manually: editing a LiteLLMCredential triggers an automatic rollout on the owning instance.

## Reconciliation

The credential controller follows this pattern:

1. Fetch the `LiteLLMCredential` CR.
2. Resolve `instanceRef` to ensure the target LiteLLMInstance exists.
3. Fetch the API key Secret and verify the expected key is present.
4. Count LiteLLMModel CRs in the same namespace that reference this credential.
5. Set `status.configured = true` and the `Ready` condition.
6. Requeue every 5 minutes to refresh the referenced-by count and re-validate the Secret.

When the CR is deleted, the operator's finalizer removes the credential from the ConfigMap on the next instance reconciliation (triggered by the Watch).

## Security considerations

- API keys are only read from Secrets — never stored in the CR spec or written to ConfigMaps.
- The operator's ServiceAccount needs `get` / `list` / `watch` permissions on Secrets in the namespaces it manages.
- Credentials are namespace-scoped: a LiteLLMModel can only reference a LiteLLMCredential in the same namespace. Use separate instances per namespace for strong isolation.
- Rotating a key is a Secret update — the next reconciliation picks it up and rolls the Deployment via the env-var checksum on the Deployment pod template.

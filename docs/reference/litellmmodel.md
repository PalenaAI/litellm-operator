# LiteLLMModel

Registers an AI model with a LiteLLM proxy instance. The operator syncs the model to the LiteLLM API via `POST /model/new` and `POST /model/update`.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMModel`
**Short Name:** `lm`

## Example

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
    rpm: 500
    tpm: 100000
    timeout: 60
  modelInfo:
    maxTokens: 128000
    inputCostPerToken: 0.0000025
    outputCostPerToken: 0.00001
  tags: ["paid"]
```

## Spec Fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | Reference to the LiteLLMInstance |
| `modelName` | string | Yes | Model name exposed to clients |
| `litellmParams` | LiteLLMModelParams | Yes | Provider-specific parameters |
| `modelInfo` | *ModelInfo | No | Optional model metadata |
| `tags` | []string | No | Tags for tag-based routing (requires `enableTagFiltering` on the instance) |

### `litellmParams`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `model` | string | Yes | Provider/model string (e.g., `openai/gpt-4o`, `anthropic/claude-sonnet-4-20250514`) |
| `credentialRef` | *CredentialRef | No | Reference to a [LiteLLMCredential](./litellmcredential) in the same namespace. Takes precedence over inline `apiBase` / `apiVersion` / `apiKeySecretRef` |
| `apiBase` | string | No | API base URL for the provider. Ignored if `credentialRef` is set |
| `apiVersion` | string | No | Provider API version (e.g., `2024-10-21` for Azure OpenAI / Azure AI Foundry). Ignored if `credentialRef` is set |
| `apiKeySecretRef` | *SecretKeyRef | No | Secret containing the provider API key. Ignored if `credentialRef` is set |
| `rpm` | *int | No | Requests per minute limit |
| `tpm` | *int | No | Tokens per minute limit |
| `timeout` | *int | No | Request timeout in seconds |
| `streamTimeout` | *int | No | Stream timeout in seconds |
| `maxRetries` | *int | No | Max retries for failed requests |

#### Using a shared credential

Instead of repeating `apiKeySecretRef` and `apiBase` in every model, declare a [LiteLLMCredential](./litellmcredential) once and reference it by name:

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
    credentialRef:
      name: openai-prod   # → LiteLLMCredential in the same namespace
```

The operator resolves the credential's `api_base` / `api_version` / `api_key` and writes them **inline** on the model registered with LiteLLM (and also sends `litellm_credential_name: openai-prod` for Admin UI association). Resolving inline is deliberate: LiteLLM's request-time resolution of a named credential into a DB-stored model is unreliable on a cold start — a model registered with only `litellm_credential_name` can come up with no `api_base`, which breaks Azure deployments. Inline values are restart-safe and always win. See [LiteLLMCredential](./litellmcredential) for how credentials are declared.

::: tip Azure
For Azure OpenAI / Azure AI Foundry, set `apiVersion` (e.g. `2024-10-21`) on the credential — or inline on the model when not using `credentialRef`. Without it, the deployment falls back to LiteLLM's default API version, which many Azure models reject.
:::

### `modelInfo`

| Field | Type | Description |
| --- | --- | --- |
| `maxTokens` | *int | Maximum tokens supported |
| `inputCostPerToken` | *float64 | Input cost per token in USD |
| `outputCostPerToken` | *float64 | Output cost per token in USD |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the model is synced to LiteLLM |
| `litellmModelId` | string | LiteLLM-assigned model ID |
| `lastSyncTime` | *Time | Last successful sync time |
| `health` | string | Model health status from LiteLLM |
| `latencyP50Ms` | *int | P50 latency in milliseconds |
| `latencyP95Ms` | *int | P95 latency in milliseconds |
| `requestsLast24h` | *int64 | Request count in last 24 hours |
| `conditions` | []Condition | Standard conditions |

## Print Columns

```bash
kubectl get lm
NAME            MODEL    SYNCED   HEALTH    AGE
gpt4o           gpt-4o   true     healthy   3d
claude-sonnet   claude   true     healthy   3d
```

## Multiple Providers for the Same Model

Register the same model name from multiple providers for automatic fallback:

```yaml
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMModel
metadata:
  name: gpt4o-openai
spec:
  instanceRef:
    name: my-gateway
  modelName: gpt-4o
  litellmParams:
    model: openai/gpt-4o
    apiKeySecretRef:
      name: openai-credentials
      key: OPENAI_API_KEY
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMModel
metadata:
  name: gpt4o-azure
spec:
  instanceRef:
    name: my-gateway
  modelName: gpt-4o
  litellmParams:
    model: azure/gpt-4o
    apiBase: https://my-deployment.openai.azure.com/
    apiKeySecretRef:
      name: azure-credentials
      key: AZURE_API_KEY
```

LiteLLM's router handles load balancing and fallback between the two.

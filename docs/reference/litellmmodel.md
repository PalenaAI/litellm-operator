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
| `weight` | *int | No | Weighted load-balancing weight across deployments in this model group |
| `order` | *int | No | Routing priority within the model group (lower is preferred; higher-order deployments act as fallbacks) |
| `maxInputTokens` | *int | No | Context-window size used for context-window-aware routing / fallbacks |
| `temperature` | *float64 | No | Default temperature applied to requests to this deployment |
| `topP` | *float64 | No | Default `top_p` applied to requests to this deployment |
| `maxTokens` | *int | No | Default `max_tokens` (completion tokens) sent to the provider. Distinct from `modelInfo.maxTokens` |
| `seed` | *int | No | Default seed for reproducible outputs (providers that support it) |
| `organization` | string | No | Provider organization ID (e.g. an OpenAI organization) |
| `awsRegionName` | string | No | AWS region for Bedrock / SageMaker (e.g. `us-east-1`) |
| `extraHeaders` | map[string]string | No | Extra HTTP headers sent on every upstream request to this deployment |

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
| `mode` | string | Model type so LiteLLM runs the correct health check / routing. Common values: `chat`, `completion`, `embedding`, `image_generation`, `audio_transcription`, `audio_speech`, `moderation`, `rerank`, `responses`, `batch`, `realtime` |
| `baseModel` | string | Maps this deployment to a known base model for accurate cost tracking. Required for Azure deployments where the deployment name differs from the underlying model |
| `tier` | string | Tier for tier-based routing (`free` / `paid`) |
| `regionName` | string | Region for region-based routing (e.g. `us-east-1`) |
| `accessGroups` | []string | Access groups a key/team must hold to route to this model |
| `supportedEnvironments` | []string | Environments that expose this deployment (e.g. `production`, `staging`, `development`) |
| `useInPassThrough` | *bool | Allow this deployment to be selected by pass-through endpoints |
| `inputCostPerPixel` | *float64 | Cost per pixel in USD for image models |
| `inputCostPerSecond` | *float64 | Cost per second in USD for audio / realtime models billed by duration |
| `cacheReadInputTokenCost` | *float64 | Cost per token in USD for provider prompt-cache reads |
| `cacheCreationInputTokenCost` | *float64 | Cost per token in USD for provider prompt-cache writes |
| `healthCheck` | *ModelHealthCheck | Per-model health-check tuning / disabling (see below) |

### `modelInfo.healthCheck`

Tunes or disables LiteLLM's health checks for this single deployment. All fields are optional; unset fields fall back to LiteLLM's defaults. These map to the health-check keys under `model_info` in `proxy_server_config.yaml`.

| Field | Type | Description |
| --- | --- | --- |
| `disableBackgroundHealthCheck` | *bool | Skip background health checks for this model (proxy runs with `background_health_checks` enabled). Useful for providers that bill/rate-limit probes, or models that reject the probe request shape |
| `timeoutSeconds` | *int | Health-check request timeout override (LiteLLM default 60s) |
| `maxTokens` | *int | `max_tokens` used for the health-check request |
| `maxTokensReasoning` | *int | Health-check `max_tokens` for reasoning models |
| `maxTokensNonReasoning` | *int | Health-check `max_tokens` for non-reasoning models |
| `reasoningEffort` | string | Reasoning effort for reasoning-model probes (e.g. `none`, `low`, `medium`, `high`) |
| `voice` | string | Voice for text-to-speech model probes (e.g. `alloy`) |
| `model` | string | Override the model used for the probe (for wildcard routes, e.g. `openai/gpt-4o-mini`) |

#### Disabling health checks for a model

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMModel
metadata:
  name: expensive-embeddings
spec:
  instanceRef:
    name: my-gateway
  modelName: text-embedding-3-large
  litellmParams:
    model: openai/text-embedding-3-large
    apiKeySecretRef:
      name: openai-credentials
      key: OPENAI_API_KEY
  modelInfo:
    mode: embedding
    healthCheck:
      disableBackgroundHealthCheck: true
```

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

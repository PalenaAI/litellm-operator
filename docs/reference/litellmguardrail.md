# LiteLLMGuardrail

Declares a content moderation / safety guardrail (PII detection, jailbreak prevention, prompt injection defence, content filtering) that is materialized into the proxy's `guardrails` config section. Virtual keys and teams opt into specific guardrails by name via `spec.guardrails` — a LiteLLM Enterprise feature.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMGuardrail`
**Short Name:** `lg`

## Why guardrails?

LiteLLM supports a large number of third-party guardrail providers that inspect prompts and completions for PII, policy violations, prompt injection, jailbreak attempts, and more. Without the operator you would hand-edit `proxy_server_config.yaml` and manage provider API keys by hand. `LiteLLMGuardrail` turns each integration into a declarative CR: the operator renders the `guardrails` list into the proxy config, injects provider API keys as env vars via `secretKeyRef`, and triggers a rollout of the LiteLLM Deployment whenever you add, update, or remove a guardrail.

## Supported providers

| Provider | When to use |
| --- | --- |
| `aporia` | Aporia Guardrails — SaaS platform for hallucination, PII, and policy controls |
| `lakera` | Lakera Guard — prompt injection and jailbreak detection |
| `bedrock` | AWS Bedrock Guardrails — regional, provider-integrated |
| `presidio` | Microsoft Presidio — open-source PII detection and redaction (run locally) |
| `guardrails_ai` | Guardrails AI Hub validators |
| `azure` | Azure AI Content Safety |
| `llm_guard` | LLM Guard — open-source prompt/response scanning |
| `llamaguard` | Meta Llama Guard — LLM-based content classification |
| `google_text_moderation` | Google Cloud Natural Language text moderation |
| `custom_guardrail` | Your own service behind the [LiteLLM custom guardrail interface](https://docs.litellm.ai/docs/proxy/guardrails/custom_guardrail) |

## Execution modes

| Mode | When it runs | Blocks the request? |
| --- | --- | --- |
| `pre_call` | Before the LLM request is dispatched | Yes — on failure the request never reaches the model |
| `post_call` | After the LLM response is received | Yes — on failure the response is replaced |
| `during_call` | In parallel with the LLM request | No — but response is replaced on failure |
| `logging_only` | Same as `during_call` but never blocks | No — result is logged/exported only |

## Examples

### Aporia (hosted, API key)

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMGuardrail
metadata:
  name: pii-detector
spec:
  instanceRef:
    name: my-gateway
  guardrailName: pii-detector
  provider: aporia
  mode: pre_call
  apiBase: https://gr-prd-dc.aporia.com
  apiKeySecretRef:
    name: aporia-credentials
    key: APORIA_API_KEY
  defaultOn: false
```

### Presidio (local, no API key)

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMGuardrail
metadata:
  name: presidio-redact
spec:
  instanceRef:
    name: my-gateway
  guardrailName: presidio-redact
  provider: presidio
  mode: pre_call
  # No apiKeySecretRef — point at an internal service instead.
  params:
    presidio_analyzer_api_base: http://presidio-analyzer.guardrails.svc.cluster.local:3000
    presidio_anonymizer_api_base: http://presidio-anonymizer.guardrails.svc.cluster.local:3000
```

### AWS Bedrock Guardrails (provider params)

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMGuardrail
metadata:
  name: bedrock-pii
spec:
  instanceRef:
    name: my-gateway
  guardrailName: bedrock-pii
  provider: bedrock
  mode: post_call
  apiKeySecretRef:
    name: aws-credentials
    key: AWS_SECRET_ACCESS_KEY
  params:
    guardrailIdentifier: abc123
    guardrailVersion: DRAFT
    aws_region_name: us-east-1
  envVars:
    - name: AWS_ACCESS_KEY_ID
      valueFrom:
        secretKeyRef:
          name: aws-credentials
          key: AWS_ACCESS_KEY_ID
```

### Assigning guardrails to keys and teams (enterprise)

Once a guardrail is declared, virtual keys and teams opt in by name:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: engineering-ci
spec:
  instanceRef:
    name: my-gateway
  keyAlias: engineering-ci
  guardrails:
    - pii-detector
    - bedrock-pii
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: engineering
spec:
  instanceRef:
    name: my-gateway
  teamAlias: engineering
  guardrails:
    - pii-detector
```

The names must match `spec.guardrailName` on a LiteLLMGuardrail CR bound to the same instance. Per-key/per-team guardrail assignment is a LiteLLM Enterprise feature — if LiteLLM rejects the call, the downstream controller sets `Reason: EnterpriseLicenseRequired` on the CR's `Synced` condition.

## Spec Fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | Reference to the LiteLLMInstance this guardrail belongs to |
| `guardrailName` | string | Yes | Unique name for this guardrail; referenced by keys/teams via `spec.guardrails` |
| `provider` | enum | Yes | One of `aporia`, `lakera`, `bedrock`, `presidio`, `guardrails_ai`, `azure`, `llm_guard`, `llamaguard`, `google_text_moderation`, `custom_guardrail` |
| `mode` | enum | Yes | One of `pre_call`, `post_call`, `during_call`, `logging_only` |
| `apiKeySecretRef` | SecretKeyRef | No | Reference to a Secret containing the provider API key. Omit for local/internal providers like presidio or a `custom_guardrail` pointing at an in-cluster service |
| `apiBase` | string | No | Provider API base URL |
| `defaultOn` | bool | No | When true, the guardrail runs on every request even when keys/teams do not explicitly opt in |
| `params` | map[string]string | No | Provider-specific parameters merged into `litellm_params`. Reserved keys (`guardrail`, `mode`, `api_key`, `api_base`, `default_on`) cannot be overridden |
| `envVars` | []EnvVar | No | Additional env vars for this guardrail (e.g., `AWS_ACCESS_KEY_ID`, `AWS_REGION`) — each becomes a container env var on the LiteLLM Deployment |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `configured` | bool | Whether the guardrail has been validated and picked up by the instance controller |
| `lastSyncTime` | *Time | Last successful validation/reconciliation time |
| `conditions` | []Condition | Standard conditions. `Ready=True` with reason `Validated` means the guardrail spec is valid |

### Ready condition reasons

| Reason | Meaning |
| --- | --- |
| `Validated` | Guardrail spec is valid and will be rendered by the instance controller |
| `InstanceNotFound` | `spec.instanceRef.name` does not resolve to a LiteLLMInstance in the same namespace |
| `SecretNotFound` | `spec.apiKeySecretRef.name` does not exist |
| `SecretKeyMissing` | The referenced Secret exists but does not contain `spec.apiKeySecretRef.key` |

## Print Columns

```bash
kubectl get lg
NAME              GUARDRAIL         PROVIDER    MODE         INSTANCE     CONFIGURED   AGE
pii-detector      pii-detector      aporia      pre_call     my-gateway   true         1h
presidio-redact   presidio-redact   presidio    pre_call     my-gateway   true         1h
bedrock-pii       bedrock-pii       bedrock     post_call    my-gateway   true         1h
```

## How it works

Guardrails are **config-level** resources: the operator materializes them into the `guardrails` section of `proxy_server_config.yaml` rather than calling the LiteLLM REST API. For each guardrail, the operator writes:

```yaml
guardrails:
  - guardrail_name: pii-detector
    litellm_params:
      guardrail: aporia
      mode: pre_call
      api_key: os.environ/GUARDRAIL_PII_DETECTOR_API_KEY
      api_base: https://gr-prd-dc.aporia.com
      default_on: false
```

The API key is never written to the ConfigMap. The operator injects a `GUARDRAIL_{SANITIZED_NAME}_API_KEY` environment variable on the LiteLLM Deployment backed by the Secret reference, and LiteLLM resolves the `os.environ/...` placeholder at startup. Any additional `envVars` declared on the CR are appended to the container's env list as-is.

### Env var naming

The env var name follows `GUARDRAIL_<SANITIZED>_API_KEY` where `<SANITIZED>` is the guardrail name uppercased with any non-alphanumeric characters replaced by underscores. For example:

| guardrailName | env var |
| --- | --- |
| `pii-detector` | `GUARDRAIL_PII_DETECTOR_API_KEY` |
| `aporia.prod` | `GUARDRAIL_APORIA_PROD_API_KEY` |
| `bedrock-pii` | `GUARDRAIL_BEDROCK_PII_API_KEY` |

### Watches

Both the guardrail controller and the LiteLLMInstance controller watch LiteLLMGuardrail objects:

- The **guardrail controller** validates the CR: checks the referenced instance exists and (if declared) the API key Secret exists and contains the expected key.
- The **LiteLLMInstance controller** re-reconciles the target instance whenever a guardrail is created, updated, or deleted — rebuilding the ConfigMap and rolling the Deployment with the new env vars.

There is nothing to "apply" manually: editing a LiteLLMGuardrail triggers an automatic rollout on the owning instance.

## Reconciliation

The guardrail controller follows this pattern:

1. Fetch the `LiteLLMGuardrail` CR.
2. Resolve `instanceRef` to ensure the target LiteLLMInstance exists.
3. If `apiKeySecretRef` is set, fetch the API key Secret and verify the expected key is present.
4. Set `status.configured = true` and the `Ready` condition.
5. Requeue every 5 minutes to re-validate the Secret.

When the CR is deleted, the operator's finalizer lets the instance controller's Watch strip the entry from the ConfigMap on the next instance reconciliation.

## Security considerations

- API keys are only read from Secrets — never stored in the CR spec or written to ConfigMaps.
- The operator's ServiceAccount needs `get` / `list` / `watch` permissions on Secrets in the namespaces it manages.
- Guardrails are namespace-scoped: they can only reference LiteLLMInstances in the same namespace.
- Rotating a provider API key is a Secret update — the instance controller's Watch on the Secret (via the guardrail Watch chain) rolls the Deployment with the new value.
- **Per-key/per-team guardrail assignment requires a LiteLLM Enterprise license.** Declaring a guardrail CR and rendering it into the config works on open-source LiteLLM, but opting a specific key or team in via `spec.guardrails` will cause the downstream controller to report `EnterpriseLicenseRequired` unless a license is active. See [Enterprise License](../guide/enterprise-license) for activation.

# External Secret Managers

LiteLLM can connect to external secret managers at runtime to fetch API keys, store virtual keys, and read configuration secrets — without the secrets ever being stored in Kubernetes etcd.

This is distinct from the [External Secrets Operator](https://external-secrets.io/) approach, which syncs secrets into Kubernetes Secrets. Both patterns are valid and can coexist:

| Approach | Secrets in etcd? | Operator changes needed? |
| --- | --- | --- |
| External Secrets Operator + `secretKeyRef` | Yes | None (already works) |
| LiteLLM native secret manager | No | `spec.secretManager` on `LiteLLMInstance` |

## Supported Providers

| Provider | `provider` value | Enterprise? |
| --- | --- | --- |
| AWS Secret Manager | `aws_secret_manager` | No |
| AWS KMS | `aws_kms` | No |
| Azure Key Vault | `azure_key_vault` | Yes |
| Google Secret Manager | `google_secret_manager` | No |
| Google KMS | `google_kms` | No |
| HashiCorp Vault | `hashicorp_vault` | No |

## Common Settings

These fields apply regardless of provider:

```yaml
spec:
  secretManager:
    provider: aws_secret_manager          # Required

    # Secret containing provider credentials (e.g., AWS_ACCESS_KEY_ID).
    # Omit when using workload identity (IRSA, GKE WI).
    credentialsSecretRef:
      name: litellm-aws-credentials

    # Env var names LiteLLM should resolve from the secret manager
    # instead of the pod environment.
    hostedKeys:
      - OPENAI_API_KEY
      - ANTHROPIC_API_KEY

    # Store generated virtual keys in the secret manager.
    storeVirtualKeys: true
    prefixForStoredVirtualKeys: "litellm/"

    # read_only | write_only | read_and_write
    accessMode: read_and_write

    # A single secret containing multiple key-value pairs as JSON.
    primarySecretName: "litellm/all-keys"
```

Secrets are referenced in the proxy config using `os.environ/SECRET_NAME` — LiteLLM resolves them from the configured secret manager instead of actual environment variables.

## AWS Secret Manager / AWS KMS

### With static credentials

```yaml
spec:
  secretManager:
    provider: aws_secret_manager
    credentialsSecretRef:
      name: litellm-aws-credentials
    hostedKeys:
      - OPENAI_API_KEY
    accessMode: read_only
    aws:
      region: us-east-1
---
apiVersion: v1
kind: Secret
metadata:
  name: litellm-aws-credentials
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: "AKIAIOSFODNN7EXAMPLE"
  AWS_SECRET_ACCESS_KEY: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

### With IRSA (EKS)

On EKS, use IAM Roles for Service Accounts to avoid static credentials entirely:

```yaml
spec:
  secretManager:
    provider: aws_secret_manager
    # No credentialsSecretRef — IRSA handles authentication
    hostedKeys:
      - OPENAI_API_KEY
    accessMode: read_only
    aws:
      region: us-east-1
      roleARN: arn:aws:iam::123456789012:role/litellm-secrets
      sessionName: litellm
      webIdentityTokenPath: /var/run/secrets/eks.amazonaws.com/serviceaccount/token
```

The operator injects `AWS_REGION_NAME`, `aws_role_name`, `aws_session_name`, and `aws_web_identity_token` as environment variables.

### With STS endpoint (VPC)

For environments with a custom STS endpoint:

```yaml
    aws:
      region: us-east-1
      stsEndpoint: https://sts.us-east-1.amazonaws.com
```

## Azure Key Vault

::: warning Enterprise Feature
Azure Key Vault requires a LiteLLM Enterprise license.
:::

```yaml
spec:
  secretManager:
    provider: azure_key_vault
    credentialsSecretRef:
      name: litellm-azure-credentials
    hostedKeys:
      - OPENAI_API_KEY
    accessMode: read_only
    azure:
      vaultURI: https://my-vault.vault.azure.net
      tenantID: 00000000-0000-0000-0000-000000000000
---
apiVersion: v1
kind: Secret
metadata:
  name: litellm-azure-credentials
type: Opaque
stringData:
  AZURE_CLIENT_ID: "..."
  AZURE_CLIENT_SECRET: "..."
```

The operator injects `AZURE_KEY_VAULT_URI` and `AZURE_TENANT_ID` as environment variables, and the credentials Secret keys via `envFrom`.

## Google Secret Manager / Google KMS

```yaml
spec:
  secretManager:
    provider: google_secret_manager
    credentialsSecretRef:
      name: litellm-gcp-credentials
    hostedKeys:
      - OPENAI_API_KEY
    accessMode: read_only
---
apiVersion: v1
kind: Secret
metadata:
  name: litellm-gcp-credentials
type: Opaque
stringData:
  GOOGLE_APPLICATION_CREDENTIALS: |
    {
      "type": "service_account",
      ...
    }
```

On GKE with Workload Identity, omit `credentialsSecretRef` and configure your ServiceAccount for Workload Identity Federation instead.

## HashiCorp Vault

### AppRole auth (default)

```yaml
spec:
  secretManager:
    provider: hashicorp_vault
    credentialsSecretRef:
      name: litellm-vault-credentials
    hostedKeys:
      - OPENAI_API_KEY
    accessMode: read_only
    vault:
      address: https://vault.example.com:8200
      namespace: admin
      authMethod: approle       # default
      mountName: secret
      pathPrefix: litellm/
      refreshInterval: 300      # seconds
---
apiVersion: v1
kind: Secret
metadata:
  name: litellm-vault-credentials
type: Opaque
stringData:
  HCP_VAULT_APPROLE_ROLE_ID: "..."
  HCP_VAULT_APPROLE_SECRET_ID: "..."
```

### Token auth

```yaml
    vault:
      address: https://vault.example.com:8200
      authMethod: token
---
apiVersion: v1
kind: Secret
metadata:
  name: litellm-vault-credentials
type: Opaque
stringData:
  HCP_VAULT_TOKEN: "hvs.EXAMPLE..."
```

## Status

When a secret manager is configured, the operator reports its status:

```yaml
status:
  secretManager:
    configured: true
    provider: aws_secret_manager
```

If the `credentialsSecretRef` points to a Secret that doesn't exist, `configured` is set to `false` and a `SecretNotFound` warning event is emitted on the instance.

## Combining with Kubernetes Secrets

You can use both approaches simultaneously. For example:

- Use the secret manager for provider API keys (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`)
- Use Kubernetes Secrets for the master key and database credentials (`spec.masterKey.secretRef`, `spec.database.external.connectionSecretRef`)
- Use `LiteLLMCredential` CRDs for model-specific credentials

The secret manager handles keys referenced as `os.environ/NAME` in the LiteLLM config; Kubernetes Secrets handle values injected directly as pod environment variables.

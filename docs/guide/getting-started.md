# Getting Started

This guide walks you through deploying your first LiteLLM instance with the operator.

## Prerequisites

- Kubernetes v1.28+ cluster
- `kubectl` configured to access your cluster
- A PostgreSQL database (external or operator-managed)
- Provider API keys (OpenAI, Anthropic, etc.)

## 1. Install the Operator

The quickest way to install for development:

```bash
# Install CRDs
make install

# Deploy the operator
make deploy IMG=ghcr.io/palenaai/litellm-operator:latest
```

See the [Installation guide](/guide/installation) for OLM and Helm options.

## 2. Create a Database Secret

LiteLLM requires a PostgreSQL database. Create a Secret with your connection string:

```bash
kubectl create secret generic litellm-db-credentials \
  --from-literal=DATABASE_URL='postgresql://user:pass@host:5432/litellm'
```

## 3. Deploy a LiteLLM Instance

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  replicas: 2
  image:
    repository: ghcr.io/berriai/litellm
    tag: main-v1.60.0
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: litellm-db-credentials
        key: DATABASE_URL
  service:
    type: ClusterIP
    port: 4000
  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: "1"
      memory: 512Mi
```

```bash
kubectl apply -f instance.yaml
```

The operator creates a Deployment, ConfigMap, Service, and runs a database migration Job. Check status:

```bash
kubectl get litellminstances
# or using the short name:
kubectl get li
```

## 4. Create a Provider Secret

```bash
kubectl create secret generic openai-credentials \
  --from-literal=OPENAI_API_KEY='sk-...'
```

## 5. Register a Model

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
```

```bash
kubectl apply -f model.yaml
kubectl get litellmmodels  # or: kubectl get lm
```

## 6. Create a Team

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: engineering
spec:
  instanceRef:
    name: my-gateway
  teamAlias: engineering
  models:
    - gpt-4o
  maxBudgetMonthly: 1000
  budgetDuration: "30d"
  memberManagement: mixed
  members:
    - email: lead@example.com
      role: admin
    - email: dev@example.com
      role: user
```

```bash
kubectl apply -f team.yaml
kubectl get litellmteams  # or: kubectl get lt
```

## 7. Generate an API Key

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
```

```bash
kubectl apply -f key.yaml
```

The operator generates an API key and stores it in a Kubernetes Secret:

```bash
# Retrieve the generated API key
kubectl get secret eng-ci-key-key -o jsonpath='{.data.api-key}' | base64 -d
```

## 8. Test the Gateway

```bash
# Port-forward to the LiteLLM service
kubectl port-forward svc/my-gateway 4000:4000

# Make a request
curl http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $(kubectl get secret eng-ci-key-key -o jsonpath='{.data.api-key}' | base64 -d)" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Next Steps

- [Architecture](/guide/architecture) — understand how the operator works
- [Enterprise License](/guide/enterprise-license) — activate LiteLLM Enterprise features
- [Caching](/guide/caching) — configure response caching
- [Config Sync](/guide/config-sync) — learn about bidirectional synchronization
- [SSO Setup](/guide/sso) — configure single sign-on
- [CRD Reference](/reference/crds) — full field reference for all CRDs

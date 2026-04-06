# Installation

The LiteLLM Operator can be installed three ways depending on your cluster setup.

## OLM (OpenShift / OperatorHub)

For clusters with the Operator Lifecycle Manager installed (all OpenShift clusters, or vanilla Kubernetes with OLM):

```bash
operator-sdk run bundle ghcr.io/palenaai/litellm-operator-bundle:v0.5.0
```

Verify the installation:

```bash
kubectl get csv -n operators
kubectl get crd | grep litellm
```

To uninstall:

```bash
operator-sdk cleanup litellm-operator
```

## Helm Chart

For vanilla Kubernetes, k3s, RKE2, and other clusters without OLM:

```bash
helm install litellm-operator deploy/charts/litellm-operator/
```

To customize values:

```bash
helm install litellm-operator deploy/charts/litellm-operator/ \
  --set image.repository=ghcr.io/palenaai/litellm-operator \
  --set image.tag=v0.5.0
```

To uninstall:

```bash
helm uninstall litellm-operator
```

## Direct (Makefile)

For development and CI/CD:

```bash
# Install CRDs only
make install

# Deploy the operator to the cluster
make deploy IMG=ghcr.io/palenaai/litellm-operator:v0.5.0

# Or run locally against your kubeconfig cluster
make run
```

To uninstall:

```bash
make undeploy
make uninstall
```

## Single YAML Install

For a standalone install without Helm or OLM:

```bash
# Build the installer manifest
make build-installer IMG=ghcr.io/palenaai/litellm-operator:v0.5.0

# Apply it
kubectl apply -f dist/install.yaml
```

## Namespace-Scoped Watching

By default, the operator watches all namespaces. To restrict it to specific namespaces:

### Helm

```bash
helm install litellm-operator deploy/charts/litellm-operator/ \
  --set watchNamespaces="team-a,team-b"
```

### Flag

Pass `--watch-namespaces` to the manager binary:

```bash
/manager --watch-namespaces=team-a,team-b
```

### OLM

When installed via OLM in `OwnNamespace` or `SingleNamespace` mode, OLM automatically sets the `WATCH_NAMESPACE` environment variable. The operator reads this variable and scopes its watches accordingly — no additional configuration needed.

### Makefile (development)

Set the environment variable before running:

```bash
WATCH_NAMESPACE=default make run
```

## Verify Installation

After installation, verify the operator is running:

```bash
# Check the operator pod
kubectl get pods -n litellm-operator-system

# Check CRDs are installed
kubectl get crd | grep litellm

# Expected CRDs:
# litellminstances.litellm.palena.ai
# litellmmodels.litellm.palena.ai
# litellmteams.litellm.palena.ai
# litellmusers.litellm.palena.ai
# litellmvirtualkeys.litellm.palena.ai
```

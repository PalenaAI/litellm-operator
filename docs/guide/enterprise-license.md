# Enterprise License

The operator supports convention-based activation of LiteLLM Enterprise features. When a license Secret is detected, the operator injects the `LITELLM_LICENSE` environment variable into the proxy Deployment. No CRD fields are required — the operator discovers the Secret automatically.

## How It Works

1. You create a Kubernetes Secret containing your LiteLLM Enterprise license key
2. The operator detects the Secret during reconciliation
3. The `LITELLM_LICENSE` env var is injected into the Deployment via `secretKeyRef` (the license value is never read into operator memory)
4. LiteLLM validates the license at startup and enables enterprise features
5. License status is reported in `.status.license`

The operator watches license Secrets and triggers a rolling restart of the Deployment when a license is added, updated, or removed.

## Secret Convention

### Naming

The operator checks for Secrets in this order (first match wins):

1. **`{instance-name}-license`** — per-instance override
2. **`litellm-license`** — namespace-wide fallback (covers all instances in the namespace)

### Structure

The Secret must be `Opaque` with a `license-key` key:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-gateway-license   # or litellm-license for shared
  namespace: litellm
type: Opaque
stringData:
  license-key: "your-litellm-enterprise-license-key"
```

## Per-Instance License

Create a Secret named `{instance-name}-license`:

```bash
kubectl create secret generic my-gateway-license \
  --from-literal=license-key='your-litellm-enterprise-license-key'
```

This activates the license only for the `my-gateway` instance.

## Namespace-Wide License

Create a Secret named `litellm-license`:

```bash
kubectl create secret generic litellm-license \
  --from-literal=license-key='your-litellm-enterprise-license-key'
```

This activates the license for all `LiteLLMInstance` resources in the namespace. A per-instance Secret always takes precedence over the namespace-wide fallback.

## Checking License Status

```bash
kubectl get litellminstance my-gateway -o jsonpath='{.status.license}'
```

```json
{"active": true, "secretName": "my-gateway-license"}
```

When no license Secret is found:

```json
{"active": false}
```

## Enterprise Feature Errors

If a downstream resource (Model, Team, User, VirtualKey) attempts an operation that requires an enterprise license and LiteLLM returns a 403 error, the operator sets a clear status condition:

```yaml
status:
  conditions:
    - type: Synced
      status: "False"
      reason: EnterpriseLicenseRequired
      message: "This feature requires a LiteLLM Enterprise license. Create a license Secret to activate."
```

The operator does **not** requeue the resource in this case — the condition persists until the user creates a license Secret, which triggers reconciliation of the `LiteLLMInstance` and eventually the downstream resources.

## Removing a License

Delete the license Secret:

```bash
kubectl delete secret my-gateway-license
```

The operator detects the deletion, removes the `LITELLM_LICENSE` env var from the Deployment, and triggers a rolling restart. The status updates to `license.active: false`.

## Design Decisions

- **No separate CRD** — a Secret naming convention keeps things simple and avoids CRD proliferation
- **No operator-side validation** — the operator passes the license key through to LiteLLM, which owns the license format and validation
- **No owner reference** — the license Secret is user-managed and must survive `LiteLLMInstance` deletion
- **Secret-only storage** — the license key is referenced via `secretKeyRef` and never stored in the operator's memory or logs

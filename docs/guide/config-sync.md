# Bidirectional Config Sync

The operator's distinguishing feature is **bidirectional config sync** — a reconciliation loop that keeps CRD state and LiteLLM's runtime state (stored in PostgreSQL, accessed via REST API) in sync.

## The Problem it Solves

With the Helm chart approach, LiteLLM stores models, teams, users, and keys in its database. The Admin UI writes directly to that database. But the Helm chart only manages the static `proxy_server_config.yaml` — so:

- Changes made via the **Admin UI** are invisible to GitOps
- A Helm upgrade can **overwrite** Admin UI changes
- There's no single source of truth

## How Config Sync Works

The operator manages configuration through the **LiteLLM REST API** instead of the config file. Both the CRDs and the Admin UI write to the same backing store (PostgreSQL via the API), so they coexist naturally.

The config sync loop runs on a configurable interval (default 30s):

```
Config Sync Loop
│
├── 1. Collect desired state from all CRDs referencing this instance
│   ├── List all LiteLLMModel CRs
│   ├── List all LiteLLMTeam CRs
│   ├── List all LiteLLMUser CRs
│   ├── List all LiteLLMVirtualKey CRs
│   ├── List all LiteLLMOrganization CRs
│   └── List all LiteLLMCustomer CRs
│
├── 2. Fetch actual state from LiteLLM API
│   ├── GET /model/info
│   ├── GET /team/list
│   ├── GET /user/list
│   ├── GET /key/list
│   ├── GET /organization/list
│   └── GET /customer/list
│
├── 3. Compute diff
│   ├── In CRD but not in API → create
│   ├── In both but different → update
│   └── In API but not in CRD → apply unmanaged policy
│
├── 4. Apply changes via LiteLLM API
│
└── 5. Update CRD statuses
```

## Enabling Config Sync

Add the `configSync` section to your `LiteLLMInstance`:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  # ... other fields ...
  configSync:
    enabled: true
    interval: "30s"
    unmanagedResourcePolicy: preserve
    conflictResolution: crd-wins
    auditChanges: true
```

## Unmanaged Resource Policies

When the sync loop finds resources in the LiteLLM API that don't have a corresponding CRD, the `unmanagedResourcePolicy` controls what happens:

### `preserve` (default)

Leave unmanaged resources alone. This is the safest option and allows the Admin UI and CRDs to coexist.

**Example:** A team member creates a model via the Admin UI. The operator sees it during sync but ignores it since there's no matching CRD. The model continues to work.

### `prune`

Delete unmanaged resources. The CRDs become the sole source of truth — anything not declared as a CRD is removed.

::: warning
Use `prune` with caution. Any resources created via the Admin UI, SCIM, or direct API calls will be deleted on the next sync cycle.
:::

### `adopt`

Report unmanaged resources in the instance status without modifying them. Useful for auditing what exists outside of CRD management.

## Conflict Resolution

When a resource exists in both the CRD and the API but they differ, the `conflictResolution` strategy determines which wins:

| Strategy | Behavior |
| --- | --- |
| `crd-wins` (default) | CRD state is pushed to the API, overwriting any API-side changes |
| `api-wins` | API state is preserved; CRD status reflects the API state |
| `manual` | Conflict is reported in status conditions; no automatic resolution |

## Change Detection

The operator uses **content hashing** to detect changes efficiently. A SHA-256 hash of the spec is stored as an annotation (`litellm.palena.ai/sync-hash`) on each CR. During sync, the current hash is compared to the stored hash — if different, the resource is queued for update.

## Audit Changes

When `auditChanges: true`, the operator emits Kubernetes Events for every change made during sync:

```bash
kubectl get events --field-selector reason=ConfigSyncUpdate
```

## Status Reporting

The `LiteLLMInstance` status reports sync statistics:

```yaml
status:
  configSync:
    lastSyncTime: "2026-04-01T12:00:00Z"
    syncedModels: 5
    syncedTeams: 3
    syncedUsers: 10
    syncedKeys: 8
    syncedOrganizations: 2
    syncedCustomers: 15
    unmanagedModels: 2
    unmanagedTeams: 0
    unmanagedUsers: 5
    unmanagedKeys: 1
    unmanagedOrganizations: 0
    unmanagedCustomers: 3
```

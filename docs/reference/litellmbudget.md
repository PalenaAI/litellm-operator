# LiteLLMBudget

Defines a reusable **budget / rate-limit tier** registered with a LiteLLM proxy via the REST API (`POST /budget/new`). Other resources reference it by `budget_id` instead of repeating inline limits — e.g. [`LiteLLMVirtualKey.spec.budgetId`](./litellmvirtualkey) or `LiteLLMInstance.spec.defaultCustomerBudget.budgetId`.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMBudget`
**Short Name:** `lb`

## Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMBudget
metadata:
  name: tier-standard
spec:
  instanceRef:
    name: my-gateway
  budgetId: tier-standard   # stable id others reference; defaults to metadata.name
  maxBudget: 100.0
  softBudget: 80.0
  budgetDuration: 30d
  tpmLimit: 100000
  rpmLimit: 1000
  maxParallelRequests: 20
```

## Spec Fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | Reference to the LiteLLMInstance |
| `budgetId` | string | No | Stable identifier other resources reference (sent as `budget_id`). Defaults to `metadata.name` |
| `maxBudget` | *float64 | No | Maximum budget in USD |
| `softBudget` | *float64 | No | Alert threshold in USD below `maxBudget` (does not block) |
| `budgetDuration` | string | No | Reset interval (e.g. `1d`, `7d`, `30d`) |
| `tpmLimit` | *int | No | Tokens-per-minute limit |
| `rpmLimit` | *int | No | Requests-per-minute limit |
| `maxParallelRequests` | *int | No | Maximum concurrent requests |
| `modelMaxBudget` | map[string]float64 | No | Per-model budget caps in USD (model name → max budget) |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the budget is synced to LiteLLM |
| `litellmBudgetId` | string | The `budget_id` assigned/used in LiteLLM |
| `currentSpend` | *float64 | Current spend in USD, refreshed from `/budget/info` |
| `lastSyncTime` | *Time | Last successful sync time |
| `conditions` | []Condition | Standard conditions |

## Referencing a budget

Assign the tier to a key by `budget_id`:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: app-key
spec:
  instanceRef:
    name: my-gateway
  budgetId: tier-standard   # → the LiteLLMBudget above
```

The operator deletes the budget from LiteLLM when the CR is removed (finalizer). Deleting a budget still referenced by keys is a LiteLLM-side concern — remove or repoint the references first.

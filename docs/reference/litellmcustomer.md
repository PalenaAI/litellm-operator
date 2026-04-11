# LiteLLMCustomer

Manages external end-users (customers) of the LiteLLM AI gateway with per-customer budgets, rate limits, and model access policies.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMCustomer`
**Short Name:** `lcust`

## What is a Customer?

Unlike [LiteLLMUser](./litellmuser) — which represents an *internal* proxy user (an admin, developer, or service account that calls LiteLLM directly) — a **Customer** represents an *external* end-user of an application built on top of LiteLLM. Typical use cases:

- A SaaS product where each paying customer is identified by an ID and has a per-customer spend cap.
- A mobile app that passes an end-user ID to the proxy so usage and budgets are tracked per app user.
- A multi-tenant API where the tenant is tracked as the `user_id` in each request.

Customers are identified by LiteLLM as **end_users**. The application sends requests to LiteLLM with the customer ID (via the `user` field in the OpenAI request body or a header), and LiteLLM applies the customer's budget/rate limits at request time. The operator manages the customer records themselves — creation, updates, deletion, spend tracking.

## Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMCustomer
metadata:
  name: customer-42
spec:
  instanceRef:
    name: my-gateway
  customerId: customer-42
  alias: "Acme End-User"
  maxBudget: 100
  budgetDuration: "30d"
  tpmLimit: 50000
  rpmLimit: 500
  models:
    - gpt-4o
    - claude-4-sonnet
  metadata:
    source: crm
    tier: premium
```

## Spec Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | — | Reference to the LiteLLMInstance |
| `customerId` | string | Yes | — | External user ID; maps to LiteLLM's `user_id` / `end_user_id` |
| `alias` | string | No | — | Human-readable display name |
| `maxBudget` | *float64 | No | — | Maximum spend in USD |
| `budgetDuration` | string | No | — | Budget reset period (e.g., `1d`, `7d`, `30d`) |
| `budgetId` | string | No | — | Reference to a predefined budget tier ID |
| `tpmLimit` | *int64 | No | — | Tokens per minute limit |
| `rpmLimit` | *int64 | No | — | Requests per minute limit |
| `models` | []string | No | — | List of models the customer may call |
| `defaultModel` | string | No | — | Default model when no model is specified in the request |
| `allowedModelRegion` | string | No | — | Restrict the customer to models in a specific region (e.g., `eu`, `us`) |
| `blocked` | *bool | No | — | If `true`, the customer cannot make any requests |
| `objectPermission` | [ObjectPermission](#objectpermission) | No | — | Restrict access to MCP servers, vector stores, agents, and access groups |
| `metadata` | map[string]string | No | — | Arbitrary metadata stored with the customer |

### ObjectPermission

Controls which advanced objects the customer is permitted to use. All fields are optional.

| Field | Type | Description |
| --- | --- | --- |
| `mcpServers` | []string | Allowed MCP servers |
| `accessGroups` | []string | Allowed access groups |
| `vectorStores` | []string | Allowed vector stores |
| `agents` | []string | Allowed agents |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the customer is synced to LiteLLM |
| `currentSpend` | *float64 | Current spend in USD, refreshed from `/customer/info` |
| `blocked` | bool | Whether the customer is blocked (as reported by LiteLLM) |
| `lastSyncTime` | *Time | Last successful sync time |
| `conditions` | []Condition | Standard conditions (`Synced`) |

## Print Columns

```bash
kubectl get lcust
NAME          CUSTOMERID    MAXBUDGET   SPEND   SYNCED   AGE
customer-42   customer-42   100         12.34   true     2d
```

## Budgets: inline vs. tier

A customer's budget can be defined two ways, which are mutually exclusive in practice:

- **Inline:** set `maxBudget` and `budgetDuration` directly on the customer.
- **Tier:** set `budgetId` to reference a predefined budget (created via the LiteLLM budget API).

Tiers are useful when you have many customers sharing a budget/rate profile — updating the tier updates every customer referencing it.

## Default budget for all customers

You can set a platform-wide default budget applied to *any* customer created on this instance (including those created implicitly at request time). Configure it on the `LiteLLMInstance`:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  defaultCustomerBudget:
    maxBudget: 10        # USD
    # or, use a predefined tier:
    # budgetId: "tier-free"
```

This is written to the proxy config as `litellm_settings.max_end_user_budget` / `litellm_settings.max_end_user_budget_id`.

## Reconciliation

The customer controller follows the standard reconciliation pattern:

1. Fetch the `LiteLLMCustomer` CR.
2. Resolve the `instanceRef` to the target LiteLLMInstance, load endpoint + master key.
3. If not yet synced, call `POST /customer/new`.
4. If the spec hash changed, call `POST /customer/update`.
5. Refresh `status.currentSpend` and `status.blocked` from `GET /customer/info`.
6. Requeue every 5 minutes.

When the CR is deleted, the finalizer calls `POST /customer/delete` to remove the customer from LiteLLM.

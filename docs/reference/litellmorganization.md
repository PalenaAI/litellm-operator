# LiteLLMOrganization

Creates and manages an organization for multi-tenant isolation with budget limits, model access control, and member management.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMOrganization`
**Short Name:** `lo`

## Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMOrganization
metadata:
  name: acme-corp
spec:
  instanceRef:
    name: my-gateway
  organizationAlias: acme-corp
  models:
    - gpt-4o
    - claude-4-sonnet
  maxBudget: 5000
  budgetDuration: "30d"
  rpmLimit: 1000
  tpmLimit: 200000
  members:
    - email: admin@acme.com
      role: org_admin
    - email: user@acme.com
      role: internal_user
  metadata:
    department: engineering
```

## Spec Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | — | Reference to the LiteLLMInstance |
| `organizationAlias` | string | Yes | — | Human-readable organization name |
| `models` | []string | No | — | Models this organization can access |
| `maxBudget` | *float64 | No | — | Maximum budget in USD |
| `budgetDuration` | string | No | — | Budget reset period (e.g., `30d`, `7d`, `1d`) |
| `tpmLimit` | *int64 | No | — | Tokens per minute limit |
| `rpmLimit` | *int64 | No | — | Requests per minute limit |
| `softBudget` | *float64 | No | — | Alert threshold in USD below `maxBudget` (does not block) |
| `modelRpmLimit` | map[string]int | No | — | Per-model requests-per-minute caps (model name → RPM) |
| `modelTpmLimit` | map[string]int | No | — | Per-model tokens-per-minute caps (model name → TPM) |
| `objectPermission` | *ObjectPermission | No | — | Grant access to MCP servers, vector stores, agents, access groups |
| `members` | []OrganizationMember | No | — | Organization members |
| `metadata` | map[string]string | No | — | Custom metadata |

### `members[]`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `email` | string | — | User email address |
| `role` | string | `internal_user` | `org_admin` or `internal_user` |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the organization is synced to LiteLLM |
| `litellmOrganizationId` | string | LiteLLM-assigned organization ID |
| `currentSpend` | *float64 | Current spend in USD |
| `memberCount` | int | Number of members |
| `lastSyncTime` | *Time | Last successful sync time |
| `conditions` | []Condition | Standard conditions |

## Print Columns

```bash
kubectl get lo
NAME        ALIAS       MEMBERS   SYNCED   AGE
acme-corp   acme-corp   5         true     2d
```

## Team Association

Teams can be scoped to an organization using `spec.organizationRef`:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: acme-engineering
spec:
  instanceRef:
    name: my-gateway
  organizationRef:
    name: acme-corp
  teamAlias: acme-engineering
  models: [gpt-4o]
```

The team controller resolves the organization reference to its LiteLLM-assigned ID and passes `organization_id` to the team create/update API calls. If the referenced organization is not yet synced, the team controller requeues after 30 seconds.

## Member Management

The organization controller syncs members by comparing `spec.members` with the API state:

- **Missing members** are added via `/organization/member_add`
- **Extra members** (in API but not in spec) are removed via `/organization/member_delete`

Member roles:
- `org_admin` — full admin access to the organization (requires enterprise license)
- `internal_user` — standard member access

## LiteLLM Hierarchy

Organizations are the top level of LiteLLM's multi-tenant hierarchy:

```
Organization (LiteLLMOrganization)
├── Team (LiteLLMTeam)
│   ├── User (LiteLLMUser)
│   └── VirtualKey (LiteLLMVirtualKey)
└── Team (LiteLLMTeam)
    └── ...
```

An organization's budget and model access constraints apply to all teams within it.

# LiteLLMUser

Creates a user in LiteLLM for non-SSO environments. Useful for service accounts, bot users, and environments without an Identity Provider.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMUser`
**Short Name:** `lu`

## Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMUser
metadata:
  name: service-bot
spec:
  instanceRef:
    name: my-gateway
  userId: service-bot@example.com
  userEmail: service-bot@example.com
  userRole: internal_user
  maxBudget: 500
  budgetDuration: "30d"
  models:
    - gpt-4o
  teams:
    - teamRef:
        name: engineering
      role: user
```

## Spec Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | — | Reference to the LiteLLMInstance |
| `userId` | string | Yes | — | Unique user identifier (typically email) |
| `userEmail` | string | No | — | User email address |
| `userRole` | string | No | `internal_user` | User role (see below) |
| `maxBudget` | *float64 | No | — | Maximum budget in USD |
| `budgetDuration` | string | No | — | Budget reset period (e.g., `30d`) |
| `models` | []string | No | — | Models this user can access |
| `teams` | []UserTeamMembership | No | — | Team memberships |
| `tpmLimit` | *int | No | — | Tokens per minute limit |
| `rpmLimit` | *int | No | — | Requests per minute limit |
| `metadata` | map[string]string | No | — | Custom metadata |
| `blocked` | *bool | No | — | Disable all requests from this user without deleting it |
| `softBudget` | *float64 | No | — | Alert threshold in USD below `maxBudget` (does not block) |
| `modelRpmLimit` | map[string]int | No | — | Per-model requests-per-minute caps (model name → RPM) |
| `modelTpmLimit` | map[string]int | No | — | Per-model tokens-per-minute caps (model name → TPM) |
| `objectPermission` | *ObjectPermission | No | — | Grant access to MCP servers, vector stores, agents, access groups |

### User Roles

| Role | Description |
| --- | --- |
| `proxy_admin` | Full admin access to all LiteLLM features |
| `proxy_admin_viewer` | Read-only admin access |
| `internal_user` | Standard user with scoped access |
| `internal_user_viewer` | Read-only standard user |

### `teams[]`

| Field | Type | Description |
| --- | --- | --- |
| `teamRef` | *InstanceRef | Reference to a `LiteLLMTeam` CR |
| `teamId` | string | Direct team ID (for teams not managed by a CRD) |
| `role` | string | Role within the team (default: `user`) |
| `maxBudgetInTeam` | *float64 | Max budget within this team |

You can use either `teamRef` (references a `LiteLLMTeam` CR by name) or `teamId` (direct LiteLLM team ID). The operator resolves `teamRef` to the team's `status.litellmTeamId`.

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the user is synced to LiteLLM |
| `litellmUserId` | string | LiteLLM-assigned user ID |
| `currentSpend` | *float64 | Current spend in USD |
| `resolvedTeams` | []ResolvedTeamMembership | Resolved team memberships |
| `lastSyncTime` | *Time | Last successful sync time |
| `conditions` | []Condition | Standard conditions |

## Print Columns

```bash
kubectl get lu
NAME          USERID                     ROLE            SYNCED   AGE
service-bot   service-bot@example.com    internal_user   true     1d
```

## When to Use LiteLLMUser

- **Service accounts** for CI/CD pipelines
- **Bot users** that need API access
- **Non-SSO environments** where you manage users declaratively
- **GitOps user management** alongside SSO for specific accounts

When SSO/SCIM handles user provisioning, `LiteLLMUser` CRDs are typically not needed for human users.

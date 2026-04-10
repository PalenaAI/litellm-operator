# LiteLLMTeam

Creates and manages a team with budget limits, rate limits, and configurable member management.

**API Version:** `litellm.palena.ai/v1alpha1`
**Kind:** `LiteLLMTeam`
**Short Name:** `lt`

## Example

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
    - claude-4-sonnet
  maxBudgetMonthly: 1000
  budgetDuration: "30d"
  rpmLimit: 200
  tpmLimit: 50000
  tags: ["paid"]
  memberManagement: mixed
  members:
    - email: lead@example.com
      role: admin
    - email: dev@example.com
      role: user
```

## Spec Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | — | Reference to the LiteLLMInstance |
| `teamAlias` | string | Yes | — | Human-readable team name |
| `models` | []string | No | — | Models this team can access |
| `maxBudgetMonthly` | *float64 | No | — | Maximum monthly budget in USD |
| `budgetDuration` | string | No | — | Budget reset period (e.g., `30d`, `7d`) |
| `tpmLimit` | *int | No | — | Tokens per minute limit |
| `rpmLimit` | *int | No | — | Requests per minute limit |
| `teamMemberRpmLimit` | *int | No | — | Per-member RPM limit |
| `teamMemberTpmLimit` | *int | No | — | Per-member TPM limit |
| `metadata` | map[string]string | No | — | Custom metadata |
| `tags` | []string | No | — | Tags for tag-based routing (keys inherit these tags) |
| `memberManagement` | string | No | `mixed` | `crd`, `sso`, or `mixed` |
| `members` | []TeamMember | No | — | Team members list |

### `members[]`

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `email` | string | — | User email address |
| `role` | string | `user` | `admin` or `user` |

## Status Fields

| Field | Type | Description |
| --- | --- | --- |
| `synced` | bool | Whether the team is synced to LiteLLM |
| `litellmTeamId` | string | LiteLLM-assigned team ID |
| `currentSpend` | *float64 | Current spend in USD |
| `totalMemberCount` | int | Total members (CRD + SSO) |
| `crdMembers` | []TeamMemberStatus | Members managed by the CRD |
| `ssoMembers` | []TeamMemberStatus | Members provisioned by SSO |
| `lastSyncTime` | *Time | Last successful sync time |
| `conditions` | []Condition | Standard conditions |

## Print Columns

```bash
kubectl get lt
NAME          ALIAS         MEMBERS   MEMBERMGMT   SYNCED   AGE
engineering   engineering   5         mixed        true     2d
```

## Member Management

See [Team Member Management](/guide/team-members) for a detailed explanation of the three modes.

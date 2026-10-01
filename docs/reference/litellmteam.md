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

## Per-Team Logging Example

Route logs for a team to a dedicated Langfuse project (enterprise):

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: engineering
spec:
  instanceRef:
    name: my-gateway
  teamAlias: engineering
  logging:
    callbacks:
      - name: langfuse
        type: success_and_failure
        credentialsSecretRef:
          name: engineering-langfuse
        config:
          langfuse_host: "https://cloud.langfuse.com"
---
apiVersion: v1
kind: Secret
metadata:
  name: engineering-langfuse
stringData:
  langfuse_public_key: "pk-team-engineering"
  langfuse_secret: "sk-team-engineering"
```

Disable all logging for a team (GDPR compliance):

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: gdpr-team
spec:
  instanceRef:
    name: my-gateway
  teamAlias: gdpr-team
  logging:
    disabled: true
```

## Spec Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `instanceRef` | InstanceRef | Yes | — | Reference to the LiteLLMInstance |
| `organizationRef` | *OrganizationRef | No | — | Reference to a [LiteLLMOrganization](/reference/litellmorganization) in the same namespace |
| `teamId` | string | No | — | Pins the LiteLLM `team_id`. An existing team with this id is adopted instead of duplicated; otherwise the team is created with it. See [Adopting Existing Teams](#adopting-existing-teams) |
| `teamAlias` | string | Yes | — | Human-readable team name |
| `models` | []string | No | — | Models this team can access |
| `maxBudgetMonthly` | *float64 | No | — | Maximum monthly budget in USD |
| `budgetDuration` | string | No | — | Budget reset period (e.g., `30d`, `7d`) |
| `tpmLimit` | *int | No | — | Tokens per minute limit |
| `rpmLimit` | *int | No | — | Requests per minute limit |
| `teamMemberRpmLimit` | *int | No | — | Per-member RPM limit |
| `teamMemberTpmLimit` | *int | No | — | Per-member TPM limit |
| `teamMemberBudget` | *float64 | No | — | Per-member max budget in USD (distinct from the team-wide `maxBudgetMonthly`); resets on `budgetDuration` |
| `metadata` | map[string]string | No | — | Custom metadata |
| `blocked` | *bool | No | — | Disable all requests from this team without deleting it |
| `softBudget` | *float64 | No | — | Alert threshold in USD below `maxBudgetMonthly` (does not block) |
| `modelRpmLimit` | map[string]int | No | — | Per-model requests-per-minute caps (model name → RPM) |
| `modelTpmLimit` | map[string]int | No | — | Per-model tokens-per-minute caps (model name → TPM) |
| `objectPermission` | *ObjectPermission | No | — | Grant access to MCP servers, vector stores, agents, access groups |
| `tags` | []string | No | — | Tags for tag-based routing (keys inherit these tags) |
| `maxParallelRequests` | *int | No | — | Maximum concurrent requests for this team |
| `guardrails` | []string | No | — | Names of [LiteLLMGuardrail](/reference/litellmguardrail) CRs this team opts into. Each entry must match `spec.guardrailName` on a guardrail bound to the same instance (enterprise) |
| `logging` | *TeamLoggingSpec | No | — | Per-team logging configuration (enterprise). See below |
| `memberManagement` | string | No | `mixed` | `crd`, `sso`, or `mixed` |
| `members` | []TeamMember | No | — | Team members list |

### `logging`

Per-team logging configuration (enterprise). Enables routing logs to team-specific provider instances (e.g., separate Langfuse projects) and GDPR-compliant logging disable.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `disabled` | bool | `false` | Disable all logging for this team (GDPR compliance). When true, no request/response data is logged |
| `callbacks` | []TeamCallback | — | Team-specific logging callbacks |

**TeamCallback:**

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | — | Callback provider: `langfuse`, `gcs_bucket`, `langsmith`, `arize` (required) |
| `type` | string | `success_and_failure` | When to invoke: `success`, `failure`, `success_and_failure` |
| `credentialsSecretRef` | SecretRef | — | Reference to a Secret containing provider credentials (required) |
| `config` | map[string]string | — | Additional provider-specific configuration merged with Secret data |

**Credentials Secret keys by provider:**

| Provider | Expected Secret Keys |
| --- | --- |
| `langfuse` | `langfuse_public_key`, `langfuse_secret`, `langfuse_host` (optional) |
| `gcs_bucket` | `gcs_bucket_name`, `gcs_path_service_account` (optional) |
| `langsmith` | `langsmith_api_key`, `langsmith_project` |
| `arize` | `arize_api_key`, `arize_space_key` |

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
| `loggingSynced` | bool | Whether team logging callbacks are synced |
| `loggingDisabled` | bool | Whether logging is disabled for this team (GDPR) |
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

## Adopting Existing Teams

LiteLLM does not treat `team_alias` as unique, so a `LiteLLMTeam` for a team that already exists (made in the Admin UI, or before the operator was installed) would otherwise create a second team with the same alias. Set `teamId` to the existing team's id to bring it under the CR instead:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: engineering
spec:
  instanceRef:
    name: my-gateway
  teamId: 3f2b6c1e-0000-4000-8000-000000000000  # id of the existing team
  teamAlias: engineering
  models: [gpt-4o]
```

The first time it reconciles the CR, the operator looks up `teamId`:

| `/team/info` result | What happens |
| --- | --- |
| Team found | Adopted: the id is recorded, the CR is annotated `litellm.palena.ai/adopted: "true"`, and the team is updated to match the spec |
| 404 | The team is created with that `team_id` |
| Any other error | Nothing is created; the reconcile is retried |

An adopted team is **never deleted** from LiteLLM when its CR is deleted. The operator did not create it, and the keys and spend history attached to it cannot be recreated. Teams the operator created are deleted on CR deletion as before.

Adoption applies the whole spec to the existing team, including membership. With the default `memberManagement: mixed`, `members` are only added. With `memberManagement: crd`, members of the existing team who are not listed in `members` are **removed**, so list them before adopting a team in that mode.

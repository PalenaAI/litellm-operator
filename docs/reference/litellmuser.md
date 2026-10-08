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
| `initialPasswordSecretRef` | *SecretKeyRef | No | — | Secret key holding an initial Admin UI login password (see [Initial Password](#initial-password)) |

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
| `initialPasswordDigest` | string | bcrypt digest of the last password applied from `initialPasswordSecretRef` (never the plaintext) |
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

## Initial Password

`initialPasswordSecretRef` gives a user a password for logging in to the Admin UI, for environments without SSO. The value is read from a Secret in the CR's namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alice-initial-password
stringData:
  password: "Change-Me-On-First-Login!"
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMUser
metadata:
  name: alice
spec:
  instanceRef:
    name: my-gateway
  userId: alice@example.com
  userEmail: alice@example.com
  initialPasswordSecretRef:
    name: alice-initial-password
    key: password
```

It is an *initial* password: the user is expected to change it after logging in, and the operator does not undo that.

- The password is applied once, right after the user is created (or adopted), via `POST /user/update`. LiteLLM rejects a password on `/user/new`.
- After that it is applied **only when the Secret's value changes**. Resyncs, spec edits and operator restarts never re-send it. To reset a user's password, put a new value in the Secret.
- The operator records a bcrypt digest of the applied value in `status.initialPasswordDigest` to detect changes. The plaintext is never stored on the CR, logged or put in events.
- Removing the field leaves the user's current password in LiteLLM untouched. Adding it back applies the Secret's value again.
- Recent LiteLLM versions mark an admin-set password as requiring a reset at first login, and validate it against the proxy's password policy and breached-password check.

Progress is reported on the `InitialPasswordApplied` condition, separate from `Synced`, so a password problem never stops the rest of the user from syncing:

| Reason | Meaning |
| --- | --- |
| `Applied` | The Secret's current value is the applied password |
| `SecretNotFound` / `SecretKeyMissing` | The Secret or key does not exist or is empty; applied as soon as it appears |
| `PasswordRejected` | LiteLLM rejected the value (e.g. password policy); not retried until the Secret changes |
| `ApplyFailed` / `SecretFetchFailed` | Transient failure; retried after 30s |

## Adopting Existing Users

If `userId` already exists in LiteLLM (created by SSO, the Admin UI, or before the operator was installed), `/user/new` returns 409. The operator then confirms through `/user/info` that the user exists under `userId` and adopts it: the id is recorded, the CR is annotated `litellm.palena.ai/adopted: "true"`, and the user is updated to match the spec.

LiteLLM also returns 409 when `userEmail` belongs to a *different* user. In that case `/user/info` finds no user under `userId`, so nothing is adopted and the CR reports the conflict.

An adopted user is **never deleted** from LiteLLM when its CR is deleted, because the operator did not create it. Users the operator created are deleted on CR deletion as before.

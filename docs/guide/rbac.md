# RBAC (Role-Based Access Control)

The operator supports LiteLLM's role-based access control system via `spec.rbac` on `LiteLLMInstance`. RBAC lets you control which API routes are accessible, who can generate keys, and what each role is allowed to do.

## Quick Start

Enable RBAC enforcement and restrict admin routes:

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: litellm-db
        key: DATABASE_URL
  rbac:
    enabled: true
    adminOnlyRoutes:
      - /model/new
      - /model/delete
      - /organization/new
```

When `enabled: true`, the operator writes `enforce_rbac: true` to `general_settings` in the generated `proxy_server_config.yaml`.

## Route Restrictions

### Admin-Only Routes

Restrict specific routes so only `proxy_admin` users can access them:

```yaml
rbac:
  enabled: true
  adminOnlyRoutes:
    - /model/new
    - /model/delete
    - /model/update
    - /organization/new
    - /organization/delete
```

### Allowed Routes

Define an allowlist of routes accessible to all authenticated users. Routes not in this list are blocked:

```yaml
rbac:
  enabled: true
  allowedRoutes:
    - /chat/completions
    - /embeddings
    - /key/info
    - /user/info
```

::: tip
`adminOnlyRoutes` and `allowedRoutes` can be used together. `adminOnlyRoutes` restricts routes to admins, while `allowedRoutes` defines what non-admin users can access.
:::

## Key Generation Control

### Force Team-Based Keys

Prevent users from creating personal keys — all keys must be associated with a team:

```yaml
rbac:
  enabled: true
  defaultTeamDisabled: true
```

### Restrict Key Generation Roles (Enterprise)

Control which roles can generate team keys and personal keys:

```yaml
rbac:
  enabled: true
  keyGeneration:
    teamKeyGeneration:
      allowedTeamMemberRoles: ["admin"]
    personalKeyGeneration:
      allowedUserRoles: ["proxy_admin"]
```

In this example:
- Only team **admins** can generate keys for their team (not regular team members)
- Only **proxy_admin** users can generate personal keys

## Per-Role Permissions (Enterprise)

Define which routes and models each role can access:

```yaml
rbac:
  enabled: true
  rolePermissions:
    internal_user:
      routes:
        - /key/generate
        - /key/delete
        - /key/info
      models:
        - gpt-4
        - claude-3-haiku
    internal_user_viewer:
      routes:
        - /key/info
        - /user/info
      models:
        - gpt-4-mini
```

## Full Example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: litellm-db
        key: DATABASE_URL
  rbac:
    enabled: true
    adminOnlyRoutes:
      - /model/new
      - /model/delete
      - /organization/new
    allowedRoutes:
      - /chat/completions
      - /embeddings
      - /key/info
      - /user/info
    defaultTeamDisabled: true
    keyGeneration:
      teamKeyGeneration:
        allowedTeamMemberRoles: ["admin"]
      personalKeyGeneration:
        allowedUserRoles: ["proxy_admin"]
    rolePermissions:
      internal_user:
        routes:
          - /key/generate
          - /key/delete
          - /key/info
        models:
          - gpt-4
          - claude-3-haiku
```

## Enterprise Features

Some RBAC features require a [LiteLLM Enterprise license](/guide/enterprise-license):

| Feature | Open Source | Enterprise |
| --- | --- | --- |
| `enforce_rbac` | Yes | Yes |
| `adminOnlyRoutes` | Yes | Yes |
| `allowedRoutes` | Yes | Yes |
| `defaultTeamDisabled` | Yes | Yes |
| `keyGeneration` | No | Yes |
| `rolePermissions` | No | Yes |

When enterprise features (`keyGeneration` or `rolePermissions`) are configured without a license Secret, the operator sets an `EnterpriseFeaturesConfigured` warning condition on the instance.

## Generated Config

The operator writes RBAC settings to `general_settings` in the generated `proxy_server_config.yaml`:

```yaml
general_settings:
  enforce_rbac: true
  admin_only_routes:
    - /model/new
    - /model/delete
  allowed_routes:
    - /chat/completions
    - /embeddings
  default_team_disabled: true
  key_generation_settings:
    team_key_generation:
      allowed_team_member_roles: ["admin"]
    personal_key_generation:
      allowed_user_roles: ["proxy_admin"]
  role_permissions:
    internal_user:
      routes:
        - /key/generate
        - /key/delete
      models:
        - gpt-4
        - claude-3-haiku
```

## Interaction with Other Features

- **User roles** — `LiteLLMUser.spec.userRole` assigns global roles (`proxy_admin`, `internal_user`, etc.) to users. RBAC controls what those roles can do.
- **Organizations** — `org_admin` role permissions work with RBAC enforcement. Implement organizations first for full RBAC support.
- **JWT Auth** — JWT claim-to-role mapping (`spec.jwtAuth.userRoleJwtField`) works with RBAC to enforce permissions for token-authenticated users.
- **SSO** — `spec.sso.defaultUserParams.userRole` sets the role for auto-created SSO users, which RBAC then governs.

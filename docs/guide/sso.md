# SSO Setup

The operator supports configuring SSO authentication on LiteLLM instances. SSO configuration is translated into environment variables and config entries on the Deployment.

::: tip Looking for API-level authentication?
SSO handles Admin UI login via browser flows. For API-level authentication (services calling the proxy with JWT tokens), see [JWT / OAuth2 Auth](/guide/jwt-oauth2-auth).
:::

## Supported Providers

| Provider | `spec.sso.provider` | Notes |
| --- | --- | --- |
| Azure Entra ID | `azure-entra` | Uses Microsoft-specific env vars |
| Okta | `okta` | Uses generic OIDC endpoints |
| Google | `google` | Uses Google-specific env vars |
| Generic OIDC | `generic-oidc` | Any OIDC-compliant provider |

## Configuration

### 1. Create SSO Client Credentials Secret

```bash
kubectl create secret generic sso-credentials \
  --from-literal=client-id='your-client-id' \
  --from-literal=client-secret='your-client-secret'
```

### 2. Configure SSO on the Instance

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  # ... other fields ...
  sso:
    enabled: true
    provider: azure-entra
    tenantId: "00000000-0000-0000-0000-000000000000"  # Azure directory GUID
    clientId:
      name: sso-credentials
      key: client-id
    clientSecret:
      name: sso-credentials
      key: client-secret
    teamIdsJwtField: groups
    defaultUserParams:
      userRole: internal_user
      maxBudget: 100
      budgetDuration: "30d"
      models:
        - gpt-4o
    defaultTeamParams:
      maxBudget: 500
      budgetDuration: "30d"
      models:
        - gpt-4o
```

## Provider-Specific Configuration

### Azure Entra ID

```yaml
sso:
  enabled: true
  provider: azure-entra
  tenantId: "00000000-0000-0000-0000-000000000000"
  clientId:
    name: azure-sso
    key: client-id
  clientSecret:
    name: azure-sso
    key: client-secret
  teamIdsJwtField: groups
```

The operator sets `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`, and (when `tenantId` is provided) `MICROSOFT_TENANT` on the Deployment.

::: tip
`tenantId` is a plain string (the Azure directory/tenant GUID), not a Secret reference. Omit it only if you're intentionally relying on the `common` multi-tenant endpoint.
:::

### Okta

```yaml
sso:
  enabled: true
  provider: okta
  clientId:
    name: okta-sso
    key: client-id
  clientSecret:
    name: okta-sso
    key: client-secret
  authorizationEndpoint: https://your-org.okta.com/oauth2/default/v1/authorize
  tokenEndpoint: https://your-org.okta.com/oauth2/default/v1/token
  userinfoEndpoint: https://your-org.okta.com/oauth2/default/v1/userinfo
```

The operator sets `GENERIC_CLIENT_ID`, `GENERIC_CLIENT_SECRET`, and the three `GENERIC_*_ENDPOINT` env vars on the Deployment. Configure scopes in your Okta application — the `scopes` field is only wired for `generic-oidc` (see below).

### Google

```yaml
sso:
  enabled: true
  provider: google
  clientId:
    name: google-sso
    key: client-id
  clientSecret:
    name: google-sso
    key: client-secret
```

The operator sets `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` on the Deployment.

### Generic OIDC

```yaml
sso:
  enabled: true
  provider: generic-oidc
  clientId:
    name: oidc-sso
    key: client-id
  clientSecret:
    name: oidc-sso
    key: client-secret
  authorizationEndpoint: https://idp.example.com/authorize
  tokenEndpoint: https://idp.example.com/token
  userinfoEndpoint: https://idp.example.com/userinfo
  scopes:
    - openid
    - profile
    - email
    - groups
```

## User Attribute Mappings

For providers with non-standard claims, configure attribute mappings:

```yaml
sso:
  userAttributeMappings:
    userId: sub
    email: email
    displayName: name
    firstName: given_name
    lastName: family_name
    role: custom_role_claim
```

## Default Parameters for SSO Users

When SSO users log in for the first time, LiteLLM auto-creates their account. Control the defaults:

```yaml
sso:
  defaultUserParams:
    userRole: internal_user
    maxBudget: 100
    budgetDuration: "30d"
    models:
      - gpt-4o
    teams:
      - teamId: "default-team-id"
        role: user                   # mapped to user_role
        maxBudgetInTeam: 25          # optional per-team cap
  defaultTeamParams:
    maxBudget: 500
    models:
      - gpt-4o
    tpmLimit: 100000
    rpmLimit: 1000
```

These are written to `litellm_settings.default_internal_user_params` and `litellm_settings.default_team_params` in the ConfigMap. Entries under `teams` become `{team_id, user_role, max_budget_in_team}` — new SSO users are auto-enrolled in the listed teams on first login.

## Proxy Base URL

SSO providers redirect the browser back to LiteLLM after authentication. LiteLLM builds that callback URL from the `PROXY_BASE_URL` env var, which the operator derives as follows:

1. If `spec.ingress.enabled` is set with a `host` → `http(s)://<host>`
2. Otherwise → `http://<instance-name>.<namespace>.svc:<port>` (in-cluster Service DNS)

If you expose the gateway through an OpenShift Route, a Gateway API HTTPRoute, or an external load balancer (rather than a plain Ingress), the fallback kicks in and you'll see redirects like `http://my-gateway.my-ns.svc:4000/ui/login?redirect_to=…` in the browser. Override it explicitly via `extraEnvVars`:

```yaml
spec:
  extraEnvVars:
    - name: PROXY_BASE_URL
      value: https://gateway.example.com
```

Values in `extraEnvVars` replace operator-set env vars of the same name, so there's no duplicate env entry in the Pod spec.

## Logout Redirect

Setting `sso.logoutUrl` makes the Admin UI redirect to an end-session endpoint on logout, so the user is signed out of both LiteLLM and the IdP in one click. Without it, logout only clears the local session.

```yaml
sso:
  logoutUrl: https://auth.example.com/application/o/litellm/end-session/
```

The operator injects this as the `PROXY_LOGOUT_URL` env var on the Deployment.

## Custom SSO Handler

LiteLLM can invoke a user-defined Python function after a successful SSO login — useful for custom team assignment, attribute enrichment, or auditing. The operator supports two modes:

### Option 1 — Module baked into a custom image

Use when you ship your own LiteLLM image with the handler code already on disk.

```yaml
sso:
  customSsoHandler:
    module: "my_package.my_handler"
```

### Option 2 — Code loaded from a ConfigMap

Use when you prefer GitOps management of the handler source. The operator mounts the ConfigMap at `/app/custom_sso_handlers/` inside the pod and writes `general_settings.custom_sso` to `custom_sso_handlers.<stem>.<functionName>`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-sso-handler
  namespace: llm
data:
  handler.py: |
    async def handle_sso(userIDPInfo):
        # run your own code here — add claims, sync groups, etc.
        return userIDPInfo
---
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
  namespace: llm
spec:
  sso:
    customSsoHandler:
      configMapRef:
        name: my-sso-handler
        fileName: handler.py       # must end in .py; becomes the module stem
        functionName: handle_sso   # Python function inside the file
```

::: warning Operational notes

- ConfigMaps are capped at ~1 MiB. For larger handlers or anything with third-party dependencies, bake a custom image instead.
- The handler runs inside the LiteLLM pod with the same privileges as the gateway — treat the ConfigMap as a privileged resource.
- Changing the ConfigMap contents does **not** automatically restart pods. Trigger a rollout (`kubectl rollout restart deployment/<instance>`) after editing the handler.
- Only modules under `custom_sso_handlers.*` are importable via this mechanism — nested subpackages are not supported. One file per ConfigMap.

:::

::: warning Known limitations

- The `sso.scopes` field only takes effect when `provider: generic-oidc` (emitted as `GENERIC_SCOPE`). For `azure-entra`, `okta`, and `google`, configure scopes in the IdP application itself.

:::

## SSO with Team Member Management

When using SSO, set `memberManagement: sso` or `memberManagement: mixed` on your `LiteLLMTeam` CRDs to prevent the operator from interfering with SSO-provisioned memberships. See [Team Member Management](/guide/team-members) for details.

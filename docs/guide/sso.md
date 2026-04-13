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
    clientId:
      name: sso-credentials
      key: client-id
    clientSecret:
      name: sso-credentials
      key: client-secret
    scopes:
      - openid
      - profile
      - email
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
  clientId:
    name: azure-sso
    key: client-id
  clientSecret:
    name: azure-sso
    key: client-secret
  teamIdsJwtField: groups
```

The operator sets `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`, and `MICROSOFT_TENANT` on the Deployment.

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
        role: user
  defaultTeamParams:
    maxBudget: 500
    models:
      - gpt-4o
    tpmLimit: 100000
    rpmLimit: 1000
```

These are written to `litellm_settings.default_internal_user_params` and `litellm_settings.default_team_params` in the ConfigMap.

## SSO with Team Member Management

When using SSO, set `memberManagement: sso` or `memberManagement: mixed` on your `LiteLLMTeam` CRDs to prevent the operator from interfering with SSO-provisioned memberships. See [Team Member Management](/guide/team-members) for details.

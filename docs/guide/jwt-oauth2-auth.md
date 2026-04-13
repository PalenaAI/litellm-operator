# JWT / OAuth2 Authentication

The operator supports configuring JWT and OAuth2 authentication on LiteLLM instances. These are **enterprise** features that enable API-level authentication — applications can call the proxy using tokens from their identity provider without needing LiteLLM virtual keys.

::: tip SSO vs JWT/OAuth2 Auth
**SSO** (`spec.sso`) handles Admin UI login — users authenticate via a browser flow to access the LiteLLM dashboard.

**JWT auth** (`spec.jwtAuth`) and **OAuth2 auth** (`spec.oauth2Auth`) handle API-level authentication — services and applications authenticate via JWT tokens in API requests.

Both can be active simultaneously.
:::

## Prerequisites

- A LiteLLM Enterprise license (see [Enterprise License](/guide/enterprise-license))
- An identity provider (IdP) that issues JWTs with a JWKS endpoint (Azure AD, Okta, Auth0, Keycloak, etc.)

## JWT Authentication

JWT auth validates tokens from your IdP and maps JWT claims to LiteLLM concepts (users, teams, organizations, roles). LiteLLM automatically discovers the IdP's JWKS endpoint from the JWT `iss` (issuer) claim.

### Basic Setup

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  # ... database, masterKey, etc. ...
  jwtAuth:
    enabled: true
    userIdJwtField: "sub"
    userEmailJwtField: "email"
    teamIdsJwtField: "groups"
```

This writes the following to `proxy_server_config.yaml`:

```yaml
general_settings:
  enable_jwt_auth: true
  litellm_jwtauth:
    user_id_jwt_field: "sub"
    user_email_jwt_field: "email"
    team_ids_jwt_field: "groups"
```

### Full Configuration

```yaml
jwtAuth:
  enabled: true

  # Admin access — JWTs with this scope get admin routes
  adminJwtScope: "litellm_proxy_admin"
  adminAllowedRoutes:
    - openai_routes
    - info_routes

  # Claim field mappings
  teamIdJwtField: "client_id"       # single team ID
  teamIdsJwtField: "groups"         # array of team IDs
  orgIdJwtField: "org_id"           # organization ID
  userIdJwtField: "sub"             # user ID
  userEmailJwtField: "email"        # user email
  userRoleJwtField: "role"          # user role
  endUserIdJwtField: "end_user_id"  # end-user ID (for customer tracking)

  # Public key caching
  publicKeyTtl: 600  # cache JWKS keys for 10 minutes

  # Scope-to-model access control
  scopeModelMappings:
    "scope:gpt":
      - gpt-4
      - gpt-4-mini
    "scope:claude":
      - claude-3-opus
      - claude-3-sonnet
    "scope:all":
      - "*"
```

### Scope-to-Model Mappings

`scopeModelMappings` restricts which models a JWT holder can access based on their token scopes. A token with `scope: "scope:gpt"` can only use `gpt-4` and `gpt-4-mini`. Use `"*"` to grant access to all models.

## OAuth2 Machine-to-Machine Auth

OAuth2 auth enables service-to-service authentication by mapping JWT fields to LiteLLM attributes. This is designed for machine clients (CI/CD pipelines, backend services, automated tools) that authenticate via client credentials flow.

### Setup

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: my-gateway
spec:
  # ... database, masterKey, etc. ...
  oauth2Auth:
    enabled: true
    configMappings:
      - name: clientId
        jwtField: "client_id"
        litellmAttribute: "team_id"
      - name: userId
        jwtField: "sub"
        litellmAttribute: "user_id"
```

This writes the following to `proxy_server_config.yaml`:

```yaml
general_settings:
  enable_oauth2_auth: true
  oauth2_config_mappings:
    clientId:
      jwt_field: "client_id"
      litellm_attribute: "team_id"
    userId:
      jwt_field: "sub"
      litellm_attribute: "user_id"
```

Each mapping tells LiteLLM: "read the `jwt_field` from the token and treat it as the `litellm_attribute`." For example, a token with `client_id: "service-a"` is treated as belonging to team `service-a`.

## Using Both Together

JWT and OAuth2 auth can be enabled simultaneously. A common pattern:

- **JWT auth** for human users (browser-based apps sending tokens from the IdP)
- **OAuth2 auth** for machine clients (services using client credentials)

```yaml
spec:
  jwtAuth:
    enabled: true
    userIdJwtField: "sub"
    teamIdsJwtField: "groups"
    adminJwtScope: "litellm_admin"

  oauth2Auth:
    enabled: true
    configMappings:
      - name: serviceTeam
        jwtField: "client_id"
        litellmAttribute: "team_id"
```

## Enterprise License Warning

When JWT or OAuth2 auth is configured without a license Secret, the operator:

1. Generates the config normally (LiteLLM enforces the license at runtime)
2. Sets an `EnterpriseFeaturesConfigured` warning condition on the instance status
3. Emits a Kubernetes warning event

```bash
kubectl get litellminstance my-gateway -o jsonpath='{.status.conditions}' | jq '.[] | select(.type == "EnterpriseFeaturesConfigured")'
```

```json
{
  "type": "EnterpriseFeaturesConfigured",
  "status": "True",
  "reason": "EnterpriseFeaturesConfigured",
  "message": "JWT auth, OAuth2 auth requires a LiteLLM Enterprise license"
}
```

To resolve, create a license Secret. See [Enterprise License](/guide/enterprise-license).

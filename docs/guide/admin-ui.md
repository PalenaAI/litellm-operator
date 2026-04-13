# Admin UI

LiteLLM ships a built-in Admin UI at `<proxy_base_url>/ui` for key management, spend tracking, model addition, and user self-service. The operator exposes configuration for this UI via `spec.adminUI` on `LiteLLMInstance`.

## Disable the Admin UI

```yaml
spec:
  adminUI:
    disabled: true
```

Sets the `DISABLE_ADMIN_UI=True` environment variable. The `/ui` endpoint will return 404.

## Restrict Access to Admins

```yaml
spec:
  adminUI:
    adminOnly: true
```

Writes `ui_access_mode: "admin_only"` to `general_settings`. Only users with `proxy_admin` or `proxy_admin_viewer` roles can access the UI.

## Enable Dynamic Model Management

```yaml
spec:
  adminUI:
    storeModelInDB: true
```

Writes `store_model_in_db: true` to `general_settings`. Models added through the UI are persisted in the database and shared across all proxy replicas without a restart.

::: tip
This is recommended for production multi-replica deployments.
:::

## Disable Personal Key Creation

```yaml
spec:
  adminUI:
    defaultTeamDisabled: true
```

Writes `default_team_disabled: true` to `general_settings`. Users cannot create personal API keys — they must belong to a team. This pairs well with the `LiteLLMTeam` CRD for enforcing team-scoped spend tracking.

## Customize Docs and Redirect URLs

```yaml
spec:
  adminUI:
    apiDocBaseURL: "https://api.example.com"
    docsURL: "/docs"
    rootRedirectURL: "/ui"
```

| Field | Env Var | Description |
| --- | --- | --- |
| `apiDocBaseURL` | `LITELLM_UI_API_DOC_BASE_URL` | Override the base URL for API docs shown in the UI |
| `docsURL` | `DOCS_URL` | Custom path for the docs endpoint |
| `rootRedirectURL` | `ROOT_REDIRECT_URL` | Where to redirect when root `/` is accessed |

## Custom Branding

```yaml
spec:
  adminUI:
    logoURL: "https://example.com/logo.png"
    emailLogoURL: "https://example.com/email-logo.png"
    emailSupportContact: "support@example.com"
```

| Field | Env Var | Description |
| --- | --- | --- |
| `logoURL` | `UI_LOGO_PATH` | URL to a hosted logo image displayed in the Admin UI |
| `emailLogoURL` | `EMAIL_LOGO_URL` | URL to a logo image included in email notifications |
| `emailSupportContact` | `EMAIL_SUPPORT_CONTACT` | Support email address shown in notifications |

::: tip
LiteLLM recommends using a hosted image URL for the logo — it's easier to set up than a local file path. Users can also upload a logo at runtime through the Admin UI settings page.
:::

## Custom Color Theme

Create a ConfigMap with your brand colors using the [Tremor color palette](https://www.tremor.so/docs/layout/color-palette#default-colors):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: litellm-colors
data:
  enterprise_colors.json: |
    {
      "brand": {
        "DEFAULT": "indigo",
        "faint": "indigo",
        "muted": "indigo",
        "subtle": "indigo",
        "emphasis": "indigo",
        "inverted": "indigo"
      }
    }
```

Then reference it in the instance:

```yaml
spec:
  adminUI:
    colorThemeConfigMapRef:
      name: litellm-colors
```

The operator mounts `enterprise_colors.json` from the ConfigMap into the container at `/app/enterprise/enterprise_ui/enterprise_colors.json` using a `subPath` mount, so existing files in that directory are preserved.

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
  adminUI:
    adminOnly: true
    storeModelInDB: true
    defaultTeamDisabled: true
    logoURL: "https://example.com/logo.png"
    emailSupportContact: "support@example.com"
    colorThemeConfigMapRef:
      name: litellm-colors
```

## Notes

- All fields are optional. When `adminUI` is omitted, LiteLLM uses its defaults (UI enabled, all users can access, models in config file, personal keys allowed).
- `LITELLM_UI_PATH` and `LITELLM_ASSETS_PATH` are not exposed — they are internal to the container image and can be overridden via `extraEnvVars` if needed.

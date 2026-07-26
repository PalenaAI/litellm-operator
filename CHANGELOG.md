# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`LiteLLMInstance.spec.observability.serviceMonitor` now supports endpoint authorization** ([#20](https://github.com/PalenaAI/litellm-operator/issues/20)) — when the proxy is protected with a master key, the `/metrics` endpoint rejects unauthenticated scrapes ("Malformed API Key passed in. Ensure Key has a Bearer prefix."), so the operator-managed `ServiceMonitor` could not scrape an authenticated deployment. Added `serviceMonitor.authorization` (mirroring the Prometheus Operator's native `endpoints[].authorization` block) with `type` (default `Bearer`) and `credentials` (a Secret name/key). **Credentials default to the instance's master key Secret** when omitted — the referenced Secret for a user-supplied master key, or the auto-generated `<instance>-master-key` Secret otherwise — so the common case needs only `authorization: {}`. Also added `serviceMonitor.path` to override the scrape path (default `/metrics`).

## [0.20.0] - 2026-07-21

### Added

- **Pod-level faults now surface on `LiteLLMInstance.status`** — when the gateway is unready, the cause is visible from the CR instead of requiring a dig through pod logs. Adds a **`PodsHealthy`** condition (independent of `Ready`, which keeps meaning "at least one replica is serving") and `.status.unhealthyPods[]` with `name`, `phase`, `reason`, a truncated `message`, and `restartCount`. Recognizes `CrashLoopBackOff` (reporting the *previous* container termination — e.g. `last exit code 3` — rather than the useless back-off timing), `ImagePullBackOff`, `OOMKilled`, `CreateContainerConfigError` (a missing Secret/ConfigMap key), and `Unschedulable`. Kept cheap and quiet: pods are listed only while not ready (no extra reads when healthy, no pod watch), the list is capped at 3 with 512-char messages to bound the object, and a Warning event fires only when the cause *changes* so a crash loop can't spam the event stream. Requires new pod `get;list;watch` RBAC.

### Security

- **Bumped Go to 1.25.12** (go.mod and the `golang:1.25` builder image digest) to clear a HIGH standard-library advisory flagged by the image scanner — CVE-2026-39822 (fixed in Go 1.25.12). No code changes; the operator binary is rebuilt against the patched stdlib.

## [0.19.1] - 2026-07-14

### Fixed

- **Enterprise-gated resources gave up permanently instead of retrying, leaving them stuck until manually recreated.** When the proxy rejected a `LiteLLMVirtualKey`, `LiteLLMModel`, `LiteLLMOrganization`, `LiteLLMUser`, `LiteLLMCustomer`, or `LiteLLMTeam` with a `403` "enterprise" error (no LiteLLM Enterprise license installed yet), the controller set an `EnterpriseLicenseRequired` condition and returned **without a requeue**. Because the license is typically installed *after* the resource is first requested (e.g. a platform activates SSO + virtual keys once the license Secret lands), and re-applying an unchanged spec produces no reconcile event, the resource never synced on its own — the only fix was deleting and recreating the CR. Most visibly, an auto-wired `LiteLLMVirtualKey` never minted its key Secret, so a consumer waiting on it (e.g. a ChatUI) stayed stuck. All six reconcilers now **requeue** (every 2m) on the enterprise gate, so the resource syncs itself once the license is present.

## [0.19.0] - 2026-07-10

### Fixed

- **`LiteLLMCredential` reconcile crash-looped the proxy with `UniqueViolationError` on `credential_name`.** The controller decided create-vs-update from `status.configured`, a subresource that can be lost independently of the sync-hash annotation. When it was `false` but the credential already existed in LiteLLM's DB, the operator re-`POST`ed `/credentials` → Prisma unique-constraint violation (returned as HTTP 500, which the old 400-only fallback missed) → the proxy logged the error on every reconcile. Now the decision keys off the sync-hash annotation (the reliable "already pushed" marker), and a create that conflicts (400/409, or a 500 carrying the unique-constraint message) falls back to an idempotent `PATCH`. Existing stuck credentials self-heal on the next reconcile.

### Added

- **JWT token-validation env vars on `LiteLLMInstance.spec.jwtAuth`**: `publicKeyUrl`, `issuer`, `audience` → `JWT_PUBLIC_KEY_URL` / `JWT_ISSUER` / `JWT_AUDIENCE` on the proxy Deployment. LiteLLM reads these from the environment (not from `litellm_jwtauth`), so without `publicKeyUrl` the proxy can't fetch signing keys and JWT auth fails even when `litellm_jwtauth` is fully configured.

## [0.18.1] - 2026-07-10

### Fixed

- **`general_settings.role_permissions` crashed the proxy on startup.** `spec.rbac.rolePermissions` was rendered as a map keyed by role name, but LiteLLM expects a **list** of objects each carrying its own `role` field (it iterates the list and calls `RoleBasedPermissions(**item)`). Iterating a map yielded string keys → `TypeError: argument after ** must be a mapping, not str` → CrashLoopBackOff (exit 3). It now renders a sorted list of `{role, models, routes}`. Affects every release since the field was introduced; anyone using `spec.rbac.rolePermissions` should upgrade.

## [0.18.0] - 2026-07-10

### Added

- **Full `litellm_jwtauth` surface on `LiteLLMInstance.spec.jwtAuth`** (enterprise) — the operator now exposes essentially every JWT-auth knob LiteLLM supports:
  - **User/team/org:** `userIdUpsert`, `teamIdUpsert`, `teamIdDefault`, `teamAllowedRoutes`, `teamAliasJwtField`, `orgAliasJwtField`, `teamClaimFallback`, `userAllowedEmailDomain`, `rolesJwtField`.
  - **Enforcement:** `enforceTeamBasedModelAccess`, `enforceScopeBasedAccess`, `syncUserRoleAndTeams`.
  - **Structured mappings:** `scopeMappings` (`{scope, models, routes}`), `roleMappings` (`{role, internalRole}`), `jwtLitellmRoleMap` (`{jwtRole, litellmRole}`), and `routingOverrides` (`{iss, clientId, scope, aud, path}`).
  - **OIDC UserInfo:** `oidcUserinfoEnabled` / `oidcUserinfoEndpoint` / `oidcUserinfoCacheTtl`.
  - **Virtual-key mapping:** `virtualKeyClaimField` / `virtualKeyMappingCacheTtl`.
  - **Custom validation:** `customValidate` (dotted path to a handler baked into the proxy image).

### Fixed

- **`spec.jwtAuth.scopeModelMappings` was a no-op and now works.** It rendered `general_settings.litellm_jwtauth.scope_model_mappings`, a key LiteLLM does not read. It's now rendered into `scope_mappings` (the list shape LiteLLM actually consumes), merged with the new structured `scopeMappings`. Existing `scopeModelMappings` values start taking effect on upgrade.

## [0.17.0] - 2026-07-09

### Added

- **JWT role-based model access (RBAC) on `LiteLLMInstance.spec.jwtAuth`** (enterprise). Four new fields wire a JWT roles claim into LiteLLM's `role_permissions`: `userRolesJwtField` (`user_roles_jwt_field` — the claim holding a *list* of roles, distinct from the existing single-value `userRoleJwtField`), `userAllowedRoles` (`user_allowed_roles` — roles that map to `internal_user`), `enforceRbac` (`litellm_jwtauth.enforce_rbac` — the JWT-level RBAC toggle, distinct from the `general_settings`-level `spec.rbac.enforceRbac`), and `objectIdJwtField` (`object_id_jwt_field`). Combined with `spec.rbac.rolePermissions`, this reproduces the standard LiteLLM JWT-RBAC config (a JWT role → `internal_user` → per-role model allow-list) — previously the `role_permissions` target was expressible but the JWT plumbing to feed it was not.

## [0.16.2] - 2026-07-07

### Fixed

- **The operator no longer live-probes models when health checks are disabled — a real cost fix.** On every `LiteLLMModel` reconcile (~5 min) the operator called `GET /health` to populate `status.health`. When `background_health_checks` is off (the LiteLLM default), `GET /health` **live-probes every deployment** — a billable completion per model — so setting `spec.generalSettings.backgroundHealthChecks: false` disabled LiteLLM's background loop but the operator kept triggering paid probes. The operator now polls `/health` **only when `backgroundHealthChecks: true`** (when it returns cached results); otherwise it skips the probe and marks `status.health: unknown`. Leaving the field unset also means no probing.

### Added

- **`LiteLLMInstance.spec.generalSettings.healthCheckSkipDisabledModels`** → renders `health_check_skip_disabled_background_models`, so models with `modelInfo.healthCheck.disableBackgroundHealthCheck: true` are also excluded from the on-demand `GET /health` probe (not just the background loop).

## [0.16.1] - 2026-07-04

### Fixed

- **OLM CSV descriptors for the new `LiteLLMBudget` CRD.** The v0.16.0 bundle listed `LiteLLMBudget` as a bare stub (no `displayName`, `resources`, or `specDescriptors`), which failed the operator-sdk scorecard `olm-crds-have-resources` and `olm-spec-descriptors` tests. Added the complete owned-CRD entry to the CSV base.
- **CI lint cleanups** (pre-existing, unrelated to features): extracted the repeated `"ca.crt"` Secret key into a `caCrtKey` constant (`goconst`) and dropped the always-`"gw"` parameter from the `tlsInstance` test helper (`unparam`).
- **Hardened CI syft install** in `e2e.yml` and `release.yml`: download the pinned installer to a file before executing instead of piping `curl` into a shell (clears the Semgrep `gha-curl-pipe-shell` finding).

## [0.16.0] - 2026-07-04

### Changed

- **`LiteLLMGuardrail.spec.params` and `LiteLLMCredential.spec.params` now accept arbitrary JSON values** (`map[string]JSON` instead of `map[string]string`). This unblocks structured provider config that couldn't be expressed as strings — e.g. Presidio's `pii_entities_config: {CREDIT_CARD: MASK}`, per-entity numeric thresholds, or a Vertex AI service-account JSON object as a credential value. Existing string-valued params remain valid (a string is valid JSON), so this is backward-compatible at the YAML level.

### Added

- **New `LiteLLMBudget` CRD** — declares a reusable budget / rate-limit tier via the LiteLLM REST API (`/budget/new`, `/budget/update`, `/budget/info`, `/budget/delete`). Fields: `budgetId` (defaults to the object name), `maxBudget`, `softBudget`, `budgetDuration`, `tpmLimit`, `rpmLimit`, `maxParallelRequests`, `modelMaxBudget`. Other resources reference it by `budget_id` (e.g. `LiteLLMVirtualKey.spec.budgetId`), which previously had no CRD to create the tier — you had to make budgets out-of-band. Short name `lb`; finalizer-backed delete; `status.currentSpend` refreshed from `/budget/info`.
- **Cross-CRD access & budget controls** on `LiteLLMOrganization`, `LiteLLMTeam`, `LiteLLMUser`, and `LiteLLMVirtualKey` (shared, consistent field names):
  - `objectPermission` — grant access to MCP servers, vector stores, agents, and access groups (`object_permission`; the `LiteLLMCustomer` type was generalized into a shared `ObjectPermission`).
  - `softBudget` — alert threshold below the hard budget (`soft_budget`).
  - `modelRpmLimit` / `modelTpmLimit` — per-model rate-limit maps (`model_rpm_limit` / `model_tpm_limit`).
- **Incident-response `blocked` flag on `LiteLLMTeam`, `LiteLLMUser`, and `LiteLLMVirtualKey`** (`spec.blocked: true`) — disables all requests from a team/user/key without deleting it. Forwarded to `/team/{new,update}`, `/user/{new,update}`, and `/key/{generate,update}`. (`LiteLLMCustomer` already had this.)
- **`LiteLLMTeam.spec.teamMemberBudget`** — per-member max budget (`team_member_budget`), distinct from the team-wide `maxBudgetMonthly`; reset cadence follows the team's `budgetDuration`.
- **`LiteLLMModel` provider/routing additions**: `litellmParams.dropParams` (silently drop params a provider rejects, e.g. `temperature` on reasoning models) and first-class Vertex AI auth — `vertexProject`, `vertexLocation`, and `vertexCredentialsSecretRef` (reads the GCP service-account JSON from a Secret and sends it as `vertex_credentials`, never logged).
- **`LiteLLMInstance` router settings**: `routerSettings.streamTimeout`, `routerSettings.enablePreCallChecks` (context-window/region pre-filtering), `routerSettings.modelGroupAlias`; and the `routingStrategy` enum now includes `usage-based-routing-v2` and `cost-based-routing`.
- **`LiteLLMInstance` general settings**: alerting **delivery** — `generalSettings.alerting`, `alertingThreshold`, `alertToWebhookUrl` (previously only `alertTypes` was exposed, so alerts never fired); plus `backgroundHealthChecks`, `healthCheckInterval`, `healthCheckDetails`.
- **`LiteLLMInstance.spec.litellmSettings`** (new block) with `jsonLogs` for structured JSON logging — the home for future `litellm_settings` knobs.

### Fixed

- **Three CRD fields that were silently ignored are now rendered.** They existed in the API (and passed schema validation) but no controller/resource code ever emitted them, so setting them did nothing:
  - `spec.generalSettings.customKeyGenerate` → `general_settings.custom_key_generate`
  - `spec.routerSettings.retryAfter` → `router_settings.retry_after`
  - `spec.database.connectionPool.maxConnections` → `general_settings.database_connection_pool_limit`

### Added

- **`LiteLLMModel` now exposes LiteLLM's full per-model config surface.** Previously only `model`, auth, `rpm`/`tpm`/`timeout`/`streamTimeout`/`maxRetries` and three `modelInfo` fields (`maxTokens`, cost per token) were configurable — LiteLLM accepts far more.
  - **`spec.modelInfo.healthCheck`** — per-model health-check controls, including `disableBackgroundHealthCheck` to turn off background liveness probing for a single deployment (e.g. providers that bill/rate-limit probes, or models that reject the probe request shape). Also `timeoutSeconds`, `maxTokens` / `maxTokensReasoning` / `maxTokensNonReasoning`, `reasoningEffort`, `voice`, and `model` (probe target for wildcard routes). These are flattened onto `model_info` in the `/model/new` payload to match LiteLLM's wire format.
  - **`spec.litellmParams`** additions: `weight` and `order` (weighted / priority load-balancing across deployments in a model group), `maxInputTokens` (context-window-aware routing/fallbacks), default request params `temperature` / `topP` / `maxTokens` / `seed`, and provider knobs `organization`, `awsRegionName`, `extraHeaders`.
  - **`spec.modelInfo`** additions: `mode` (declare model type so the correct health check / routing runs), `baseModel` (required for accurate Azure cost tracking), `tier` and `regionName` (tier-/region-based routing), `accessGroups` and `supportedEnvironments` (access control / visibility), `useInPassThrough`, and cost fields `inputCostPerPixel`, `inputCostPerSecond`, `cacheReadInputTokenCost`, `cacheCreationInputTokenCost`.

## [0.15.0] - 2026-06-29

### Added

- **The operator's own admin-API calls now use verified HTTPS when the proxy serves TLS.** When `spec.tls.serverCertSecretRef` is set, `status.endpoint` becomes `https://…` — the single URL every controller (model/team/user/key/org/customer/credential sync, config sync) and the health probes use — so all operator→proxy traffic is TLS automatically. The operator **validates the serving certificate**: it trusts the CA from the server-cert Secret's `ca.crt` (cert-manager populates it for CA/intermediate issuers), falling back to `spec.tls.trustedCASecretRef`; a publicly-trusted cert needs neither. Verification is never disabled. A `ValidationFailed` warning event is emitted if HTTPS is served but no CA is resolvable (operator→proxy calls would then fail verification). This completes the serve-TLS feature from v0.14.0, which switched the *listener* and probe scheme to HTTPS but left the operator's client on `http://` — it would have lost the ability to reconcile against a TLS-serving gateway.

## [0.14.0] - 2026-06-28

### Added

- **`LiteLLMInstance.spec.tls` — TLS for the proxy pod.** Three secret-ref-based knobs, all accepting cert-manager's standard `tls.crt`/`tls.key`/`ca.crt` keys:
  - `serverCertSecretRef` — mounts a `kubernetes.io/tls` Secret and sets `SSL_KEYFILE_PATH` + `SSL_CERTFILE_PATH` (together), so uvicorn **serves HTTPS** on port 4000. When set, the container health probes switch to the `HTTPS` scheme and the internal `PROXY_BASE_URL` becomes `https://` — clients (and any Ingress/Route/HTTPRoute in front) must use `https://`.
  - `trustedCASecretRef` (`{ name, key (default ca.crt) }`) — mounts a CA bundle and sets `SSL_CERT_FILE` so **outbound** provider calls and logging callbacks (e.g. Langfuse) trust a custom CA. This is the documented LiteLLM/httpx knob (`SSL_CERT_FILE`, not `REQUESTS_CA_BUNDLE`); being process-level, it also covers the callback HTTP clients (historic Langfuse gap [BerriAI/litellm#7046](https://github.com/BerriAI/litellm/issues/7046)).
  - `clientCertSecretRef` — mounts a TLS Secret and sets `SSL_CERTIFICATE` for **outbound mTLS**.
- **`LiteLLMInstance.spec.database.tls` — PostgreSQL TLS.** `caSecretRef` and `clientCertSecretRef` mount the Postgres CA bundle (`/etc/litellm/db-tls/ca/<key>`) and, for mTLS, client cert/key (`/etc/litellm/db-tls/client/`) on **both** the proxy Deployment and the migration Job. Because `DATABASE_URL` is sourced from a Secret the operator does not rebuild — and Prisma's native connector reads SSL params from the connection string (using `sslmode=require&sslaccept=strict`, **not** libpq's `verify-full` or `PG*` env vars) — the operator only mounts the material; the caller adds `?sslmode=require&sslaccept=strict&sslrootcert=…` (and `sslcert`/`sslkey`) to the URL. (No `sslMode` field is exposed because it cannot be enforced on a Secret-sourced URL and would be misleading.)
- **`LiteLLMInstance.spec.extraVolumes` / `spec.extraVolumeMounts`** — generic escape hatch to attach arbitrary volumes/mounts to the proxy pod (the env escape hatch, `spec.extraEnvVars`/`spec.extraEnvFrom`, already existed).
- Validation: the instance controller verifies each referenced TLS Secret exists and that cert Secrets carry both `tls.crt` and `tls.key`, emitting `SecretNotFound` / `SecretKeyMissing` warning events (non-fatal).

### Security

- **Bumped Go to 1.25.11** (go.mod and the `golang:1.25` builder image digest) to clear two HIGH standard-library advisories flagged by the image scanner — CVE-2026-27145 and CVE-2026-42504 (both fixed in Go 1.25.11). No code changes; the operator binary is rebuilt against the patched stdlib.

## [0.13.0] - 2026-06-27

### Fixed

- **Azure models authenticated via `LiteLLMModel.spec.litellmParams.credentialRef` now work at request time.** A model that referenced a `LiteLLMCredential` was registered with only `litellm_credential_name` and no inline `api_base`, on the assumption LiteLLM would resolve the named credential into the DB-stored model at request time. It does not, reliably: LiteLLM hydrates a DB model's `litellm_credential_name` at router-load time, and on a cold start that runs **before** DB-backed credentials (those created via the `/credentials` API) are loaded into the in-memory `credential_list` — so the lookup returns nothing and the Azure deployment boots with no endpoint, failing every request with `AzureException APIError - Must provide one of the base_url or azure_endpoint arguments`. It would self-heal on the 30s router resync and break again on the next pod restart. The model controller now **resolves the credential's `api_base` / `api_version` / `api_key` and writes them inline** on the `/model/new`/`/model/update` payload (LiteLLM only fills fields left unset, so inline always wins and is restart-safe), while still sending `litellm_credential_name` for Admin UI association and best-effort merge of any extra credential params. The DB-backed credential is unchanged, so secret-rotation-without-restart is preserved — and the model's sync hash now covers the resolved auth material, so a rotated Secret or edited credential re-pushes the model even though `model.Spec` is unchanged.

### Added

- **`LiteLLMModel.spec.litellmParams.apiVersion`** — sets the provider `api_version` inline (required by most Azure OpenAI / Azure AI Foundry deployments). Previously the API version was only reachable through `credentialRef`; inline Azure models had no way to set it. When `credentialRef` is set, the credential's `apiVersion` takes precedence.

### Changed

- **Switching a model between `credentialRef` and inline auth now deletes and recreates the LiteLLM model.** `/model/update` is a merge and cannot clear provider fields (`api_base` / `api_key` / `litellm_credential_name`) left by the previous auth mode — so flipping modes used to leave stale values on the DB model. The controller now tracks the last-pushed auth mode (`litellm.palena.ai/auth-mode` annotation) and, on a flip, deletes and re-creates the model for a clean record. In-mode field changes still use `/model/update`.

## [0.12.2] - 2026-06-14

### Fixed

- **Gateway now actually loads its generated config — `litellm_settings` (success/failure callbacks, etc.) are no longer silently dropped.** The operator mounted the rendered `proxy_server_config.yaml` ConfigMap at `/app/config` and set `LITELLM_CONFIG_DIR=/app/config`, but current LiteLLM does **not** honor `LITELLM_CONFIG_DIR` — it only reads its config from the `CONFIG_FILE_PATH` env var (or a `--config` arg). So the file was mounted but never read: completions returned 200 (models still loaded from the DB via `STORE_MODEL_IN_DB=True`, masking the bug) while `success_callback`/`failure_callback` and every other `litellm_settings` entry were ignored — no Langfuse traces (or any callback) were ever emitted. The Deployment now sets `CONFIG_FILE_PATH=/app/config/proxy_server_config.yaml`, derived from the same constants as the volumeMount path and the ConfigMap data key so they cannot drift, and litellm logs `Initialized Success Callbacks - [...]` on startup. The dead `LITELLM_CONFIG_DIR` env var was removed.

### Changed

- **Default gateway image tag is now `latest` instead of `main-latest`.** `main-latest` tracks LiteLLM's `main` branch (unreleased nightly builds, labeled `org.opencontainers.image.version=main`), which is a poor default for a "production-ready" gateway. `ghcr.io/berriai/litellm:latest` tracks the most recent tagged **release** (currently resolves to `v1.87.0`), so new `LiteLLMInstance`s without an explicit `spec.image.tag` get a released LiteLLM that satisfies the operator's v1.86+ migration assumptions. Existing CRs are unaffected (the value is already persisted); pin `spec.image.tag` for reproducible deployments. Applied consistently to the CRD default, the deployment/migration-Job fallbacks, and `status.version`.

## [0.12.1] - 2026-06-03

### Fixed

- **`RedisReady` condition is no longer permanently `False` ([#10](https://github.com/PalenaAI/litellm-operator/issues/10)).** The instance health probe parsed a `redis` boolean off `GET /health/readiness`, but LiteLLM has never emitted that field — and as of 1.86.x the public readiness payload was reduced to `{"status", "db"}` — so the absent field decoded to Go's zero value (`false`), reporting Redis as disconnected on every reconcile and generating continuous spurious `RedisDisconnected` warning events despite healthy Redis. Redis health is now probed correctly: when response caching is Redis-backed, the operator calls `GET /cache/ping` (which actively pings Redis and performs a test write) for a genuine connectivity verdict; when Redis is wired only for router coordination (no Redis-backed cache), LiteLLM exposes no runtime signal, so the condition reports `Ready=True` with reason `RedisConfigured` instead of falsely claiming disconnection. The phantom `ReadinessResponse.RedisConnected`/`CacheHealth` fields were removed.
- **CI: license-header check now passes.** `.licenserc.yaml` declared the header text as `Copyright [year] bitkaio LLC` (no trailing period), but every source file's actual header — stamped from `hack/boilerplate.go.txt` — reads `Copyright [year] bitkaio LLC.` (with a period). skywalking-eyes requires the configured text to match, so the mismatch failed *every* Go file. The check only runs on `pull_request` events (not pushes to `main`), so it had been silently broken since it was introduced. Added the trailing period to the config to match the established convention.
- **CI: operator scorecard `olm-spec-descriptors` now passes.** The `LiteLLMGuardrail` fields `guardrailClass` and `unreachableFallback` (added in v0.12.0) were missing matching `specDescriptors` in the ClusterServiceVersion, so the bundle's sample CR exercised fields with no descriptor. Added both descriptors to the CSV base (and regenerated bundle). The scorecard job only runs on pushes to `main`, so this surfaced after v0.12.0 merged.
- **Helm chart no longer pins the operator image to the long-stale `v0.5.0`.** `values.yaml` hardcoded `image.tag: "v0.5.0"`, which overrode the intended `appVersion` fallback in the image helper — so every default `helm install` deployed the **v0.5.0 operator** regardless of the chart version installed. `image.tag` now defaults to empty and the helper resolves to `v<appVersion>` (matching the `v`-prefixed tags published by the release workflow), so a default install tracks the chart's release. An explicit `--set image.tag=…` is still honored verbatim.

## [0.12.0] - 2026-06-02

### Fixed

- **Database migration Job now actually applies LiteLLM's versioned migrations.** The Job command has switched from `prisma db push --schema=/app/schema.prisma --accept-data-loss --skip-generate` to `prisma migrate deploy --schema=/app/schema.prisma`. `db push` syncs the live DB to `schema.prisma` directly and **ignores LiteLLM's 38+ versioned migration files** in `litellm-proxy-extras/migrations` — fine for fresh installs, but on upgrades it left `_prisma_migrations` out of sync with reality and could drop columns under `--accept-data-loss`. The previous behavior was masked pre-LiteLLM v1.86 because the proxy itself ran a schema sync on startup; v1.86's componentization PR ([#27557](https://github.com/BerriAI/litellm/pull/27557)) disabled in-pod schema updates, exposing the issue as failed migrations / missing columns at request time. `migrate deploy` is the same command LiteLLM's own componentized Helm chart uses and works against every gateway image v1.85.x and later (the migrations directory is shipped in the image).

### Added

- **`spec.database.migration.useDatabaseImage`** — opt-in toggle to run LiteLLM's dedicated `ghcr.io/berriai/litellm-migrations` migrations image (introduced in v1.86.0) **instead of** invoking prisma inside the gateway image. When `true`, only the database image runs and the operator does not override its Command — the image's entrypoint (`python3 /app/run.py`) wraps `prisma migrate deploy` with P3005 baseline / P3009/P3018 idempotent-error recovery and the v2 migration resolver that avoids schema thrashing during rolling deploys. **Recommended for v1.86+ and especially for upgrading from operator versions that previously used `prisma db push`** — the database image's recovery flow heals the resulting `_prisma_migrations`/schema drift automatically, where the gateway-image path would raise Prisma P3005 on a non-empty database. Tag defaults to `spec.image.tag` so the migrations image stays version-aligned with the gateway; gateway pull secrets are reused. **Tag availability caveat:** as of June 2026 the LiteLLM team only publishes `litellm-migrations` tags for release candidates (e.g. `v1.87.0-rc.1`, `v1.88.0-rc.1`) — no v1.86.x or v1.87.0 stable tag exists yet. If your gateway tag isn't published, override via `databaseImage.tag` or stay on the gateway-image path (which now runs the same `ProxyExtrasDBManager.setup_database` recovery logic inside the gateway image and works on every LiteLLM v1.85+ tag).
- **`spec.database.migration.databaseImage`** — optional repo/tag/pullPolicy override for `useDatabaseImage` (e.g. private registry mirror, pinning to a different version). Only consulted when `useDatabaseImage: true`.
- **`LiteLLMGuardrail` — HTTP/API guardrails via `generic_guardrail_api`.** You can now point the proxy at any HTTP service you host (e.g. a container running in your cluster) instead of baking a Python class into the proxy image. Set `spec.provider: generic_guardrail_api` and `spec.apiBase` to your guardrail Service; LiteLLM POSTs request/response content to `{apiBase}/beta/litellm_basic_guardrail_api` and acts on the `{action, blocked_reason, texts, images}` verdict it returns. New `spec.unreachableFallback` field (`fail_closed` / `fail_open`) controls behavior when the endpoint is unreachable, `spec.apiKeySecretRef` is sent as a Bearer token, and `spec.params` are forwarded under `additional_provider_specific_params`. The controller validates that `apiBase` is set for this provider and that `unreachableFallback` is only used with it. Note: this is a BETA LiteLLM feature — its request/response contract may change.
- **`LiteLLMGuardrail` — `custom_guardrail` class path support.** Added `spec.guardrailClass`, the dotted Python import path to a `CustomGuardrail` subclass (e.g. `my_pkg.adapters.MyGuardrail`). Previously `provider: custom_guardrail` emitted the literal `guardrail: custom_guardrail` into `proxy_server_config.yaml`, which LiteLLM cannot resolve, so custom guardrails were unusable end-to-end. The operator now writes the class path as `litellm_params.guardrail`, and the controller enforces that `guardrailClass` is set iff the provider is `custom_guardrail`. The class and its dependencies must be present in the proxy image (custom image via `spec.image`).

## [0.11.3] - 2026-05-26

### Security

- **Go toolchain bumped to 1.25.10.** The previous Dockerfile pin (`cd05a378…`) and `go.mod` directive (`go 1.25.0`) both resolved to Go 1.25.9 in practice, which carries five HIGH stdlib CVEs (CVE-2026-33811, -33814, -39820, -39836, -42499) that broke the weekly Trivy scan against `:latest` and the govulncheck CI step. `go.mod` is now `go 1.25.10` and the Dockerfile pins `golang:1.25@sha256:c138bff7…` (= Go 1.25.10). All seven `actions/setup-go` invocations in CI / E2E / release / scheduled now use `check-latest: true` so future Go patch releases flow in via the latest matching `go 1.25.x` without requiring a `go.mod` edit.
- **`golang.org/x/net` bumped 0.52.0 → 0.55.0** to clear GO-2026-4918 (HTTP/2 server-push DoS) — the only third-party finding from govulncheck. Pulled in by `go mod tidy`; `x/sys`, `x/term`, `x/text`, and `x/tools` came along as transitive bumps.

### Fixed

- **`LiteLLMCredential` is now actually honored at request time.** The credential controller previously rendered credentials into `credential_list` inside `proxy_server_config.yaml`. LiteLLM only merges `credential_list` entries into models defined in the config file's `model_list`; models registered via `POST /model/new` (which is how the operator registers every `LiteLLMModel`) are stored in the DB and *do not* see config-level credentials. Net effect: `LiteLLMModel.spec.litellmParams.credentialRef` was silently a no-op for the entire v0.11.x series, and any provider that requires `api_base`/`api_version` (notably all Azure OpenAI / Azure AI Foundry models) failed with `Must provide one of the base_url or azure_endpoint arguments`. The controller now reconciles credentials against LiteLLM's `/credentials` API (`POST` / `PATCH` / `DELETE` `/credentials/{name}`), which stores them in the DB encrypted with `LITELLM_SALT_KEY` and merges them into request-time `litellm_params`. The credential is now also visible in the Admin UI's Credentials tab.

### Changed

- **Kubernetes Secret rotation now propagates to LiteLLM in seconds.** The credential controller adds a Secret watch: when the referenced `apiKeySecretRef` Secret changes, the controller is enqueued immediately, computes a fresh `(spec + secret-value)` hash, and pushes a `PATCH /credentials/{name}` if the hash differs from the one stored in the `litellm.palena.ai/sync-hash` annotation. No proxy pod restart is required — DB credentials are looked up on each request.
- **BREAKING (internal):** `BuildConfigMap`, `BuildDeployment`, and `GenerateProxyConfig` in `internal/resources` no longer take a `[]LiteLLMCredential` parameter. The `CREDENTIAL_<name>_API_KEY` env var the operator used to inject on the proxy Deployment (so config-level `os.environ/...` references resolved at startup) is no longer emitted, and the `credential_list` block in `proxy_server_config.yaml` is no longer rendered. This is a no-op for end users — credentials still work the same from the CR side — but anyone reading the rendered ConfigMap directly will notice the section is gone.
- **`LiteLLMCredential` controller now requires a Ready `LiteLLMInstance`.** Previously the controller could validate a credential against a not-yet-ready instance (because rendering into a config file did not need the proxy reachable). The new flow needs the proxy `/credentials` endpoint, so credentials whose instance is not Ready stay `Configured=false` with `reason=InstanceNotReady` until the instance reports ready. Order of CR creation does not matter — once the instance is ready, the Secret-watch + periodic resync push the credential automatically.

### Notes

- **Migrating an existing v0.11.x cluster:** on upgrade, the new controller will register each `LiteLLMCredential` against `/credentials` on its first reconcile. The old `credential_list` block in the proxy `ConfigMap` is dropped on the next instance reconcile, the proxy rolls once (because the ConfigMap hash changes), and from then on credentials are DB-backed. Any existing `LiteLLMModel` with `credentialRef` continues to send `litellm_credential_name` in its params — that wire format is unchanged; only what's behind the lookup moved from config to DB.

## [0.11.2] - 2026-05-21

### Fixed

- **Weekly scheduled scans no longer fail on the published OLM bundle.** Two unrelated bugs caused the `Scheduled scans` workflow to fail every Monday and auto-open false-positive Trivy CVE issues. (1) The `litellm-operator-bundle` GHCR package was private, so anonymous Trivy and `operator-sdk bundle validate` calls returned `UNAUTHORIZED`; the package has been switched to public visibility to match the operator image. (2) `operator-sdk bundle validate` was being passed a `docker://…` reference, but it shells out to `docker pull` which does not understand that scheme — the prefix has been removed.
- **OLM bundle CSV — empty `icon` block removed.** The CSV declared an icon entry with empty `base64data`, which made `operator-sdk bundle validate --select-optional suite=operatorframework` fail with `csv.Spec.Icon elements should contain both data and mediatype`. The block has been removed from both the kustomize base and the generated bundle manifest; a real icon can be added later when an SVG asset is available.
- **Helm chart — operator ClusterRole synced with the kustomize source.** The hand-maintained Helm `clusterrole.yaml` had drifted significantly from `config/rbac/role.yaml` (generated from `+kubebuilder:rbac` markers). On v0.11.x the operator pod crashed at startup with `failed to wait for caches to sync` because list/watch on `litellmcredentials` and `litellmguardrails` (added in v0.11.x) was forbidden. Several other features were also silently broken for Helm-installed operators: Gateway API `httproutes`, OpenShift `routes`, Prometheus `servicemonitors` + `prometheusrules`, and CloudNativePG `scheduledbackups` were all missing from the chart's ClusterRole. The template now mirrors the kustomize ruleset 1:1 (verified by tuple-diff). OLM/kustomize installs were not affected.

### Security

- **Dockerfile base images pinned by digest.** `golang:1.25` and `gcr.io/distroless/static:nonroot` are now pinned by SHA256 digest in addition to their tags, addressing the OpenSSF Scorecard `Pinned-Dependencies` finding for the operator container. Renovate continues to manage updates via tag rules.
- **Go toolchain bumped to 1.25.10 (via base-image digest).** Repinned `golang:1.25` from the 1.25.9 digest to the 1.25.10 digest. This clears five HIGH stdlib CVEs that Trivy was failing the E2E pipeline on: `CVE-2026-33811` (cgo DNS resolver long-CNAME parsing), `CVE-2026-33814` (HTTP/2 SETTINGS infinite loop), `CVE-2026-39820` (`net/mail` address parsing DoS), `CVE-2026-39836` (Windows `Dial`/`LookupPort` NUL-byte panic), and `CVE-2026-42499` (`consumePhrase` DoS).
- **Trivy bumped 0.69.3 → 0.70.0** across `e2e.yml`, `release.yml`, and `scheduled.yml` for up-to-date scanner behaviour and vulnerability DB compatibility.

## [0.11.1] - 2026-04-17

### Fixed

- **Release pipeline — GitHub release upload no longer fails on oversized scancode report.** `scancode-toolkit` in the release workflow previously scanned the entire Go module cache (`$(go env GOMODCACHE)`, ~5 GB of vendored dependencies), which produced a ~50 MB `scancode-report.json`. The scan took ~100 minutes and the oversized report caused `softprops/action-gh-release` to fail with `Request body length does not match content-length header`, leaving releases partially published. The scan is now scoped to the project source tree (`.`) with ignore patterns for build outputs (`bin/`, `dist/`, `testbin/`, `release-artifacts/`, tarballs). Dependency license coverage remains fully intact via `go-licenses` (hard-fail on forbidden/restricted) and the `syft` CycloneDX SBOM.
- **CI lint — removed dead `customSSOPackageName` constant** in `internal/resources/deployment.go` that was left over after the custom SSO handler refactor. `golangci-lint` was failing the build on the `unused` linter.
- **CI drift check — regenerated Helm chart CRDs.** The `customSsoHandler` field in `deploy/charts/litellm-operator/crds/litellm.palena.ai_litellminstances.yaml` was still the old plain-string shape instead of the union struct introduced in v0.11.0. `make sync-helm-crds` now produces a clean tree.

## [0.11.0] - 2026-04-17

### Added

- **SSO logout redirect** — `spec.sso.logoutUrl` on `LiteLLMInstance` is now wired to the `PROXY_LOGOUT_URL` env var on the Deployment. When set, the Admin UI's logout action redirects to the IdP's end-session endpoint so users are signed out of both LiteLLM and the IdP in one click.
- **SSO custom handler (ConfigMap-backed)** — `spec.sso.customSsoHandler` is now wired to `general_settings.custom_sso` and supports two modes: `module` (dotted Python path to a handler baked into a custom image) or `configMapRef` (operator mounts the handler source from a ConfigMap at `/app/custom_sso_handlers/` and writes the derived module path — `custom_sso_handlers.<stem>.<functionName>`). Handlers run inside the LiteLLM pod with the gateway's privileges; ConfigMap changes require a pod rollout to take effect.
- **SSO default-user team auto-assignment** — `spec.sso.defaultUserParams.teams` is now emitted under `litellm_settings.default_internal_user_params.teams` in the generated `proxy_server_config.yaml`. Each entry maps `teamId` → `team_id`, `role` → `user_role`, and optional `maxBudgetInTeam` → `max_budget_in_team`. New SSO users are auto-enrolled in the listed teams on first login.

### Changed

- **BREAKING:** `spec.sso.customSsoHandler` changes shape from a plain string (previously unwired) to a union struct (`{module | configMapRef}`). Anyone who had set the old string form was not getting any behaviour; on upgrade, migrate the value into `sso.customSsoHandler.module`.
- **`extraEnvVars` now overrides operator-set env vars by name.** Previously, putting a variable like `PROXY_BASE_URL` in `spec.extraEnvVars` resulted in two entries with the same name in the Pod spec (operator value first, user value second). The operator now merges user-supplied env vars over operator-derived ones: the user entry replaces the operator entry in place and no duplicates are emitted. Useful for overriding `PROXY_BASE_URL` when exposing the gateway via Gateway API HTTPRoute, OpenShift Route, or an external load balancer — cases where the operator's ingress-based derivation falls back to in-cluster Service DNS.

## [0.10.0] - 2026-04-13

### Added

- **Admin UI management** — new `spec.adminUI` field on `LiteLLMInstance` configures the built-in Admin UI. `disabled` disables the UI entirely via the `DISABLE_ADMIN_UI` environment variable. `adminOnly` restricts UI access to proxy admins via `ui_access_mode: "admin_only"` in `general_settings`. `storeModelInDB` enables dynamic model management from the UI without proxy restart via `store_model_in_db`. `defaultTeamDisabled` prevents personal key creation via `default_team_disabled`. `apiDocBaseURL`, `docsURL`, and `rootRedirectURL` customize API docs and root redirect behavior via environment variables. `logoURL` sets a custom logo for the Admin UI via `UI_LOGO_PATH`. `emailLogoURL` and `emailSupportContact` customize email notification branding via `EMAIL_LOGO_URL` and `EMAIL_SUPPORT_CONTACT`. `colorThemeConfigMapRef` mounts a ConfigMap containing `enterprise_colors.json` into the container for custom UI color themes (Tremor color palette). All settings are optional — when `adminUI` is omitted, LiteLLM uses its defaults.

- **Per-team logging (enterprise)** — new `spec.logging` field on `LiteLLMTeam` configures per-team logging destinations and GDPR-compliant logging disable. Each team can have its own logging callbacks routing to separate provider instances (Langfuse, GCS Bucket, LangSmith, Arize) via the `/team/{team_id}/callback` API. Callback credentials are read from Kubernetes Secrets and passed securely in the API call. Setting `logging.disabled: true` calls `/team/{team_id}/disable_logging` to prevent any request/response data from being logged for that team (GDPR compliance). Status fields `loggingSynced` and `loggingDisabled` reflect the current state.

- **Instance-level logging controls** — new `spec.logging` field on `LiteLLMInstance` configures audit logs, global message logging, and spend log retention. `auditLogs.enabled` writes `store_audit_logs: true` to `general_settings` with optional `retentionDays` (enterprise). `turnOffMessageLogging` disables logging of request/response content (only metadata is logged). `redactUserApiKeyInfo` redacts API key information from logs. `spendLogRetention` configures `maximum_spend_logs_retention_period` and `maximum_spend_logs_retention_interval` in `general_settings`.

- **Role-based access control (RBAC)** — new `spec.rbac` field on `LiteLLMInstance` configures LiteLLM's RBAC enforcement. Supports `enforce_rbac`, `admin_only_routes` (restrict specific routes to proxy admins), `allowed_routes` (restrict which routes are accessible at all), `default_team_disabled` (force team-based keys), `key_generation_settings` (control which roles can generate team/personal keys, enterprise), and `role_permissions` (per-role route and model access, enterprise). Settings are written to `general_settings` in the generated `proxy_server_config.yaml`. The instance controller sets an `EnterpriseFeaturesConfigured` warning condition when enterprise RBAC features (`key_generation_settings`, `role_permissions`) are enabled without a license Secret. Includes unit tests and documentation.

- **JWT/OAuth2 authentication (enterprise)** — new `spec.jwtAuth` and `spec.oauth2Auth` fields on `LiteLLMInstance` configure API-level authentication via JWT tokens and OAuth2 machine-to-machine auth. JWT auth (`enable_jwt_auth` + `litellm_jwtauth`) validates tokens from identity providers and maps claims to LiteLLM roles, teams, organizations, and end-users. Supports all claim field mappings (`teamIdJwtField`, `teamIdsJwtField`, `orgIdJwtField`, `userIdJwtField`, `userEmailJwtField`, `userRoleJwtField`, `endUserIdJwtField`), admin scope configuration (`adminJwtScope` + `adminAllowedRoutes`), public key TTL, and scope-to-model mappings for fine-grained model access control. OAuth2 auth (`enable_oauth2_auth` + `oauth2_config_mappings`) enables service-to-service authentication by mapping JWT fields to LiteLLM attributes (e.g., `client_id` → `team_id`). Both features complement the existing SSO support (which handles Admin UI login) by enabling API-level authentication. The instance controller sets an `EnterpriseFeaturesConfigured` warning condition when JWT/OAuth2 is enabled without a license Secret. Settings are written to `general_settings` in the generated `proxy_server_config.yaml`. Includes sample CR, unit tests, and documentation.

- **External secret manager integration** — new `spec.secretManager` field on `LiteLLMInstance` configures LiteLLM's native secret manager support. LiteLLM connects to the external provider at runtime to fetch API keys, store generated virtual keys, and read configuration secrets — without the secrets ever being stored in Kubernetes etcd. Supports 6 providers: AWS Secret Manager (`aws_secret_manager`), AWS KMS (`aws_kms`), Azure Key Vault (`azure_key_vault`, enterprise), Google Secret Manager (`google_secret_manager`), Google KMS (`google_kms`), and HashiCorp Vault (`hashicorp_vault`). Configurable `hostedKeys` (env var names resolved from the secret manager), `storeVirtualKeys`, `prefixForStoredVirtualKeys`, `accessMode` (read_only/write_only/read_and_write), and `primarySecretName`. Provider credentials are injected via `envFrom` from a referenced Kubernetes Secret; AWS IRSA and GKE Workload Identity are supported by omitting the credentials Secret and configuring the workload identity token path. Provider-specific settings (AWS region/role/STS endpoint, Azure vault URI/tenant, Vault address/namespace/auth method/mount/prefix/refresh interval) are injected as environment variables. The instance controller validates the credentials Secret and reports `status.secretManager.configured` and `status.secretManager.provider`. This is complementary to the External Secrets Operator approach — both patterns are valid and can coexist.

## [0.9.0] - 2026-04-11

### Added

- **Guardrails (content moderation / safety)** — new `LiteLLMGuardrail` CRD (short name `lg`) declaratively manages guardrail integrations for content moderation, PII detection, jailbreak prevention, and prompt injection defence. Supports 10 providers (`aporia`, `lakera`, `bedrock`, `presidio`, `guardrails_ai`, `azure`, `llm_guard`, `llamaguard`, `google_text_moderation`, `custom_guardrail`) and all four execution modes (`pre_call`, `post_call`, `during_call`, `logging_only`). Each guardrail references an optional `apiKeySecretRef` for the provider credentials, an optional `apiBase`, a `defaultOn` flag, free-form provider `params`, and additional `envVars`. Guardrails are **config-level resources** materialized by the instance controller: each entry is rendered into the `guardrails` section of the generated `proxy_server_config.yaml`, and the API key is injected into the pod via a `secretKeyRef`-backed env var (`GUARDRAIL_{NAME}_API_KEY`) referenced from config as `os.environ/…`. The instance controller watches `LiteLLMGuardrail` CRs and rebuilds the ConfigMap + Deployment when they change. New `spec.guardrails []string` field on `LiteLLMVirtualKey` and `LiteLLMTeam` lets keys and teams opt into specific guardrails (enterprise feature) — the list is forwarded to `/key/generate`, `/key/update`, `/team/new`, and `/team/update`. Includes a dedicated validation controller that checks the instance reference and the API key Secret, sample CR with three example providers (Aporia, local Presidio, AWS Bedrock), RBAC, Helm chart CRD, and unit tests for config generation, env var collection, instance filtering, and env var sanitization.
- **Credential management** — new `LiteLLMCredential` CRD (short name `lc`) manages reusable provider credentials declaratively. Each credential references a Kubernetes Secret for the API key and an optional `apiBase` / `apiVersion` / free-form `params` map, and is materialized into the `credential_list` section of the generated `proxy_server_config.yaml`. The instance controller watches `LiteLLMCredential` CRs and rebuilds the ConfigMap + Deployment whenever credentials change; the credential's API key is injected into the pod via a `secretKeyRef`-backed env var (`CREDENTIAL_{NAME}_API_KEY`) and referenced from config as `os.environ/…` so the secret value never lives in the operator's memory. `LiteLLMModel` gains an optional `credentialRef` field under `litellm_params` — when set, the model is registered with `litellm_credential_name` instead of inline `apiKeySecretRef` / `apiBase`, letting many models share one credential. The model controller watches `LiteLLMCredential` CRs to re-reconcile dependent models when a credential changes, and the credential controller reports `.status.referencedByModels` and validates that the referenced Secret exists. Includes RBAC roles, sample CR, Helm chart CRD, and controller/unit tests.
- **End-users / Customers** — new `LiteLLMCustomer` CRD (short name `lcust`) manages external end-users of the AI gateway (e.g., SaaS application customers). Unlike `LiteLLMUser` (internal proxy users), a customer represents an *external* consumer identified by an application-supplied ID that LiteLLM tracks as `user_id` / `end_user_id`. Supports per-customer budgets (`maxBudget` + `budgetDuration` or a shared `budgetId` tier), TPM/RPM rate limits, allowed model list, `defaultModel`, `allowedModelRegion`, `blocked` flag, `objectPermission` (MCP servers, vector stores, agents, access groups), and metadata. The controller creates/updates/deletes customers via `/customer/new`, `/customer/update`, `/customer/delete` and refreshes spend from `/customer/info` on each reconcile. `LiteLLMInstance` gains `spec.defaultCustomerBudget` (`maxBudget` and/or `budgetId`) which is written to `litellm_settings.max_end_user_budget` / `max_end_user_budget_id` in the generated proxy config, applying a platform-wide default budget to every customer. Includes RBAC roles (admin/editor/viewer), sample CR, Helm chart CRD, and controller tests.
- **Organizations (multi-tenancy)** — new `LiteLLMOrganization` CRD (short name `lo`) adds the top-level tenant in LiteLLM's hierarchy: **Organization > Team > User > Key**. Supports organization alias, model access lists, budget/duration, TPM/RPM limits, member management (add/remove via API), and metadata. Member sync compares spec with API state and adds/removes as needed. `LiteLLMTeam` gains an optional `organizationRef` field — when set, the team controller resolves the organization's LiteLLM ID and passes `organization_id` to team create/update API calls. Includes RBAC roles (admin/editor/viewer), sample CR, and full controller tests.
- **Advanced budget controls** — new budget and concurrency fields across `LiteLLMInstance`, `LiteLLMTeam`, and `LiteLLMVirtualKey`. `spec.generalSettings` gains `maxBudget`, `budgetDuration`, `globalMaxParallelRequests`, `budgetReschedulerMinTime`, and `budgetReschedulerMaxTime` for global proxy budget and concurrency limits. `spec.routerSettings` gains `defaultMaxParallelRequests` (per-model-deployment concurrency cap) and `providerBudgetConfig` (per-provider spending limits with time periods). `LiteLLMTeam` gains `maxParallelRequests` for team-level concurrency caps. `LiteLLMVirtualKey` gains `modelMaxBudget` (per-model spending limits per key, enterprise) and `maxParallelRequests` (per-key concurrency cap). All new fields are written to `proxy_server_config.yaml` or passed to the LiteLLM API as appropriate.
- **Pass-through endpoints** — new `spec.passThroughEndpoints` field on `LiteLLMInstance` configures arbitrary API pass-through proxying. Each endpoint defines a path, target URL, optional LiteLLM authentication (`auth`), header forwarding (`forwardHeaders`), sub-path routing (`includeSubpath`), allowed HTTP methods, static headers, secret-backed headers (`headerSecrets` with prefix support), and default query parameters. Secret-backed headers are injected as environment variables via `secretKeyRef` and referenced in config as `os.environ/PASSTHROUGH_{PATH}_{HEADER}`. Settings are written to `general_settings.pass_through_endpoints` in the generated `proxy_server_config.yaml`.
- **IP allowlisting (enterprise)** — new `spec.security.ipAllowlist` field on `LiteLLMInstance` configures application-layer IP address filtering. Supports a list of allowed IPs and CIDR ranges (`allowedIPs`), `useXForwardedFor` for correct client IP detection behind load balancers, and optional `maxRequestSizeMB` / `maxResponseSizeMB` limits. Settings are written to `general_settings` in the generated `proxy_server_config.yaml`.
- **Tag-based routing** — new `enableTagFiltering` and `tagFilteringMatchAny` fields on `spec.routerSettings` enable routing requests to model deployments by tag. New `tags` field on `LiteLLMModel` assigns tags to model deployments (passed via `litellm_params.tags`). New `tags` field on `LiteLLMTeam` associates tags with teams so keys generated for team members inherit routing tags. Tags are written into the generated `proxy_server_config.yaml` router settings and included in model/team API create and update calls.
- **Response caching** — new `spec.caching` field on `LiteLLMInstance` configures LiteLLM response caching. Supports 6 cache backends: `redis`, `redis-semantic`, `s3`, `gcs`, `qdrant`, and `local` (in-memory). Configurable TTL, namespace isolation, call-type filtering (`supportedCallTypes`), and `default_off` mode. When cache type is `redis` and no cache-specific Redis config is provided, the operator reuses the instance's existing `spec.redis` connection. Secret references for backend credentials (Redis password, AWS credentials, GCS service account, Qdrant API key) are injected as environment variables.
- **Fallback chains** — new `spec.fallbacks` field on `LiteLLMInstance` configures model fallback routing. Supports `defaultFallbacks` (global fallback list for any error), per-model `modelFallbacks`, `contentPolicyFallbacks` (on content policy violations), `contextWindowFallbacks` (on context window exceeded), and `maxFallbacks` to limit chain depth. Fallback entries map a primary model to an ordered list of fallback models in the format LiteLLM expects.
- **Retry policies** — new `retryPolicy` and `modelGroupRetryPolicy` fields on `spec.routerSettings` configure per-error-type retry counts globally and per model group (e.g., `TimeoutError: 2`, `RateLimitError: 3`). Retries happen on the same model; fallbacks switch to a different model.
- **Enterprise license management** — convention-based LiteLLM Enterprise license activation. The operator detects a well-known Secret (`{instance-name}-license` per-instance, or `litellm-license` namespace-wide fallback) and injects the `LITELLM_LICENSE` environment variable into the Deployment via `secretKeyRef` (the license value is never read into operator memory). License status is reflected in `.status.license`. The controller watches license Secrets and triggers reconciliation on create/update/delete. All downstream controllers (Model, Team, User, VirtualKey) detect enterprise-only API errors (403 + "enterprise") and set `Reason: EnterpriseLicenseRequired` without requeueing.

## [0.7.0] - 2026-04-06

### Added

- **Namespace-scoped watching** — new `--watch-namespaces` flag (comma-separated) restricts the operator to only watch and manage resources in the specified namespaces. Also supports the `WATCH_NAMESPACE` environment variable set automatically by OLM for `OwnNamespace` and `SingleNamespace` install modes. Available in the Helm chart via `watchNamespaces` value.
- **OpenShift Route support** — new `spec.route` field on `LiteLLMInstance` creates an OpenShift Route with configurable host and TLS termination (`edge`, `passthrough`, `reencrypt`). Uses unstructured objects to avoid requiring OpenShift API dependencies.
- **Gateway API HTTPRoute support** — new `spec.gatewayHTTPRoute` field on `LiteLLMInstance` creates a `gateway.networking.k8s.io/v1` HTTPRoute with configurable parent Gateway references, hostname, and section name. Compatible with any Gateway API implementation (Istio, Envoy Gateway, Cilium, etc.).
- **CloudNativePG scheduled backups (Level 3: Full Lifecycle)** — new `spec.database.cloudnativepg.backup` field creates a CloudNativePG `ScheduledBackup` CR with configurable schedule, retention, method (`snapshot`/`barmanObjectStore`), and suspend control. Backup status is reported in `.status.backup`. Requires the CloudNativePG operator to be installed.
- **Auto-rollback on failed upgrades (Level 3: Full Lifecycle)** — when `spec.upgrade.autoRollback: true` is set, the operator tracks the last successful deployment revision. If a deployment hits `ProgressDeadlineExceeded`, the operator automatically triggers a rollback and sets a status condition explaining the action.
- **ServiceMonitor creation (Level 4: Deep Insights)** — `spec.observability.serviceMonitor.enabled: true` now actually creates a `monitoring.coreos.com/v1` ServiceMonitor targeting the LiteLLM proxy's HTTP port with configurable scrape interval and labels. Gracefully degrades if Prometheus Operator CRDs are not installed.
- **PrometheusRule with default alerts (Level 4: Deep Insights)** — `spec.observability.prometheusRule.enabled: true` creates a PrometheusRule with six built-in alerts: `LiteLLMInstanceDown` (critical), `LiteLLMInstanceDegraded`, `LiteLLMPodRestarting`, `LiteLLMPodNotReady`, `LiteLLMHighMemoryUsage`, and `LiteLLMHighCPUUsage`. Each alert includes severity labels, descriptive annotations, and a runbook. Individual alerts can be disabled via `spec.observability.prometheusRule.disabledAlerts`.
- **Grafana dashboard ConfigMap (Level 4: Deep Insights)** — `spec.observability.grafanaDashboard.enabled: true` creates a ConfigMap with the `grafana_dashboard: "1"` label for auto-discovery by the Grafana sidecar. The dashboard includes panels for ready/desired replicas, pod restarts, CPU/memory usage, network I/O, and deployment conditions. Configurable folder and labels.
- **E2E test coverage** — comprehensive end-to-end tests for the full CRD lifecycle (LiteLLMInstance, LiteLLMModel, LiteLLMTeam, LiteLLMUser, LiteLLMVirtualKey) running against a real Kind cluster with a LiteLLM proxy.

### Changed

- Go version updated from 1.24 to 1.25.
- Dockerfile and devcontainer updated to Go 1.25.
- golangci-lint updated to v2.11.4 for Go 1.25 compatibility.

## [0.6.0] - 2026-04-04

### Added

- **OpenShift / non-root support** — new `spec.security.runAsNonRoot` field on `LiteLLMInstance` automatically switches to the official `litellm-non_root` image (`ghcr.io/berriai/litellm-non_root`), sets `RunAsNonRoot: true`, and runs as `nobody` (UID 65534). Compatible with OpenShift restricted SCC and Kubernetes Pod Security Standards.
- **ServiceAccount reconciliation** — the `LiteLLMInstance` controller now creates a ServiceAccount for the LiteLLM pods, preventing `CreateContainerConfigError` when the referenced ServiceAccount did not exist.
- **Helm chart** — new Helm chart in `deploy/charts/litellm-operator/` as an alternative to OLM-based installation. Includes ClusterRole, ClusterRoleBinding, ServiceAccount, Deployment, leader election RBAC, and all CRD manifests.

### Fixed

- **Secondary controllers failed with `masterKey.autoGenerate`** — `resolveInstance` now correctly derives the auto-generated master key Secret name (`{instance}-master-key`) when `spec.masterKey.secretRef` is nil and `autoGenerate: true` is set. Previously all secondary controllers (Model, Team, User, VirtualKey) failed with `"secret ref is nil"`.
- **Model update returned 400 "model not found"** — the `/model/update` LiteLLM API endpoint requires `model_info.id` in the request body. Added `ID` field to `ModelInfoReq` and set it in the model update path.
- **Duplicate resource creation on first sync** — all four secondary controllers (Model, Team, User, VirtualKey) could create duplicate resources in LiteLLM because the status subresource (containing the LiteLLM resource ID) was not persisted before the annotation update triggered a re-queue. Fixed by calling `Status().Update()` before `Update()` in the create path, and setting the sync hash annotation on create (not just on update).
- **Default resource limits too low** — bumped default container resources from 100m/256Mi requests and 1 CPU/512Mi limits to 250m/512Mi requests and 2 CPU/2Gi limits. LiteLLM's Python runtime and Prisma imports require more memory than the previous defaults.
- **Container security context too restrictive** — removed hardcoded `RunAsNonRoot: true`, `ReadOnlyRootFilesystem: true`, and `RunAsUser: 1001` from the default container security context. LiteLLM's default image runs as root and writes to the filesystem at startup. Non-root execution is now opt-in via `spec.security.runAsNonRoot`.
- **Migration Job uses correct image and security context** — the database migration Job now respects `spec.security.runAsNonRoot`, using the non-root image and correct UID (65534) when enabled.
- **Migration Job command updated** — changed from Python `asyncio.run(main())` to `prisma db push` which is the supported migration approach.

### Changed

- Pod security context is now conditional: applied only when `spec.security.runAsNonRoot: true`, instead of being hardcoded for all deployments.
- Image repository selection is automatic: `ghcr.io/berriai/litellm` for default mode, `ghcr.io/berriai/litellm-non_root` when non-root is enabled. Users can still override via `spec.image.repository`.

## [0.5.0] - 2026-04-01

### Added

- **LiteLLMInstance CRD** — deploy production-ready LiteLLM proxy instances with Deployment, ConfigMap, Service, Ingress, HPA, PDB, NetworkPolicy, and database migration Job management
- **LiteLLMModel CRD** — register AI models (OpenAI, Anthropic, Azure, etc.) with the LiteLLM proxy via the REST API
- **LiteLLMTeam CRD** — create and manage teams with budget limits, rate limits, and three member management modes (`crd`, `sso`, `mixed`)
- **LiteLLMUser CRD** — manage users for non-SSO environments (service accounts, bot users) with team memberships
- **LiteLLMVirtualKey CRD** — generate scoped API keys stored in Kubernetes Secrets with owner references for automatic garbage collection
- LiteLLM REST API client with interface-based design and mock implementation for testing
- Finalizer-based cleanup on CRD deletion (calls LiteLLM API delete endpoints)
- Spec hash annotations (`litellm.palena.ai/sync-hash`) for change detection to avoid unnecessary API calls
- Auto-generation of master key and salt key Secrets
- Database migration Job support (runs before Deployment rollout)
- SSO configuration support (Azure Entra ID, Okta, Google, generic OIDC)
- SCIM v2 provisioning configuration
- Redis configuration for caching and routing
- Callback configuration (Langfuse, etc.)
- Observability support (ServiceMonitor for Prometheus)
- Resource generators for all Kubernetes resources (Deployment, ConfigMap, Service, Ingress, HPA, PDB, NetworkPolicy, migration Job)
- Sample CRs for all 5 CRDs in `config/samples/`
- GitHub Actions workflows for tests, linting, and releases
- OLM bundle and catalog manifests for OperatorHub distribution

[Unreleased]: https://github.com/PalenaAI/litellm-operator/compare/v0.11.2...HEAD
[0.11.2]: https://github.com/PalenaAI/litellm-operator/compare/v0.11.1...v0.11.2
[0.11.1]: https://github.com/PalenaAI/litellm-operator/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.7.0...v0.9.0
[0.7.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/PalenaAI/litellm-operator/releases/tag/v0.5.0

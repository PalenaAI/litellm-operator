# LiteLLM Operator — Feature Gap Audit

**Date:** 2026-07-02 · **Operator version:** v0.15.0 (+ unreleased model-field extensions)

Method: four parallel audits, each reading the actual Go structs / render code (not
CLAUDE.md, which is stale) and cross-referencing the current LiteLLM surface
(docs.litellm.ai and `litellm/proxy/_types.py`, July 2026):

1. `LiteLLMInstance` vs proxy config (`general_settings` / `litellm_settings` / `router_settings` / env)
2. Identity CRDs (Org / Team / User / VirtualKey / Customer) vs their REST API params
3. Model / Credential / Guardrail CRDs
4. Whole LiteLLM subsystems with no CRD at all

Priority = real-world usefulness for a GitOps operator, not LiteLLM's own emphasis.
Effort: S ≈ add field + render + test; M ≈ new nested type / mount pattern; L ≈ new CRD / subsystem.

---

## Progress (updated 2026-07-02, unreleased)

- ✅ **Tier 0** — all three dead fields now render (`customKeyGenerate`, `retryAfter`, `connectionPool.maxConnections`).
- ✅ **Tier 1** — shipped: router `enable_pre_call_checks` / `model_group_alias` / `stream_timeout`, `routingStrategy` enum refresh, general `background_health_checks` / `health_check_interval` / `health_check_details`, alerting **delivery** (`alerting` / `alerting_threshold` / `alert_to_webhook_url`), `litellmSettings.jsonLogs`; model `dropParams` + Vertex AI auth; **`blocked`** on Team/User/VirtualKey.
- ✅ **Tier 2 shared types** — `objectPermission` (shared `ObjectPermission`, generalized from Customer), `softBudget`, and `modelRpmLimit`/`modelTpmLimit` now on Org/Team/User/Key. Plus Team `teamMemberBudget`.
- ✅ **Structured params** — `LiteLLMGuardrail`/`LiteLLMCredential` `params` are now `map[string]JSON` (arbitrary JSON values), unblocking Presidio PII config, Vertex SA JSON, numeric thresholds, etc.
- ✅ **`LiteLLMBudget` CRD** — reusable budget/rate-limit tier via `/budget/*`; closes the dangling `budgetId` reference on Customer / `defaultCustomerBudget`.
- ⏳ **Remaining** — two new CRDs (`LiteLLMMCPServer`, `LiteLLMVectorStore`), and remaining per-CRD fields (e.g. Team `model_aliases`, Key `tags`/`allowed_routes`/`budget_id`).

---

## Tier 0 — Bugs: dead CRD fields (fix first, no new API surface)

These fields already exist in the CRD (and deepcopy) but are **never rendered or
injected** by any controller/resource code. A user can set them today and nothing
happens — worse than a missing feature because it implies support that isn't there.

| Field | Should render to | Status |
|---|---|---|
| `spec.generalSettings.customKeyGenerate` | `general_settings.custom_key_generate` | Defined, never emitted by `buildGeneralSettings` |
| `spec.routerSettings.retryAfter` | `router_settings.retry_after` | Defined, never emitted by `buildRouterSettings` |
| `spec.database.connectionPool.maxConnections` | `general_settings.database_connection_pool_limit` (or env) | Defined, never rendered or injected |

Fix = wire them into the existing render funcs (or delete the fields if intentionally
dropped). Small, high-confidence.

---

## Tier 1 — High value, low effort, non-enterprise

| Area | Feature | LiteLLM key | Effort |
|---|---|---|---|
| Instance / router | Pre-call context/region checks | `router_settings.enable_pre_call_checks` | S |
| Instance / router | Model group aliases | `router_settings.model_group_alias` | S |
| Instance / router | Refresh `routing_strategy` enum | add `usage-based-routing-v2`, `cost-based-routing`; drop non-canonical values | S |
| Instance / router | Router stream timeout | `router_settings.stream_timeout` | S |
| Instance / general | **Alerting delivery** | `alerting` / `alert_to_webhook_url` / `alerting_threshold` | M |
| Instance / general | Background health checks | `background_health_checks` / `health_check_interval` / `health_check_details` | S |
| Instance / litellm | Structured JSON logs | `litellm_settings.json_logs` | S |
| Identity (Team/User/Key) | **`blocked`** — disable an entity declaratively | `blocked` | S (×3) |
| Model | **Drop unsupported params** | `litellm_params.drop_params` | S |
| Model | Vertex AI auth | `litellm_params.vertex_project` / `vertex_location` / `vertex_credentials` | S |

Notes:
- **Alerting**: the operator already exposes `alert_types` but not the delivery mechanism
  (Slack/webhook URL) — so alerts never actually fire. This is a "looks supported but
  isn't" gap, close cousin of Tier 0.
- **`blocked`** is the single most-requested incident-response knob and is missing on
  Team, User, and Key (Customer already has it). Batch as one shared change.
- **`drop_params`** is increasingly needed: reasoning models (gpt-5, o-series) reject
  `temperature`/`top_p`; without this a deployment 400s on those params.

---

## Tier 2 — Medium value (batch the cross-CRD ones)

### Recurring cross-CRD gaps — add a shared type once, wire into several CRDs

| Feature | Missing on | LiteLLM param | Notes |
|---|---|---|---|
| `object_permission` (MCP / vector-store / agent gating) | Org, Team, User, Key | `object_permission` | **Customer already has a Go struct to lift** |
| Per-model rate limits | Org, Team, User, Key | `model_rpm_limit` / `model_tpm_limit` (map) | `model_max_budget` is covered on Key/User; the rate-limit maps are not |
| Soft budget alert | Org, Team, User, Key, Customer | `soft_budget` | Alert threshold below the hard cap |

### Per-CRD medium items

| CRD | Feature | LiteLLM param | Effort |
|---|---|---|---|
| Team | Team-level model aliases | `model_aliases` | M |
| Team | Per-member budget | `team_member_budget` / `_duration` | S |
| User | Max parallel requests | `max_parallel_requests` | S |
| User | Org membership assignment | `organizations` / `organization_id` | S |
| VirtualKey | Key tags (routing / tag-budgets) | `tags` | S |
| VirtualKey | Route allowlist | `allowed_routes` | S |
| VirtualKey | Budget tier ref | `budget_id` | S |
| VirtualKey | Relative duration (e.g. "30d") | `duration` (CRD only has absolute `expiresAt`) | S |
| Model | `additional_drop_params`, `azure_ad_token`, `custom_tokenizer`, `mock_response` | resp. keys | S each |
| Guardrail | Multiple modes per guardrail (list) | `mode: [pre_call, post_call]` (CRD is single-value) | S |
| Guardrail | Newer providers | e.g. `cato_networks`, model_armor, pangea — verify + extend enum | S |
| Credential / Guardrail | **Structured `params`** | `params` is `map[string]string`; can't hold Presidio PII entity map or Vertex SA JSON — switch to `apiextensions.JSON` / `RawExtension` | M |
| Instance / general | Allow requests on DB down | `allow_requests_on_db_unavailable` | S |
| Instance / general | Spend/error-log disable toggles | `disable_spend_logs` / `disable_error_logs` / `disable_reset_budget` | S |
| Instance / general | Cancel on client disconnect | `cancel_on_disconnect` | S |
| Instance / general | Auto-redirect UI → SSO | `auto_redirect_ui_login_to_sso` | S |
| Instance / litellm | Global request timeout | `request_timeout` | S |
| Instance / litellm | SSRF URL validation | `user_url_validation` / `user_url_allowed_hosts` | S |

---

## Tier 3 — New subsystems (strategic, larger)

| Subsystem | Shape | Fit | Priority | Effort | Notes |
|---|---|---|---|---|---|
| **MCP servers** (`LiteLLMMCPServer` CRD or `instance.spec.mcpServers`) | declarative config list | Good | **High** | M | Flagship 2026 LiteLLM feature. `LiteLLMCustomer.objectPermission.mcpServers` already **references servers by name with no way to define them** — dangling today. |
| **Vector stores** (`LiteLLMVectorStore` CRD or instance block) | `vector_store_registry` list | Good | Med | S–M | Same dangling-reference problem: Customer can be *granted* stores that nothing *defines*. |
| **Reusable budget tiers** (`LiteLLMBudget` CRD) | `/budget/*` API CRUD, like Org/Team | Good | Med | M | Closes the dangling `budgetId` on Customer + `defaultCustomerBudget`; today tiers must be made out-of-band. |
| Prompt management | root `prompts:` list | Good | Med | S | Beta in LiteLLM → lower urgency. |
| Assistants / Files / Fine-tuning settings | `assistant_settings` / `files_settings` / `finetune_settings` provider-routing blocks | Good (config block only) | Low | S | Only the provider-routing block is declarative; the objects/jobs are runtime. |

---

## Enterprise-gated (deprioritize unless a licensed instance is in play)

Per-model `guardrails` (`litellm_params.guardrails`), key rotation (`auto_rotate` /
`/key/rotate`), temp budget increase (`temp_budget_increase`), tag-based budgets,
`enforced_params`, `public_routes`, `disable_global_guardrails`, org/team/user/key
`object_permission` sub-features, secret-detection / banned-keyword guardrails,
blocked-user lists. Several "uncertain" enterprise flags (`model_rpm_limit`,
`team_member_budget`, `agent_id`, `policies`) need verification against a licensed
instance before committing effort.

---

## Explicitly NOT worth modeling (operational, not declarative)

- **Batches API** (`/v1/batches`) — runtime job submission.
- **Spend / analytics queries** (`/spend/logs`, `/spend/report`, `/global/spend/*`) — read-only ops APIs.
- **Cache management** (`/cache/delete`, `/cache/ping`) and **per-request** cache/no-store headers.
- Key **rotation as an action** (a rotation *policy/schedule* could be a small field; the rotate call itself is runtime).

---

## Suggested sequencing

1. **Tier 0 dead-field fixes** — trivial, and they're actively misleading.
2. **Tier 1 batch** — `blocked` (shared), `drop_params`, the four router knobs, alerting delivery.
3. **Cross-CRD shared types** — `object_permission`, per-model rate-limit maps, `soft_budget` (one type, many CRDs).
4. **`LiteLLMMCPServer` + `LiteLLMBudget` + `LiteLLMVectorStore`** — resolves the three dangling references in `LiteLLMCustomer`/budget.
5. Remaining Tier 2 per-CRD fields as demand dictates.

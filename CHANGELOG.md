# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

[Unreleased]: https://github.com/PalenaAI/litellm-operator/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/PalenaAI/litellm-operator/releases/tag/v0.5.0

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Namespace-scoped watching** — new `--watch-namespaces` flag (comma-separated) restricts the operator to only watch and manage resources in the specified namespaces. Also supports the `WATCH_NAMESPACE` environment variable set automatically by OLM for `OwnNamespace` and `SingleNamespace` install modes. Available in the Helm chart via `watchNamespaces` value.

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

[Unreleased]: https://github.com/PalenaAI/litellm-operator/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/PalenaAI/litellm-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/PalenaAI/litellm-operator/releases/tag/v0.5.0

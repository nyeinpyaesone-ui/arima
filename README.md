# ARIMA Platform

**Go-first ERP core and AI integration bridge.** This repository establishes a production-oriented foundation for domain APIs, typed AI tool contracts, auditable actions, and reliable event delivery.

> Status: engineering foundation. Contracts and scaffolding are not yet a production ERP implementation.

## Principles

- Correctness before performance; explicit tenant and authorization boundaries.
- Business rules remain in the ERP core, never in the AI orchestrator.
- Mutations are idempotent, audited, version-checked, and approval-gated by policy.
- Transactional outbox for reliable event publication; avoid dual writes.
- OpenAPI 3.1 for HTTP and Protobuf for gRPC contracts.
- CI quality gates; deployment requires explicit environment configuration.

## Quick start

Requirements: Go 1.26.10 (or the approved patched Go 1.26 release used by CI and the container build).

```sh
go test ./...
go run ./cmd/erp-api
curl -fsS http://localhost:8080/health/live
curl -fsS http://localhost:8080/health/ready
```

Set `ARIMA_ADDR` to change the listen address (default `:8080`). The application does not automatically load dotenv files.

## Development and deployment workflows

- `.github/workflows/ci.yml`: workflow lint, format, module-integrity, vet, vulnerability, race-test, coverage, and build checks.
- `.github/workflows/container-image.yml`: build without publishing on pull requests; publish a commit-tagged image to GHCR on pushes to `main`.
- `.github/workflows/environment-setup.yml`: manual preflight, dispatchable only from `main`; does not deploy or run migrations.
- `.github/workflows/deploy-staging.yml`: manual, staging-only workflow template. It requires a protected `staging` environment and a reviewed provider adapter with bearer-token authentication. No provider is configured, and no rollout is automatic.

## Layout

- `cmd/erp-api`: executable entry point.
- `internal/transport/httpapi`: HTTP handlers and process-level health endpoints.
- `api/openapi`: REST contract.
- `api/proto`: gRPC/Protobuf contract.
- `docs/architecture`: architectural decisions and boundaries.
- `docs/runbooks`: operational guidance.
- `migrations`: reserved home for reviewed database migrations.
- `Makefile`: local check, test, vet, format-check, build, and run targets.

The initial service intentionally exposes only health endpoints. Business-resource and action endpoints are contract-first, not yet implemented. OIDC authentication enforcement, PostgreSQL persistence, tenant authorization, migrations, and business workflows must not be inferred from configuration names or design documents. Do not treat this scaffold as production-ready ERP functionality.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

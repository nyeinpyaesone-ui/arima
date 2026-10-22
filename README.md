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
Requirements: Go 1.23+.

```sh
go test ./...
go run ./cmd/erp-api
curl -fsS http://localhost:8080/health/live
curl -fsS http://localhost:8080/health/ready
```

Set `ARIMA_ADDR` to change the listen address (default `:8080`).

## Layout
- `cmd/erp-api`: executable entry point.
- `internal/transport/httpapi`: HTTP handlers and health endpoints.
- `api/openapi`: REST contract.
- `api/proto`: gRPC/Protobuf contract.
- `docs/architecture`: architectural decisions and boundaries.
- `docs/runbooks`: operational guidance.
- `migrations`: reviewed database migration home.
- `.github/workflows/ci.yml`: formatting, tests, vet and build.

The initial service intentionally exposes only health endpoints. Business-resource and action endpoints are contract-first, not yet implemented. Do not treat this scaffold as production-ready ERP functionality.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
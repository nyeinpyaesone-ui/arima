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
go vet ./...
go run ./cmd/erp-api
curl -fsS http://localhost:8080/health/live
curl -fsS http://localhost:8080/health/ready
```

## Runtime configuration

The executable validates these environment variables before listening:

| Variable | Default |
| --- | --- |
| `ARIMA_ADDR` | `:8080` |
| `ARIMA_READ_HEADER_TIMEOUT` | `5s` |
| `ARIMA_READ_TIMEOUT` | `15s` |
| `ARIMA_WRITE_TIMEOUT` | `15s` |
| `ARIMA_IDLE_TIMEOUT` | `60s` |

Timeout values use Go duration syntax and must be greater than zero and no more than five minutes. Invalid values stop startup rather than silently falling back. See [runtime configuration runbook](docs/runbooks/runtime-configuration.md) and [environment example](configs/arima.env.example). The application does not automatically read .env files.

## Layout
- `cmd/erp-api`: executable entry point.
- `internal/config`: validated runtime configuration.
- `internal/transport/httpapi`: HTTP handlers and health endpoints.
- `api/openapi`: REST contract.
- `api/proto`: gRPC/Protobuf contract.
- `docs/architecture`: architectural decisions and boundaries.
- `docs/runbooks`: operational guidance.
- `migrations`: reviewed database migration home.
- `.github/workflows/ci.yml`: formatting, tests, vet and build.

The initial service intentionally exposes only health endpoints. Business-resource and action endpoints are contract-first, not yet implemented. Health readiness currently checks process-level behavior only; it does not validate external dependencies. Do not treat this scaffold as production-ready ERP functionality.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

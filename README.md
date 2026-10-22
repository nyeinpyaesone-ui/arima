# ARIMA Platform

**Go-first ERP core and AI integration bridge.** This repository establishes a production-oriented foundation for domain APIs, typed AI tool contracts, auditable actions, and reliable event delivery.

> Status: engineering foundation. The current executable is not a production ERP implementation.

## Principles
- Correctness before performance; explicit tenant and authorization boundaries.
- Business rules remain in the ERP core, never in the AI orchestrator.
- Mutations must be idempotent, audited, version-checked, and approval-gated by policy.
- Use a transactional outbox for reliable event publication; avoid dual writes.
- OpenAPI for HTTP and Protobuf for gRPC contracts.
- Require verified CI before merging.

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

| Variable | Default |
| --- | --- |
| `ARIMA_ENV` | development |
| `ARIMA_ADDR` | `:8080` |
| `ARIMA_READ_HEADER_TIMEOUT` | `5s` |
| `ARIMA_READ_TIMEOUT` | `15s` |
| `ARIMA_WRITE_TIMEOUT` | `15s` |
| `ARIMA_IDLE_TIMEOUT` | `60s` |

Timeouts use Go duration syntax and must be greater than zero and no more than five minutes. Invalid values stop startup. The application does not automatically load dotenv files. Set `ARIMA_ENV=production` in production; startup requires complete OIDC configuration and successful provider discovery.

## OIDC JWT authentication

Configure both values or leave both unset outside production:

- `ARIMA_OIDC_ISSUER`: exact OIDC issuer URL from provider metadata.
- `ARIMA_OIDC_AUDIENCE`: expected API audience.

The maintained `github.com/coreos/go-oidc/v3/oidc` verifier validates JWT signatures, issuer, audience, expiry and not-before against provider discovery and the provider's rotating JWKS. Unknown signing keys trigger the library's remote-key refresh behavior. Provider discovery is performed at startup with a timeout. The verified token must include non-empty `sub` and `tenant_id` claims.

When authentication is unconfigured outside production, `GET /api/v1/me` fails closed with 503. Missing/invalid credentials return 401. The issuer and audience must match the actual identity provider; a configuration example is not a substitute for a provider-specific integration test. See [authentication boundary](docs/security/authentication-boundary.md), [runtime runbook](docs/runbooks/runtime-configuration.md), and [environment example](configs/arima.env.example).

## Current implementation boundary

Implemented: health probes, validated HTTP runtime settings, OIDC JWT verification, bearer middleware, a protected identity endpoint, and a production startup gate requiring OIDC configuration. Health readiness is process-level only; it does not check external dependencies.

Implemented foundation: versioned PostgreSQL migration for tenant-scoped records, forced row-level security (RLS), audit events, idempotency keys and outbox events; a tenant-scoped transaction repository; optimistic record version checks; and PostgreSQL-backed integration tests in CI. The application does not run migrations automatically.

Still not implemented: business resource/action routes, role/policy authorization, an outbox publisher/worker, operational retention/replay policies, and end-to-end business workflows that use the repository transaction to atomically persist a mutation, audit event, idempotency response and outbox event. OpenAPI contracts for planned endpoints do not mean those routes are live. Do not expose this scaffold as production ERP functionality.

## Layout
- `cmd/erp-api`: executable entry point.
- `internal/auth`: bearer middleware and OIDC JWT verification.
- `internal/store`: PostgreSQL connection and tenant-scoped transaction repository.
- `internal/config`: validated runtime configuration.
- `internal/transport/httpapi`: health and protected identity endpoints.
- `api/openapi`: REST contract.
- `api/proto`: gRPC/Protobuf contract.
- `docs/architecture`: architecture decisions and boundaries.
- `docs/runbooks`: operational guidance.
- `migrations`: reviewed database migration home.
- `.github/workflows/ci.yml`: formatting, tests, vet and build.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

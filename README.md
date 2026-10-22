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

Timeouts use Go duration syntax and must be greater than zero and no more than five minutes. Invalid values stop startup. The application does not automatically load dotenv files. Set `ARIMA_ENV=production` in production; startup then requires complete authentication configuration.

## Optional OAuth token introspection

Configure all five variables or leave all unset outside production:

- `ARIMA_OIDC_INTROSPECTION_URL`: HTTPS introspection endpoint.
- `ARIMA_OIDC_ISSUER`: exact issuer claim.
- `ARIMA_OIDC_AUDIENCE`: expected API audience.
- `ARIMA_OIDC_CLIENT_ID`: introspection client ID.
- `ARIMA_OIDC_CLIENT_SECRET`: secret-manager value only; never commit it.

The identity provider must validate token signature and active/revocation status and return `active`, `iss`, `aud`, `exp`, `sub`, and `tenant_id`. When auth is unconfigured, `GET /api/v1/me` fails closed with 503. With auth configured, missing/invalid credentials return 401. See [authentication boundary](docs/security/authentication-boundary.md), [runtime runbook](docs/runbooks/runtime-configuration.md), and [environment example](configs/arima.env.example).

## Current implementation boundary

Implemented: health probes, validated HTTP runtime settings, optional token introspection, a protected identity endpoint, and a production startup gate requiring authentication. Health readiness is process-level only; it does not check external dependencies.

Not implemented: database persistence, tenant-isolation enforcement for stored records, role/policy authorization, business resource/action routes, audit persistence, idempotency storage, and transactional outbox. OpenAPI contracts for planned endpoints do not mean those routes are live. Do not expose this scaffold as production ERP functionality.

## Layout
- `cmd/erp-api`: executable entry point.
- `internal/auth`: bearer middleware and token introspection.
- `internal/config`: validated runtime configuration.
- `internal/transport/httpapi`: health and protected identity endpoints.
- `api/openapi`: REST contract.
- `api/proto`: gRPC/Protobuf contract.
- `docs/architecture`: architecture decisions and boundaries.
- `docs/runbooks`: operational guidance.
- `migrations`: reviewed database migration home.
- `.github/workflows/ci.yml`: formatting, tests, vet and build.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

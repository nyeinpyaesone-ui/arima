# ARIMA Platform

**Go-first ERP core and AI integration bridge.** The repository is an evolving engineering foundation, not a complete production ERP application.

## Current implemented scope

- Go HTTP API with OIDC JWT verification, bearer authentication, graceful shutdown and dependency readiness.
- PostgreSQL migrations for tenant records, forced RLS, audit events, idempotency, transactional outbox, worker leases, tenant roles, permissions and memberships.
- Customer create/read API with permission checks inside tenant transactions.
- Customer creation atomically commits the customer record, audit event, idempotency result and outbox event.
- Standalone outbox worker with bounded tenant pagination, concurrent-safe leases, delivery timeout, authenticated HTTPS webhook publisher, retry and dead-letter handling.
- Multi-architecture container image builds for Linux AMD64 and ARM64; the image contains both API and worker binaries.

The system is not yet production-ready. A real OIDC provider, tenant provisioning process, webhook receiver, hosting provider, production database roles, worker monitoring and staging rollback still require environment-specific implementation and verification. Product modules such as inventory, sales, purchasing, accounting, payments, branches, reporting and approval workflows remain future implementation work.

## Quick start

Requirements: Go 1.26.11 and PostgreSQL 15+ for persistence.

```sh
go mod download
go mod verify
go test ./...
go vet ./...
go run ./cmd/erp-api
```

Without OIDC configuration, protected routes fail closed with HTTP 503; health endpoints remain available. For customer API operations, apply migrations 0001, 0002 and 0003 to a disposable development database, configure ARIMA_DATABASE_URL, and provision a tenant membership with the operator runbook.

## API routes

| Method | Path | Required permission | Purpose |
| --- | --- | --- | --- |
| GET | /health/live | None | Process liveness |
| GET | /health/ready | None | API and configured PostgreSQL readiness |
| GET | /api/v1/me | Valid OIDC access token | Return verified subject and tenant |
| POST | /api/v1/customers | customer.write | Create customer with Idempotency-Key |
| GET | /api/v1/customers/{externalID} | customer.read | Read customer within caller's tenant |

Customer create accepts JSON fields external_id, name, phone, email and notes. Request bodies are limited to 64 KiB. Unknown fields are rejected. Idempotency keys must be 8–200 characters. A repeated key with the same normalized request returns the stored response; reuse with a different request returns 409.

## Runtime configuration

API variables:

- ARIMA_ENV: set to production to enforce OIDC and verified PostgreSQL TLS.
- ARIMA_ADDR: default :8080.
- ARIMA_OIDC_ISSUER and ARIMA_OIDC_AUDIENCE: both required together; production startup fails if either is missing or provider discovery fails.
- ARIMA_DATABASE_URL: PostgreSQL URL. Production requires sslmode=verify-full.
- ARIMA_READ_HEADER_TIMEOUT, ARIMA_READ_TIMEOUT, ARIMA_WRITE_TIMEOUT and ARIMA_IDLE_TIMEOUT: validated Go durations.

The OIDC verifier uses provider discovery and JWKS through the maintained go-oidc library. The verified token must include non-empty sub and tenant_id claims. The application does not load dotenv files automatically.

## Database and tenant access

Apply the reviewed migrations in order:

1. migrations/0001_tenant_foundation.up.sql
2. migrations/0002_outbox_leases.up.sql
3. migrations/0003_tenant_authorization.up.sql

The API never runs migrations at startup. Use a dedicated migration identity. API and worker runtime identities must be NOSUPERUSER and NOBYPASSRLS. The API role requires read-only access to tenant role/membership tables and only required business DML. The worker role requires SELECT on tenants and SELECT/UPDATE on outbox_events; it must not receive business-record or membership administration access.

Provision roles, permissions and membership through the trusted operator process in [tenant access provisioning](docs/runbooks/tenant-access-provisioning.md). Never accept role names or permissions from client-controlled token claims or request bodies.

## Outbox worker

The same container image includes /erp-api and /outbox-worker. The worker process is started by overriding the container command to /outbox-worker.

Required worker environment:

- ARIMA_OUTBOX_DATABASE_URL: dedicated PostgreSQL worker identity; production requires sslmode=verify-full.
- ARIMA_OUTBOX_WEBHOOK_URL: HTTPS endpoint for the event receiver.
- ARIMA_OUTBOX_WEBHOOK_TOKEN: bearer token stored in the hosting provider's secret manager.

The webhook receiver must durably accept events and deduplicate using the event ID supplied in Idempotency-Key and X-ARIMA-Event-ID. Delivery is at-least-once, not exactly-once. 2xx means durable acceptance; 429/5xx are retried, other non-2xx responses are dead-lettered. Do not log tokens or full event payloads.

## CI and release boundary

PR CI runs workflow lint, module integrity/tidy checks, formatting, vet, vulnerability scanning, race-enabled PostgreSQL integration tests and a Linux binary build. Container validation builds both Linux AMD64 and ARM64 images without publishing on pull requests. GHCR publishing is limited to pushes on main. Staging deployment is manually dispatched from main and requires a protected staging environment and a reviewed provider adapter. No production deployment is configured by this baseline.

## Documentation

- [Authentication boundary](docs/security/authentication-boundary.md)
- [Development workflow](docs/development-workflow.md)
- [Database migrations](migrations/README.md)
- [Tenant access provisioning](docs/runbooks/tenant-access-provisioning.md)
- [Staging deployment](docs/runbooks/staging-deployment.md)
- [OpenAPI contract](api/openapi/openapi.yaml)

## Layout

- cmd/erp-api: API executable.
- cmd/outbox-worker: standalone event worker executable.
- internal/auth: bearer and OIDC authentication plus permission evaluation.
- internal/customer: transactional customer service.
- internal/outbox: worker scheduler and HTTPS webhook publisher.
- internal/store: PostgreSQL tenant repository, RLS-aware transactions and outbox lease operations.
- internal/transport/httpapi: HTTP routes and request validation.
- migrations: reviewed SQL migrations.
- .github/workflows: CI, container validation/publishing, release and guarded staging workflows.

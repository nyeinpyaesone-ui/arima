# ARIMA local implementation and verification

## Runtime baseline

The current runtime is Go 1.26.11, PostgreSQL 15+, a separate Go outbox worker, and a React 19 / Vite frontend on Node.js 22+. The root Dockerfile builds both Go processes into one non-root image. The frontend is built separately and served through the checked-in Nginx template.

Do not add FastAPI, Celery, Redis, Next.js, or Supabase: they are not dependencies of the current main-branch runtime.

## One-command local setup

Run from the repository root:

    chmod +x scripts/arima-setup.sh
    ./scripts/arima-setup.sh all

The script checks the pinned toolchain, creates .env from the local example only when missing, verifies Git ignores it, validates Go and frontend builds/tests, validates Compose, starts PostgreSQL/migrations/API, and checks health plus database role boundaries.

It does not run remote migrations, create cloud resources, deploy, or delete the local database volume. PostgreSQL-backed integration tests in GitHub CI remain authoritative; local tests may skip integration tests when dedicated test database variables are absent.

## Commands

| Command | Result |
|---|---|
| preflight | Check Go 1.26.11, Node.js 22+, Docker Compose v2, curl and jq |
| verify | Run Go, frontend, shell and Compose checks |
| up | Start local PostgreSQL, migrations and API |
| worker | Start worker only after a real HTTPS receiver is configured |
| status | Show Compose state and API readiness |
| down | Stop services without deleting the database volume |
| all | Verify, then start the local API stack |

The script never removes Docker volumes. Remove local data manually only after confirming the local database is disposable.

## Local endpoints

| Process | Method | Path | Authentication | Meaning |
|---|---|---|---|---|
| API | GET | /health/live | None | Process liveness |
| API | GET | /health/ready | None | Process readiness and configured PostgreSQL connectivity |
| API | GET | /api/v1/me | OIDC bearer token | Verified subject and tenant identity |
| API | POST | /api/v1/customers | OIDC bearer token + Idempotency-Key | Tenant-scoped customer create |
| API | GET | /api/v1/customers/{externalID} | OIDC bearer token | Tenant-scoped customer read |
| Worker | GET | /health/live | Private network | Worker process liveness |
| Worker | GET | /health/ready | Private network | Worker database connectivity |

Default local addresses:
- API: http://127.0.0.1:8080
- PostgreSQL: 127.0.0.1:5432
- Worker health: http://127.0.0.1:8081, only when configured and started

Worker readiness does not prove webhook delivery. The receiver must durably accept events, authenticate its bearer token, and deduplicate by event ID. Never expose worker health publicly.

## Customer API request

    curl --fail-with-body --request POST \
      http://127.0.0.1:8080/api/v1/customers \
      --header "Authorization: Bearer $ARIMA_DEV_ACCESS_TOKEN" \
      --header "Content-Type: application/json" \
      --header "Idempotency-Key: customer-create-000001" \
      --data '{"external_id":"CUST-0001","name":"Example Customer"}'

ARIMA_DEV_ACCESS_TOKEN must be a valid access token from a real development OIDC provider. Configure the exact issuer and audience in .env and provision the matching subject/tenant membership through the trusted operator process. Do not fabricate claims or use a production identity.

## Environment variables

API: ARIMA_ENV, ARIMA_ADDR, ARIMA_OIDC_ISSUER, ARIMA_OIDC_AUDIENCE, ARIMA_DATABASE_URL, ARIMA_READ_HEADER_TIMEOUT, ARIMA_READ_TIMEOUT, ARIMA_WRITE_TIMEOUT, ARIMA_IDLE_TIMEOUT.

Worker: ARIMA_OUTBOX_DATABASE_URL, ARIMA_OUTBOX_WEBHOOK_URL, ARIMA_OUTBOX_WEBHOOK_TOKEN, ARIMA_OUTBOX_HEALTH_ADDR.

Production PostgreSQL URLs require sslmode=verify-full. Store runtime credentials only in the provider secret manager. Keep worker webhook credentials out of the API service.

## GitHub Actions and release gates

- CI runs Go checks, PostgreSQL-backed tenant/outbox tests, Compose smoke tests, actionlint and vulnerability scanning.
- Web CI installs from web/package-lock.json, type-checks/builds the SPA, and exercises the Nginx-to-Go API proxy.
- Container Image validates Linux AMD64/ARM64 builds on PRs and publishes a commit-tagged GHCR image on main.
- Staging deployment must be manual, explicitly acknowledged, protected by the GitHub staging Environment, and use a trusted provider adapter, exact image digest verification and API/worker readiness. It must not run production migrations or promote to production.

The staging adapter and provider-side secret store are not yet evidenced as operational. Do not dispatch staging deployment until the adapter, aggregate health contract, private worker readiness, GHCR pull permissions, PostgreSQL TLS/roles/backups and rollback rehearsal are verified.

## Implemented scope versus planned contract

Implemented: health probes, authenticated identity, tenant-authorized customer create/read, PostgreSQL tenant isolation, audit/idempotency/outbox transaction handling, and a separate webhook outbox worker.

Generic ERP resource and action validation/proposal/approval/execution paths in OpenAPI are planned contracts, not implemented endpoints. Inventory, sales, purchasing, accounting, payments, branches, reporting and approval workflows are not yet a complete product implementation.

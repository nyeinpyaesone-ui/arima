# Development flow and environment setup

## Current implementation status

ARIMA now has a Go HTTP API scaffold with OIDC JWT verification, PostgreSQL-backed tenant repository primitives, forced row-level security, migrations, audit/idempotency/outbox foundations, and a leased outbox worker package. CI runs PostgreSQL 15 integration tests using a non-superuser runtime role.

This is still not a complete ERP application. Production business mutation routes, complete role/policy and tenant-membership authorization, a concrete outbox publisher, trusted tenant scheduler, and end-to-end business workflows are not implemented. The worker package is not wired into the API executable. A real OIDC provider and production database roles must be configured and verified in the target environment.

The staging workflow is a guarded provider-adapter template. No live staging or production rollout has been performed.

## Branch and pull-request flow

1. Branch from current `main`: `feat/<short-name>`, `fix/<short-name>`, or `chore/<short-name>`.
2. Keep changes focused; update tests, API contracts, migrations, and runbooks together.
3. Run `go mod verify`, `make check`, and `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`.
4. Open a PR to `main`; do not merge while CI/security checks fail or review is outstanding.
5. PR CI is the authoritative code-validation path. It includes PostgreSQL 15 integration tests. Environment Setup Preflight is supplemental and reads protected GitHub Environment configuration only from trusted `main`.
6. Release only from a reviewed, green commit using the release workflow and a version tag. Release publication is not deployment.

## Local environment

Prerequisites: Go 1.26.11, Git, and optionally Docker/Compose. PostgreSQL 15 is required for full integration testing.

```sh
export ARIMA_ADDR=127.0.0.1:8080
go mod download
go mod verify
make check
go run ./cmd/erp-api
```

For full database integration tests, provision an isolated database named `arima_test` and set both `ARIMA_TEST_DATABASE_URL` and `ARIMA_TEST_RUNTIME_DATABASE_URL`. Never point test variables at staging or production. The runtime test identity must be a non-superuser without `BYPASSRLS`.

The application does not automatically load dotenv files. Export supported local values explicitly or use a local secret manager. Never put production credentials in files, logs, command-line arguments, issue comments, or repository content.

## GitHub Environment configuration

Create GitHub Environments named `development`, `staging`, and `production` only as needed.

- `development`: use non-production credentials and isolated databases.
- `staging` and `production`: configure variables `ARIMA_OIDC_ISSUER` and `ARIMA_OIDC_AUDIENCE`, plus secret `ARIMA_DATABASE_URL`.
- Production database DSNs must use TLS certificate and hostname verification (`sslmode=verify-full`).
- Use separate migration and runtime database identities. The runtime identity must not be a superuser or have `BYPASSRLS`.
- Require reviewers and restrict deployment branches/tags for protected environments before enabling any live deployment.

The Environment Setup Preflight checks configuration presence/syntax and code-level gates; it does not connect to the production database or prove provider token issuance, tenant authorization, or business correctness. Run it from Actions → Environment Setup Preflight → Run workflow on `main`. Feature branches are validated by PR CI.

## Migration and deployment boundary

- The API never applies schema migrations at startup. Apply reviewed migrations in order with a dedicated migration identity.
- The staging workflow requires an authenticated provider adapter and protected staging environment. The adapter must allow-list this repository/image, deploy the exact digest, and retain a rollback path.
- The staging workflow does not run migrations or promote to production.
- Use private database networking where possible, TLS verification, firewall allow-lists, least-privilege roles, connection limits, backups, and tested restore procedures.
- Do not automatically apply destructive down migrations. Back up and obtain explicit approval before any rollback that deletes tenant/audit/outbox data.
- Prefer GitHub OIDC with narrowly scoped cloud permissions when the selected target supports it. Otherwise use dedicated restricted identities and rotate credentials.
- Production deployment must use a protected `production` Environment, least-privilege permissions, a reviewed immutable artifact, health checks, and a documented rollback. Never deploy untrusted pull-request code.

## Missing essentials before production use

1. A real OIDC provider integration test suite and tenant-membership authorization policy.
2. Business services/routes with atomic mutation + audit + idempotency + outbox behavior.
3. A concrete idempotent outbox publisher, trusted tenant scheduler, timeout policy, metrics and alerts.
4. Staging verification of actual runtime/migration/worker grants, RLS, migrations, backup and restore.
5. A selected hosting provider, authenticated adapter, protected staging rollout and rehearsed rollback.
6. End-to-end business tests, operational SLOs, observability, capacity/load testing, and a tested production rollout plan.

Do not call ARIMA production-ready until these capabilities are implemented and evidenced by passing checks and a staging rehearsal.

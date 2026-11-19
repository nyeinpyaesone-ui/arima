# Development flow and environment setup

## Current implementation status

ARIMA provides a Go HTTP API with OIDC JWT verification, PostgreSQL tenant storage, forced RLS, tenant-local roles and permissions, customer create/read routes, audit/idempotency/outbox transaction handling, and a standalone outbox worker with bounded scheduling and an authenticated HTTPS webhook publisher.

The customer create operation commits the customer record, audit event, idempotency response and outbox event atomically. The worker runs as a separate process from the same container image. The webhook receiver, a real OIDC provider, hosting provider, production secret store, worker delivery monitoring and rehearsed staging rollback still require environment-specific configuration and verification.

ARIMA is not a complete ERP application. Inventory, sales, purchasing, accounting, payments, branches, reporting, approvals and other domain workflows remain outside the implemented scope. No live staging or production rollout has been performed.

## Branch and pull-request flow

1. Branch from current `main`: `feat/<short-name>`, `fix/<short-name>`, or `chore/<short-name>`.
2. Keep changes focused; update tests, API contracts, migrations and runbooks together.
3. Run `go mod verify`, `make check`, and `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`.
4. Open a PR to `main`; do not merge while CI/security checks fail or review is outstanding.
5. PR CI is the authoritative code-validation path. It includes isolated PostgreSQL 15 tests for tenant controls and customer workflows, plus Compose and bootstrap-script validation.
6. Release only from a reviewed, green commit using the release workflow and a version tag. Release publication is not deployment.

## Local environment

Prerequisites: Go 1.26.11, Git, Docker Engine and the Docker Compose plugin.

```sh
cp configs/arima.env.example .env
make dev-config
make dev-up
curl --fail --silent --show-error http://127.0.0.1:8080/health/live
curl --fail --silent --show-error http://127.0.0.1:8080/health/ready
```

Compose starts PostgreSQL 15, applies migrations 0001–0003 with a local migration ledger, grants separate local-only API and worker roles, and starts the API. The database volume survives `make dev-down`; do not use `docker compose down -v` unless you intend to delete local data.

Without a configured OIDC provider, protected customer routes fail closed. To exercise them, configure a real development issuer/audience and provision the exact `sub` and `tenant_id` membership through the trusted operator process. Never manufacture a subject or role from request input.

To run the worker, set a real HTTPS event receiver and its token in the ignored local `.env`, then run `make dev-worker`. The receiver must durably accept and deduplicate event IDs. Do not start the worker with a placeholder or unconfigured receiver. The worker listens on local port 8081 for health probes; keep this endpoint private outside development.

The application does not load dotenv files itself. Compose reads the root `.env`; the process receives only explicitly configured environment variables. Never put production credentials in tracked files, command-line arguments, logs or issue comments.

## GitHub Environment configuration

The **Environment Setup Preflight** workflow runs only from trusted `main`. For `staging` and `production`, configure the non-secret variables `ARIMA_OIDC_ISSUER` and `ARIMA_OIDC_AUDIENCE` so the preflight can check their presence and HTTPS issuer syntax.

For the **staging** GitHub Environment, configure only the deployment-control values:
- Secret `STAGING_DEPLOY_HOOK_URL`.
- Secret `STAGING_DEPLOY_TOKEN`.
- Variable `STAGING_HEALTHCHECK_URL` for the public API readiness endpoint.

Runtime values and credentials (`ARIMA_DATABASE_URL`, `ARIMA_OUTBOX_DATABASE_URL`, `ARIMA_OUTBOX_WEBHOOK_URL`, `ARIMA_OUTBOX_WEBHOOK_TOKEN`) belong in the hosting provider's secret manager, not in the GitHub Actions runner or deployment payload. The provider adapter must resolve these names from the provider-side store.

Require reviewers and restrict deployment branches for protected environments before any live staging or production rollout. The preflight uses temporary isolated CI PostgreSQL databases; it does not connect to target databases, validate provider-side secrets, deploy, or apply remote migrations.

## Migration and deployment boundary

- The API and worker never apply DDL at startup. Apply migrations 0001, 0002 and 0003 as a separate reviewed operation using a dedicated migration identity.
- Runtime API and worker identities must be `NOSUPERUSER NOBYPASSRLS`. Keep their database grants separate and verify actual catalog privileges in staging.
- Production PostgreSQL connections must use TLS certificate and hostname verification (`sslmode=verify-full`).
- The staging workflow requires an authenticated provider adapter and a protected GitHub Environment. It sends an immutable image digest and the required environment-variable names, not secret values.
- The adapter must launch API and worker with separate entrypoints, resolve runtime values from the provider secret manager, and verify both readiness endpoints from inside the provider network. Keep worker port 8081 private.
- The staging workflow does not run migrations or promote to production.
- Use private database networking where possible, firewall allow-lists, connection limits, backups and tested restore procedures.
- Do not automatically apply destructive down migrations. Back up and obtain explicit approval before any rollback that deletes tenant/audit/outbox data.
- Production deployment must use a separate protected environment, least-privilege permissions, a reviewed immutable artifact, health checks and a documented rollback. Never deploy untrusted pull-request code.

## Remaining production gates

1. Configure and test a real OIDC provider, including signing-key rotation and invalid/expired token cases.
2. Select a hosting provider and implement/review the authenticated staging adapter.
3. Provision real migration/API/worker database roles; verify grants, RLS, TLS, backups and restore in staging.
4. Configure a durable webhook receiver with event-ID idempotency, plus worker delivery/lag/dead-letter metrics and alerts.
5. Rehearse staging rollout and rollback using the exact image digest.
6. Complete remaining ERP domains, end-to-end workflows, observability, SLOs and capacity/load testing.

Do not call ARIMA production-ready until these gates are evidenced by successful integration checks and a reviewed staging rehearsal.

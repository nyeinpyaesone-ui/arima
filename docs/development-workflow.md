# Development flow and environment setup

## Current implementation status

ARIMA currently provides a minimal Go HTTP API, process-level health endpoints, a container build, CI checks, release artifacts, and a GHCR image workflow. Treat this repository as a **service scaffold**, not a production ERP backend.

The following capabilities are not demonstrated by the current runtime scaffold and must not be inferred from configuration names or documentation: OIDC authentication enforcement, authorization and tenant isolation, PostgreSQL persistence, database migrations, outbox processing, and business workflows. The readiness endpoint is process-level only. Do not describe the application as production-ready until those integrations and their tests exist.

A managed-container provider has not been selected. The staging workflow is a guarded adapter template; no live staging rollout has been performed.

## Branch and pull-request flow

1. Branch from the current default branch: `feat/<short-name>`, `fix/<short-name>`, or `chore/<short-name>`.
2. Keep a change focused; update tests, API contracts, migrations, and docs together when those capabilities are implemented.
3. Run `make check`, `go mod verify`, and `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`.
4. Open a PR to `main`; do not merge while CI/security checks fail or the change lacks review.
5. PR CI is the required code-validation path. Environment Setup Preflight is supplemental and must be dispatched from the trusted `main` branch because it reads environment configuration.
6. Release only from a reviewed, green commit using the existing release workflow and a version tag. Release publication is not deployment.

## Local environment

Prerequisites: Go 1.26.10 (or the supported patched Go 1.26 release), Git, and optionally Docker/Compose for container smoke testing. PostgreSQL is not required by the current scaffold; add PostgreSQL to local test prerequisites only after persistence and integration tests are implemented.

```sh
export ARIMA_ADDR=127.0.0.1:8080
go mod download
go mod verify
make check
go run ./cmd/erp-api
```

The application does not automatically load dotenv files. Export supported local values explicitly or use a local secret manager. Never put production credentials in files, logs, command-line arguments, issue comments, or repository content.

## GitHub Environment configuration

Create GitHub Environments named `development`, `staging`, and `production` only as needed.

- `development`: no production secrets.
- `staging` and `production`: the manual preflight can check planned variables `ARIMA_OIDC_ISSUER` and `ARIMA_OIDC_AUDIENCE`. The issuer must use HTTPS.
- Do **not** configure `ARIMA_DATABASE_URL` for this scaffold: no runtime persistence or migration code currently consumes it.
- Require reviewers and restrict branches/tags for protected environments before any real deployment workflow is enabled.

The OIDC preflight checks variable presence and issuer syntax only. It does **not** prove that the application validates tokens, audience, signing-key rotation, expiry, authorization, or tenant claims. Those checks are not implemented by this scaffold.

Run Actions → Environment Setup Preflight → Run workflow from `main` and select the environment. Feature branches are validated by pull-request CI instead. The preflight runs workflow syntax, module integrity, formatting, vet, vulnerability, race-test, and build gates; it performs no deployment, migration, or release and does not verify database connectivity.

## Migration and deployment boundary

- Select and review a managed-container provider before using the staging adapter. Configure the protected `staging` environment and its adapter URL, bearer-token secret, health-check variable, and reviewer gate.
- The adapter must authenticate the `Authorization: Bearer` token, allow-list this repository/image namespace, reject non-staging targets, deploy the exact supplied digest, and preserve a rollback path. HTTPS alone is not caller authentication.
- Establish the hosting target, registry/image digest policy, network/TLS ingress, secret delivery, health-check semantics, rollout strategy, and rollback mechanism before a live staging rollout.
- When persistence is implemented, test migrations against disposable PostgreSQL first. Use a dedicated migration identity; do not grant the runtime role superuser or `BYPASSRLS`.
- Do not automatically apply destructive down migrations. Back up and obtain explicit approval before any rollback that deletes tenant/audit/outbox data.
- Prefer GitHub OIDC with narrowly scoped cloud permissions when the selected target supports it. For other mechanisms, use dedicated, restricted identities and rotate credentials.
- A production job must use a protected `production` Environment, least-privilege permissions, a reviewed immutable artifact, health/readiness checks, and a documented rollback. It must not run on untrusted pull-request code.

## Missing essentials before production deployment

1. A chosen hosting/deployment target and repeatable infrastructure definition.
2. A reviewed authenticated provider adapter and a rehearsed staging rollout/rollback.
3. Protected GitHub Environments, approved secret handling, immutable image promotion, and provenance verification.
4. OIDC authentication, authorization policy, and tenant isolation with positive and negative tests.
5. PostgreSQL persistence, schema migration, backup/restore, connection limits, and database-aware readiness.
6. Outbox delivery and end-to-end transactional business workflows.
7. Observability: structured/redacted logs, metrics, traces, alert thresholds, and an on-call owner.
8. Deployment smoke/E2E tests, SLOs, capacity/load testing, and a tested rollback path.
9. Dependency and container remediation gates with SBOM/provenance checks and release approval.

Do not call ARIMA production-ready until these capabilities are implemented and evidenced by passing checks and a staging rehearsal.

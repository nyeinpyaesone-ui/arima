# Development flow and environment setup

## Current scope

ARIMA is a Go API with PostgreSQL tenant-scoped storage, OIDC verification, OpenAPI and Protobuf contracts, a container build, CI, and a release-artifact workflow. The repository does **not** yet identify an approved hosting target, registry, DNS/TLS ingress, or production deployment identity. The environment preflight workflow is therefore deliberately non-deploying.

## Branch and pull-request flow

1. Branch from the current default branch: `feat/<short-name>`, `fix/<short-name>`, or `chore/<short-name>`.
2. Keep a change focused; update tests, OpenAPI/Protobuf contracts, migrations, and docs together when applicable.
3. Run `make check`, `go mod verify`, and `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`.
4. Open a PR to `main`; do not merge while CI/security checks fail or the change lacks review.
5. Use the existing CI as the required merge gate. The manual Environment Setup Preflight is an additional diagnostic, not a substitute for PR CI.
6. Release only from a reviewed, green commit using the existing release workflow and a version tag. Release publication is not deployment.

## Local environment

Prerequisites: Go 1.26.10 or newer patched Go 1.26 release, PostgreSQL 15 for integration tests, Git, and optionally Docker/Compose for container smoke testing.

```sh
cp configs/arima.env.example .env.local
# Edit only non-secret local values; do not commit .env.local.
export ARIMA_ENV=development
export ARIMA_ADDR=127.0.0.1:8080
go mod download
go mod verify
make check
go run ./cmd/erp-api
```

The application does not load dotenv files automatically. Export variables explicitly or use a local secret manager. Never put production credentials in `.env.local`, logs, command-line arguments, issue comments, or repository files.

## GitHub Environment configuration

Create GitHub Environments named `development`, `staging`, and `production` in repository Settings → Environments. Configure only what is needed:

- `development`: no production secrets.
- `staging`: variables `ARIMA_OIDC_ISSUER`, `ARIMA_OIDC_AUDIENCE`; secret `ARIMA_DATABASE_URL`.
- `production`: same variable/secret names, with production-specific values; require reviewers and restrict deployment branches/tags before any deployment workflow is introduced.

Use the exact issuer from the identity provider's OIDC metadata. The database secret must use TLS verification appropriate to the selected provider (prefer `sslmode=verify-full` with a trusted CA). The preflight validates configuration presence and syntax only; it does not prove database reachability, OIDC token semantics, authorization correctness, or readiness for production.

Run Actions → Environment Setup Preflight → Run workflow and select an environment. This workflow runs syntax, module, formatting, vet, vulnerability, race-test, and build gates. It does **not** deploy, run migrations against a real environment, or publish a release.

## Migration and deployment boundary

- Test migrations against disposable PostgreSQL 15 before staging. Use a dedicated migration identity; never grant the runtime role superuser or `BYPASSRLS`.
- Do not automatically apply destructive down migrations. Back up and obtain explicit approval before any rollback that deletes tenant/audit/outbox data.
- Before adding a real deploy workflow, decide the target (for example, a named VPS with SSH/OIDC, a container platform, or Kubernetes), registry, immutable image reference, network/TLS arrangement, secret delivery, health-check path, rollout strategy, and rollback mechanism.
- Prefer GitHub OIDC with narrowly scoped cloud permissions over long-lived cloud keys where the target supports it. For SSH-only targets, use a dedicated deploy account, pinned host keys, and restricted credentials.
- A production job must use the protected `production` Environment, least-privilege permissions, a reviewed immutable artifact, health/readiness checks, and a documented rollback. It must not run on untrusted pull-request code.

## Missing essentials before production deployment

1. A chosen hosting/deployment target and repeatable infrastructure definition.
2. Registry and immutable image digest promotion policy; current release workflow produces artifacts, not a running service.
3. Protected GitHub Environments and approved secret/OIDC configuration.
4. Live staging validation of OIDC issuer, audience, signing-key rotation, outage, expiry, and tenant claim handling.
5. Authorization policy and business routes; current runtime runbook states these are incomplete.
6. Outbox delivery worker and end-to-end transactional business workflows.
7. Observability: structured/redacted logs, metrics, traces, alert thresholds, and a defined on-call owner.
8. Backup/restore and migration rollback drills, including tenant isolation and data retention.
9. Deployment smoke/E2E tests, SLOs, capacity/load testing, and a tested rollback path.
10. Supply-chain policy for pinned actions/toolchain, dependency remediation, SBOM/provenance verification, and release approval.

Do not call ARIMA production-ready until these are implemented and evidenced by passing checks and a staging rehearsal.

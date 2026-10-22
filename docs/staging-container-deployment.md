# Staging container deployment

## What this workflow does

`.github/workflows/staging-deploy.yml` is a manually triggered, staging-only pipeline. It must be dispatched from `main`. It builds the existing root `Dockerfile` for Linux amd64 and arm64, publishes an immutable commit-tagged image to GHCR, requests a rollout of that exact image digest through a provider adapter, then polls the existing `/health/ready` endpoint. It never deploys production or applies database migrations.

## Repository compatibility verified

- Go API entry point: `cmd/erp-api`; default listen address `:8080`.
- Existing multi-stage root `Dockerfile` emits a non-root distroless image and exposes port 8080.
- Existing health endpoints: `/health/live` and `/health/ready`.
- Current app health handlers report process-level status; readiness does not prove database connectivity or application-level readiness.
- CI currently tests, vets, formats, and builds the Go API. It does not yet build/test the container image as part of PR CI.
- PR #3 adds environment preflight and development-flow docs, not a deployment integration.

## Required staging setup

Create a GitHub Environment named `staging`. Keep deployment secrets scoped to that environment.

- Secret `STAGING_DEPLOY_HOOK_URL`: HTTPS endpoint for a **trusted deployment adapter** that accepts a POST JSON payload with `image`, `digest`, `commit`, and `environment`, and deploys the exact digest. A generic provider deploy hook is not automatically compatible with this contract; implement/configure an adapter for the chosen vendor before enabling this workflow.
- Variable `STAGING_HEALTHCHECK_URL`: HTTPS URL ending in `/health/ready`, reachable from GitHub-hosted runners.
- Ensure the container platform can pull private `ghcr.io/nyeinpyaesone-ui/arima` images, or configure package visibility intentionally.
- Configure the container service to listen on port 8080 and inject `ARIMA_ADDR=:8080` (or the platform's supported port mapping).

## PostgreSQL hosted on the deployment host

The database URL belongs in the managed platform's secret/configuration store as `ARIMA_DATABASE_URL`; it must not be copied into GitHub workflow variables or deployment payloads. Before rollout, establish and test a private network path from the managed container runtime to the PostgreSQL host (private networking, VPN, or a tightly restricted TLS endpoint). Use TLS certificate verification, firewall allow-listing, a dedicated least-privilege runtime role, and a separate migration identity. Do not expose PostgreSQL broadly to the public internet. The current service scaffold's readiness endpoint does not test database connectivity, so database connectivity and migrations must be validated separately before claiming staging readiness.

## Safe rollout contract

The adapter must:
1. Authenticate requests and accept only the staging environment.
2. Validate the repository/image allow-list and deploy the supplied immutable digest, not a mutable tag.
3. Keep the prior known-good revision and expose a rollback operation.
4. Return success only after the platform accepts the exact digest.
5. Avoid logging the hook URL, credentials, or database DSN.

The workflow currently sends no production credentials, grants no cloud-wide permissions, and has no production environment/job. It uses `GITHUB_TOKEN` only to publish the image to GHCR. The configured adapter must be reviewed and tested before the manual workflow is used.

## Remaining readiness blockers

- Name the managed container provider and implement its authenticated adapter/API contract.
- Confirm GHCR package permissions and provider private-image pull credentials.
- Validate PostgreSQL host routing, TLS, firewall, backup/restore, migration execution and restore drills.
- Add DB-aware readiness only when it can be implemented safely and tested; current `/health/ready` is process-level only.
- Remediate any dependency vulnerabilities and add image vulnerability scanning/signature verification before production consideration.
- Add end-to-end deployment tests, rollback rehearsal, alerting, and evidence from a successful staging rehearsal.

Production deployment remains intentionally out of scope.

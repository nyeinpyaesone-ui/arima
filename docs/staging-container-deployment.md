# Staging container deployment

## Current status

The repository contains a **staging deployment workflow contract**, not a completed provider integration. `.github/workflows/staging-deploy.yml` is manually dispatched from `main`, requires an explicit acknowledgement, builds the existing root Dockerfile for Linux AMD64 and ARM64, publishes a commit-tagged image to GHCR, requests a rollout of the exact image digest through a provider adapter, and verifies the exact commit/digest plus API and worker readiness.

The workflow cannot complete until the staging environment, provider adapter, secrets, variables, registry pull access, and aggregate health endpoint are configured and tested. It does not deploy production or run database migrations. A successful adapter HTTP response alone is not deployment success; the post-rollout aggregate readiness check is required.

## Repository compatibility verified

- Go API entry point: `cmd/erp-api`; default listen address `:8080`.
- Existing multi-stage root `Dockerfile` emits a non-root distroless image and exposes port 8080.
- API health endpoints: `/health/live` and `/health/ready`.
- Worker health endpoints are private on port 8081.
- API readiness checks configured PostgreSQL connectivity; worker readiness checks its worker database connection. Neither proves webhook delivery.
- The image contains both `/erp-api` and `/outbox-worker`; the worker must run as a separate service with its entrypoint overridden to `/outbox-worker`.

## Required staging setup

Create a GitHub Environment named `staging`. Configure environment protection rules in GitHub Settings; referencing an environment in YAML does not by itself require an approval. Restrict who can deploy and, where supported by the repository plan, require an independent reviewer.

Configure only these workflow integration values in the `staging` Environment:

- Secret `STAGING_DEPLOY_HOOK_URL`: HTTPS endpoint for a trusted deployment adapter.
- Secret `STAGING_DEPLOY_TOKEN`: dedicated token accepted only by that adapter, scoped to staging rollout operations.
- Variable `STAGING_HEALTHCHECK_URL`: HTTPS aggregate status endpoint reachable from GitHub-hosted runners.

The adapter must accept a POST JSON request containing `image`, `digest`, `commit`, `environment`, and the API/worker service specifications. It must reject any environment other than staging, enforce an allow-list for this repository and image, deploy the supplied immutable digest, and never log the bearer token or secret configuration.

The aggregate health endpoint must return JSON with this contract:

```json
{
  "commit": "<deployed Git commit SHA>",
  "digest": "sha256:<deployed OCI image digest>",
  "api": { "status": "ready" },
  "worker": { "status": "ready" }
}
```

The endpoint must report ready only when the exact requested commit and digest are running and both services pass their readiness checks. The worker probe must remain private; the aggregate endpoint may report its result without exposing the worker port publicly.

Ensure the container platform can pull `ghcr.io/nyeinpyaesone-ui/arima`. Configure the API service to listen on port 8080 and the worker service to use the image entrypoint `/outbox-worker` with its private health port on 8081.

## Database and migration requirements

Keep `ARIMA_DATABASE_URL` and `ARIMA_OUTBOX_DATABASE_URL` in the hosting provider's secret manager, not in workflow variables or the deployment payload. Use private networking or a tightly restricted TLS endpoint, certificate verification, firewall allow-listing, and dedicated least-privilege runtime roles. Use a separate migration identity.

The application does not run migrations at startup. Database migrations must be applied as a separately reviewed, observable operation before any release that requires schema changes. Verify backups and restore procedures before relying on the environment.

## Rollback and failure handling

The current workflow detects failure to confirm readiness, but it does **not** automatically roll back. Before using it, document and test the provider's rollback operation, retain the last known-good image digest and configuration, and assign a responsible operator. Do not report a release as deployed based only on an accepted HTTP request.

## Remaining readiness blockers

- Select the actual hosting/container provider and implement its adapter to the contract above.
- Configure the GitHub `staging` Environment, protection rules, scoped secrets and variables.
- Confirm private GHCR pull permissions for the deployment platform.
- Verify PostgreSQL network access, TLS, role separation, backups and restore.
- Implement and test aggregate health reporting for the exact image digest and commit.
- Rehearse deployment and rollback; capture platform logs, health evidence and release identity.

Production deployment remains intentionally unconfigured.

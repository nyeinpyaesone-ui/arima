# Staging container deployment

## Current status

The repository provides two explicitly separated staging workflow modes in `.github/workflows/staging-deploy.yml`:

- **Preview (default):** builds the root Dockerfile for Linux AMD64 and ARM64 without publishing to GHCR, requesting a rollout, running migrations, or changing a live environment.
- **Deploy:** requires a separate explicit acknowledgement, the protected GitHub `staging` Environment, a trusted provider adapter, scoped credentials, and aggregate health verification.

A successful preview proves that the selected `main` source can build for the requested architectures. It does not prove that a container can start on the hosting platform or that the application is operational.

A successful adapter HTTP response alone is not deployment success. The post-rollout health check must confirm the exact requested commit and image digest and report both API and worker ready.

## Repository compatibility

- Go API entry point: `cmd/erp-api`; default listen address `:8080`.
- Root Dockerfile builds both `/erp-api` and `/outbox-worker` into a non-root distroless image.
- API health endpoints: `/health/live` and `/health/ready`.
- Worker health endpoints are private on port 8081.
- API readiness checks PostgreSQL connectivity; worker readiness checks its worker database connection. Neither proves webhook delivery.
- The worker must run as a separate service with its entrypoint overridden to `/outbox-worker`.

## Required staging setup before deploy mode

Create a GitHub Environment named `staging`. Configure environment protection rules in GitHub Settings; referencing an environment in YAML does not itself require an approval. Restrict who can deploy and, where supported, require an independent reviewer.

Configure only these integration values in the `staging` Environment:

- Secret `STAGING_DEPLOY_HOOK_URL`: HTTPS endpoint for a trusted deployment adapter.
- Secret `STAGING_DEPLOY_TOKEN`: dedicated token accepted only by that adapter and scoped to staging rollout operations.
- Variable `STAGING_HEALTHCHECK_URL`: HTTPS aggregate status endpoint reachable from GitHub-hosted runners.

The adapter must accept a POST JSON request containing `image`, `digest`, `commit`, `environment`, and API/worker service specifications. It must reject any environment other than staging, enforce an allow-list for this repository and image, deploy the supplied immutable digest, and never log the bearer token or secret configuration. It must also expose a tested rollback operation that restores the last known-good digest and configuration when readiness fails.

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

Ensure the container platform can pull `ghcr.io/nyeinpyaesone-ui/arima`. Configure the API service on port 8080 and the worker service with entrypoint `/outbox-worker` and private health port 8081.

## Database and migration requirements

Keep `ARIMA_DATABASE_URL` and `ARIMA_OUTBOX_DATABASE_URL` in the hosting provider's secret manager, not in workflow variables or deployment payloads. Use private networking or a tightly restricted TLS endpoint, certificate verification, firewall allow-listing, and dedicated least-privilege runtime roles. Use a separate migration identity.

The application does not run migrations at startup. Apply migrations as a separately reviewed, observable operation before any release requiring schema changes. Verify backups and restore procedures before relying on the environment.

## Rollback and failure handling

The workflow verifies release identity and both service readiness states, but it cannot safely implement provider-specific rollback without a selected provider and adapter contract. Before deploy mode is enabled, document and rehearse rollback, retain the last known-good image digest and configuration, and assign an operator. A failed readiness check is a failed deployment, not a successful release.

## Remaining readiness blockers

- Select the actual hosting/container provider and implement its adapter, including authenticated rollback.
- Configure the GitHub `staging` Environment, protection rules, scoped secrets and variable.
- Confirm private GHCR pull permissions for the deployment platform.
- Verify PostgreSQL network access, TLS, role separation, backups and restore.
- Implement and test aggregate health reporting for the exact image digest and commit.
- Rehearse deployment and rollback; capture platform logs, health evidence and release identity.

Production deployment remains intentionally unconfigured.

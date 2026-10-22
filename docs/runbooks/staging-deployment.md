# ARIMA staging deployment runbook

## Scope

This runbook describes a **managed-container-platform adapter** for staging. The repository does not yet identify a specific provider, so the workflow deliberately calls a provider adapter rather than pretending that a generic platform deploy hook is universally compatible.

The workflow is manually dispatched from `main`, requires explicit acknowledgement, uses the protected GitHub Environment `staging`, builds an immutable commit-tagged multi-architecture GHCR image, requests a rollout using the image digest, and polls the configured HTTPS health endpoint. It does not deploy production or run migrations.

## Required GitHub Environment

Create an environment named `staging` and configure:

- Secret `STAGING_DEPLOY_HOOK_URL`: HTTPS URL for a reviewed adapter implementing the contract below.
- Variable `STAGING_HEALTHCHECK_URL`: HTTPS URL for the deployed service's `/health/ready` endpoint.

Add required reviewers to the environment if supported by the repository's operating model. Do not store provider credentials, database DSNs, or deploy tokens in repository files or workflow variables. The adapter must keep provider credentials server-side.

## Adapter contract — mandatory before dispatch

The adapter must:

1. Authenticate and authorize the caller; HTTPS alone is not authentication.
2. Allow-list the repository and image namespace `ghcr.io/nyeinpyaesone-ui/arima`.
3. Accept JSON fields `image`, `digest`, `commit`, and `environment`.
4. Reject any environment other than `staging` and deploy the exact supplied digest, not a mutable tag.
5. Return a 2xx response only after the provider accepts the staging rollout request.
6. Preserve the previous known-good revision and document/rehearse provider-native rollback.
7. Avoid logging tokens, deploy-hook URLs, database credentials, or sensitive request bodies.

A vendor-provided one-click deploy hook is **not** assumed to implement this contract. Build and test the adapter for the chosen provider before enabling the workflow. The workflow does not follow HTTP redirects when sending the request.

## Service compatibility and limits

- Go API entry point: `cmd/erp-api`.
- Container listens on port `8080`.
- Existing endpoints: `/health/live` and `/health/ready`.
- Current readiness is process-level only; HTTP 200 with status `ready` does not establish database connectivity, OIDC readiness, or business correctness.
- The current `main` service is a scaffold. A successful rollout validates container delivery and process availability, not production ERP readiness.
- Image publishing on a push to `main` is handled by the separate container-image workflow; the staging workflow builds and publishes the selected commit itself so the rollout is tied to the image digest emitted in that run.

## PostgreSQL hosted separately from the managed container runtime

The current service scaffold has no active PostgreSQL persistence requirement. Do not inject an unused `ARIMA_DATABASE_URL` or claim database readiness until persistence and migrations are implemented.

When database-backed application code is introduced:

- Store `ARIMA_DATABASE_URL` in the managed platform's secret store, never in this workflow, logs, image labels, or adapter payload.
- Establish private connectivity from the container runtime to the PostgreSQL host using the provider's private network, VPN, or a narrowly restricted TLS endpoint.
- Enforce TLS certificate verification, firewall allow-lists, least-privilege runtime and separate migration roles, connection limits, backups, and tested restore procedures.
- Add database-aware readiness and integration tests using a non-superuser role with `NOSUPERUSER NOBYPASSRLS` where row-level security is required.
- Run schema migrations as a separately reviewed, controlled operation. This staging deploy workflow intentionally does not run them.

## Rollback and failure handling

The workflow fails if the adapter rejects the request or readiness is not observed within five minutes. It does not automatically roll back through an unknown provider API. The provider adapter/platform must retain the prior known-good revision and expose a documented rollback path. Rehearse rollback before calling staging validated; record the deployed commit, image digest, result, and redacted logs.

## Remaining readiness gates

- Select the managed container provider and implement/review its authenticated adapter.
- Configure and protect the `staging` environment and verify private GHCR image pull access.
- Validate container limits, port binding, secrets, TLS, DNS, logs, alerts, and provider rollback.
- Complete application-level OIDC/authz, tenant isolation, persistence, migrations, outbox, and database-aware readiness as applicable to the product.
- Pass current-head CI, dependency scanning, container scanning, end-to-end tests, and a documented staging/rollback rehearsal.
- Keep production deployment out of scope until the readiness evidence is reviewed and a separate protected production workflow is approved.

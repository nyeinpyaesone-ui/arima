# ARIMA staging deployment runbook

## Scope

This runbook describes a guarded managed-container-platform adapter. No specific provider has been selected, so the workflow calls a provider adapter rather than assuming a generic platform API.

The workflow is manually dispatched from `main`, requires explicit acknowledgement, uses the protected GitHub Environment `staging`, builds a commit-tagged multi-architecture GHCR image, requests rollout pinned to the exact image digest, and polls the configured HTTPS readiness endpoint. It does not deploy production or run database migrations.

## Required GitHub Environment

Create an environment named `staging` and configure:

- Secret `STAGING_DEPLOY_HOOK_URL`: HTTPS URL for a reviewed adapter implementing the contract below.
- Secret `STAGING_DEPLOY_TOKEN`: high-entropy bearer token shared with the adapter, independently rotatable from the URL.
- Variable `STAGING_HEALTHCHECK_URL`: HTTPS URL for the deployed service's `/health/ready` endpoint.
- Variables `ARIMA_OIDC_ISSUER` and `ARIMA_OIDC_AUDIENCE`; secret `ARIMA_DATABASE_URL` for the API runtime.

Require an environment reviewer before dispatching deployments. Restrict deployment branches to `main`. Do not store provider credentials, database DSNs, or deploy tokens in repository files or workflow variables. The adapter must keep provider credentials server-side. Never put tokens in URLs or health-check variables.

## Adapter contract — mandatory before dispatch

The adapter must:

1. Require HTTPS and authenticate the `Authorization: Bearer` token; compare it safely and support rotation.
2. Allow-list repository and image namespace `ghcr.io/nyeinpyaesone-ui/arima`.
3. Accept JSON fields `image`, `digest`, `commit`, and `environment`.
4. Reject any environment other than `staging` and deploy the exact supplied digest, not a mutable tag.
5. Return a 2xx response only after the provider accepts the staging rollout request.
6. Preserve the previous known-good revision and document/rehearse provider-native rollback.
7. Avoid logging tokens, deploy-hook URLs, database credentials, or sensitive request bodies.

A vendor-provided one-click deploy hook is not assumed to implement this contract. Build and test the adapter for the chosen provider before enabling the workflow. The workflow does not follow HTTP redirects when sending the request. Adapter authentication, repository allow-listing, and staging-only enforcement are mandatory; HTTPS alone is not caller authentication.

## Service compatibility and limits

- Go API entry point: `cmd/erp-api`.
- Container listens on port `8080`.
- Liveness is process-level. Readiness checks PostgreSQL when a database dependency is configured.
- Production startup requires OIDC issuer/audience and PostgreSQL schema readiness.
- The service still lacks business mutation routes and complete tenant-membership authorization. The outbox worker engine exists but is not wired to a concrete publisher or trusted tenant scheduler.
- A successful rollout validates container delivery and process/dependency readiness, not production ERP business readiness.
- Image publishing on push to `main` is handled by the container-image workflow. The staging workflow builds and publishes the selected commit itself so the rollout is tied to the image digest emitted in that run.
- No provider adapter has been selected or verified yet.

## PostgreSQL

Use a separately managed PostgreSQL 15+ service with private connectivity where possible. Store `ARIMA_DATABASE_URL` in the managed platform secret store, never in workflow logs, image labels, or adapter payloads. Production DSNs must use `sslmode=verify-full`.

Provision separate migration and runtime identities. The runtime identity must be `NOSUPERUSER NOBYPASSRLS` and receive only required table privileges. Apply migrations 0001 and 0002 as a reviewed operation before application rollout. The API does not run migrations at startup. Test migrations, RLS, backup and restore against a disposable database and then staging before production.

The CI integration suite validates migrations and tenant-scoped repository operations using a PostgreSQL 15 service and a non-superuser runtime role. This does not replace verification of the actual staging database grants, network path, TLS certificates, backup policy, or restore procedure.

## Rollback and failure handling

The workflow fails if the adapter rejects the request or readiness is not observed within five minutes. It does not automatically roll back through an unknown provider API. The provider adapter/platform must retain the prior known-good revision and expose a documented rollback path. Rehearse rollback before calling staging validated; record deployed commit, image digest, result, and redacted logs.

## Remaining readiness gates

- Select the managed-container provider and implement/review its authenticated adapter.
- Configure and protect the staging environment, rotate/secure the bearer token, and verify private GHCR image pull access.
- Validate container limits, port binding, secrets, TLS, DNS, logs, alerts, database TLS/RLS and provider rollback.
- Complete tenant-membership authorization, business routes/workflows, concrete outbox publisher/scheduler, and operational metrics.
- Pass current-head CI, dependency scanning, container scanning, end-to-end tests, and a documented staging/rollback rehearsal.
- Keep production deployment out of scope until readiness evidence is reviewed and a separate protected production workflow is approved.

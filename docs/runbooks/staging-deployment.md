# ARIMA staging deployment runbook

## Scope

This runbook describes a guarded managed-container-platform adapter. No specific provider has been selected, so the GitHub workflow calls a provider adapter rather than assuming a generic platform API.

The workflow is manually dispatched from `main`, requires explicit acknowledgement, uses the protected GitHub Environment `staging`, builds a multi-architecture GHCR image, and asks the adapter to deploy the exact image digest. The adapter must start two separate processes from that image: API and outbox worker. No production deployment or database migration is performed by this workflow.

## GitHub staging Environment

Create a GitHub Environment named `staging`. Store only the deployment-control values here:

- Secret `STAGING_DEPLOY_HOOK_URL`: HTTPS URL for the reviewed deployment adapter.
- Secret `STAGING_DEPLOY_TOKEN`: high-entropy bearer token used to authenticate workflow requests to the adapter.
- Variable `STAGING_HEALTHCHECK_URL`: HTTPS URL for the API `/health/ready` endpoint.

Require an environment reviewer before dispatching the workflow. Restrict deployment branches to `main`. Do not store runtime database credentials or webhook tokens in GitHub workflow files, image labels, or deployment payloads.

## Provider-side runtime configuration

The provider's own secret manager must supply these values to the corresponding process. They are not sent by GitHub Actions:

**API service**
- `ARIMA_ENV=production`
- `ARIMA_ADDR=:8080`
- Secret `ARIMA_DATABASE_URL`: dedicated API database role; PostgreSQL URL must use `sslmode=verify-full`.
- Variable `ARIMA_OIDC_ISSUER`: exact HTTPS issuer URL from the selected identity provider.
- Variable `ARIMA_OIDC_AUDIENCE`: audience configured for ARIMA access tokens.

**Outbox worker**
- `ARIMA_ENV=production`
- `ARIMA_OUTBOX_HEALTH_ADDR=:8081`
- Secret `ARIMA_OUTBOX_DATABASE_URL`: separate least-privileged worker database role; use `sslmode=verify-full`.
- Secret `ARIMA_OUTBOX_WEBHOOK_URL`: durable HTTPS receiver endpoint.
- Secret `ARIMA_OUTBOX_WEBHOOK_TOKEN`: bearer token accepted by that receiver.

Do not reuse migration credentials for either process. API and worker roles must be `NOSUPERUSER NOBYPASSRLS`. Keep webhook credentials out of the API container. Keep worker port 8081 private; only the API needs public ingress.

## Adapter contract — mandatory before dispatch

The adapter must:

1. Require HTTPS and authenticate `Authorization: Bearer` with `STAGING_DEPLOY_TOKEN`; support safe rotation and never log the token.
2. Allow-list repository and image namespace `ghcr.io/nyeinpyaesone-ui/arima`.
3. Accept the `image`, immutable `digest`, `commit`, `environment`, `services` and `readiness` fields from the workflow.
4. Reject any environment other than `staging`; deploy both processes from the exact supplied digest.
5. Launch the API with command `/erp-api` and the worker with command `/outbox-worker`. This is an command override, not a command argument; the image's default command is `/erp-api`.
6. Resolve each service's `required_environment` names from the provider secret manager. Never expect actual secret values in the deployment payload.
7. Verify both API and worker `/health/ready` endpoints from inside the provider network. Worker readiness must remain private; do not expose port 8081 publicly just so a GitHub-hosted runner can probe it.
8. Return 2xx only after the provider accepts the exact digest and both process readiness checks pass. Preserve the previous known-good revision and provide a tested rollback.
9. Avoid logging tokens, deployment hook URLs, database credentials, sensitive request bodies, or customer/event payloads.

A generic one-click deploy hook is not assumed to implement this contract. Build and test the adapter for the chosen provider before enabling the workflow. HTTPS alone is not caller authentication. Do not follow redirects when sending bearer credentials to the adapter.

The GitHub workflow performs a separate public API readiness check after the adapter accepts the rollout. The adapter is responsible for the worker's private readiness check.

## Service and database requirements

- The image contains `/erp-api` (port 8080) and `/outbox-worker` (health port 8081).
- API readiness verifies PostgreSQL connectivity when configured; production startup additionally verifies required schema including tenant authorization and outbox lease metadata.
- Worker readiness verifies worker-database connectivity and required outbox schema. It does not prove webhook delivery.
- Customer create/read routes enforce tenant permissions inside the tenant transaction. Customer creation commits the record, audit event, idempotency response and outbox event atomically.
- Migration 0003 creates tenant-local roles, permissions and memberships under forced RLS. Provision grants only through the trusted operator process; no public membership administration endpoint exists.
- Apply migrations 0001, 0002 and 0003 as a reviewed operation using a dedicated migration identity before starting either process. The API and worker do not run migrations at startup.

The API role should receive only SELECT/INSERT/UPDATE on `tenant_records` and `idempotency_keys`, INSERT on `audit_events`, SELECT/INSERT on `outbox_events`, and SELECT on tenant role/membership tables. It must not update outbox rows or administer memberships.

The worker role should receive only SELECT on `tenants` and SELECT/UPDATE on `outbox_events`. It must not read business records or membership tables. Verify effective privileges and forced RLS against the actual staging database.

## Webhook receiver contract

The receiver must authenticate the bearer token, accept only expected event types, durably accept each event before returning 2xx, and deduplicate by event ID using `Idempotency-Key` and `X-ARIMA-Event-ID`. Delivery is at-least-once, not exactly-once. HTTP 429/5xx are retried; permanent rejection is dead-lettered. Configure delivery metrics and alerts for pending age, retries, expired leases and dead-letter volume.

## Migration, rollback and remaining gates

The workflow deliberately does not apply migrations. Test migrations, permissions, RLS, backup and restore against a disposable PostgreSQL database and then staging. Production DSNs must use `sslmode=verify-full`; never disable TLS verification.

The adapter must retain the previous immutable image digest and support provider-native rollback. Rehearse rollback before treating staging as validated; record deployed commit, digest, result and redacted logs.

Before production use, complete real OIDC provider tests, secret-manager wiring, actual staging role/grant verification, webhook receiver verification, container/network/resource limits, monitoring, end-to-end business tests, SLOs, load tests and a separately approved protected production release process. No live staging or production deployment has been performed by this repository work.

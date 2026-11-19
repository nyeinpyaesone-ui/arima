# Initial Operations Runbook

## Local startup

Requirements: Docker Engine and Docker Compose plugin; Go 1.26.11 for local development.

```sh
cp configs/arima.env.example .env
make dev-config
make dev-up
```

Compose starts PostgreSQL 15, applies migrations 0001–0003 with a local migration ledger, creates separate least-privilege API/worker roles, and starts the API. The local database volume is persistent. `make dev-down` stops containers without deleting it; avoid `docker compose down -v` unless deleting local development data is intended.

The API is available at `http://127.0.0.1:8080`. Customer routes remain fail-closed until a real development OIDC provider is configured and the exact token subject is provisioned as a tenant member.

## Probes

- API liveness: `GET /health/live`
- API readiness: `GET /health/ready`
- Worker liveness: `GET /health/live` on internal port 8081
- Worker readiness: `GET /health/ready` on internal port 8081

API readiness checks the configured PostgreSQL connection. Worker readiness checks the worker database connection and required outbox schema; it does not prove webhook delivery. Keep worker port 8081 private. Monitor webhook failures, pending-event age, retries, expired leases and dead-letter volume separately.

The worker is optional in local development. Start it only after configuring an HTTPS receiver that durably accepts events and deduplicates by event ID:

```sh
# Set ARIMA_OUTBOX_WEBHOOK_URL and ARIMA_OUTBOX_WEBHOOK_TOKEN in .env first.
make dev-worker
```

## Incident collection

Capture deployment version, immutable image digest, UTC timestamp, request/correlation ID, redacted logs and recent change history. Never attach credentials, JWTs, database URLs, webhook tokens or unredacted customer/event payloads.

For outbox incidents, record event ID, tenant ID, event type, attempt count, last safe error code and dead-letter state. Do not copy full event payloads or bearer credentials into incident tickets.

## Rollback

Use the provider's documented rollback to redeploy the previous known-good immutable image and approved configuration. Database migrations are separate from service rollout; do not automatically run down-migrations or delete persistent volumes. Review data compatibility and backup/restore state before any schema rollback.

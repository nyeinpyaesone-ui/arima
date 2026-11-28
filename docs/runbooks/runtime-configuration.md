# Runtime configuration runbook

## Scope

ARIMA has an API process, a separate outbox worker process, PostgreSQL-backed tenant data, OIDC authentication, and forced tenant RLS. The API verifies required schema before production startup. Customer create/read routes enforce tenant permissions inside database transactions. The worker uses a distinct database identity, bounded tenant scanning, event leases, delivery timeouts and an HTTPS webhook receiver.

The application does not run schema migrations at startup. It is not a complete ERP: inventory, sales, purchasing, accounting, payments, branches, reporting and approval workflows remain outside the implemented scope.

## Local development

Use the repository's Compose setup rather than putting local credentials into tracked files:

```sh
cp configs/arima.env.example .env
# Edit .env only when you need a real local OIDC provider or webhook receiver.
make dev-up
docker compose logs -f api
```

The default API is available at `http://127.0.0.1:8080`. PostgreSQL is exposed only on the local loopback interface at port 5432. Compose runs reviewed migrations 0001, 0002 and 0003 through a one-shot migration service, records applied migration versions, and grants separate local-only API and worker roles. The persistent database volume survives `make dev-down`; do not use `docker compose down -v` unless you intentionally want to delete local data.

Without OIDC configuration, protected endpoints fail closed. To exercise customer routes, configure a real development OIDC issuer/audience and provision the exact token subject and tenant membership using the trusted operator procedure. Do not invent a tenant claim or accept a subject/role from the request body.

To start the worker, first set `ARIMA_OUTBOX_WEBHOOK_URL` to an HTTPS receiver that durably accepts events and deduplicates by event ID, and set `ARIMA_OUTBOX_WEBHOOK_TOKEN` in the ignored local `.env`. Then run:

```sh
make dev-worker
curl --fail --silent --show-error http://127.0.0.1:8081/health/ready
```

The worker endpoint checks process/database readiness, not end-to-end webhook delivery. Do not start it with an unconfigured or placeholder receiver.

## Supported API variables

| Variable | Default | Validation |
| --- | --- | --- |
| `ARIMA_ENV` | development | production requires OIDC and PostgreSQL |
| `ARIMA_ADDR` | `:8080` | host:port or :port; port 1–65535 |
| `ARIMA_READ_HEADER_TIMEOUT` | `5s` | positive Go duration, max 5m |
| `ARIMA_READ_TIMEOUT` | `15s` | positive Go duration, max 5m |
| `ARIMA_WRITE_TIMEOUT` | `15s` | positive Go duration, max 5m |
| `ARIMA_IDLE_TIMEOUT` | `60s` | positive Go duration, max 5m |
| `ARIMA_OIDC_ISSUER` | unset | exact HTTPS issuer from provider metadata |
| `ARIMA_OIDC_AUDIENCE` | unset | expected API audience |
| `ARIMA_DATABASE_URL` | unset | PostgreSQL URL; required in production, `sslmode=verify-full` |

Configure both OIDC variables or neither outside production. Production startup fails if either is absent or provider discovery fails. Discovery is attempted with a 10-second startup timeout.

## Worker variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `ARIMA_OUTBOX_DATABASE_URL` | yes | Dedicated worker database identity; production requires `sslmode=verify-full` |
| `ARIMA_OUTBOX_WEBHOOK_URL` | yes | Absolute HTTPS receiver URL without user info or fragment |
| `ARIMA_OUTBOX_WEBHOOK_TOKEN` | yes | Bearer credential for the receiver; store in a secret manager |
| `ARIMA_OUTBOX_HEALTH_ADDR` | no | Health listener; default `:8081` |

The worker process is `/outbox-worker`; the API process is `/erp-api`. The image default is `CMD ["/erp-api"]`; launch the worker by overriding the container command to `/outbox-worker`, not by passing it as an argument to `/erp-api`. Docker Compose uses `command: ["/outbox-worker"]` for the worker profile. They use separate database credentials and process health endpoints. The webhook must authenticate the bearer token, durably accept the event, deduplicate using `Idempotency-Key` and `X-ARIMA-Event-ID`, and return 2xx only after durable acceptance. Delivery is at-least-once. HTTP 429/5xx responses are retried; permanent rejections are dead-lettered. Never log tokens or full customer/event payloads.

## OIDC and authorization

ARIMA uses the maintained `github.com/coreos/go-oidc/v3/oidc` verifier. Provider discovery supplies issuer metadata and JWKS. The verifier checks JWT signature, exact issuer, audience, expiry and not-before; the remote key set supports signing-key rotation. The access token must contain non-empty `sub` and `tenant_id` claims.

Authorization is database-backed. The verified subject and tenant are used to check active membership and tenant-local permissions; client-supplied role names and permission lists are not trusted. Customer writes require `customer.write`, reads require `customer.read`. Provision role/membership rows through [tenant access provisioning](tenant-access-provisioning.md), never through public request input.

No real identity provider has been configured in this repository. Before production, test the selected provider's invalid signature, wrong issuer/audience, expiry, not-before, unknown/rotated key, missing tenant claim, outage and revocation behavior.

## PostgreSQL roles and migrations

Apply migrations 0001, 0002 and 0003 in order with a dedicated migration identity. The API never applies DDL during startup. For local development, `make dev-up` runs them through the Compose migration service and records applied versions in the local `arima_schema_migrations` ledger.

The API runtime identity must be `NOSUPERUSER NOBYPASSRLS` and receive only:
- SELECT/INSERT/UPDATE on `tenant_records` and `idempotency_keys`.
- INSERT on `audit_events`.
- SELECT/INSERT on `outbox_events`.
- SELECT on `tenant_roles`, `tenant_role_permissions` and `tenant_memberships`.

The worker identity must be `NOSUPERUSER NOBYPASSRLS` and receive only SELECT on `tenants`, plus SELECT/UPDATE on `outbox_events`. It must not read business records or tenant membership tables. The Compose role-grant script is strictly for local development and must not be used as a production provisioning mechanism.

Production PostgreSQL URLs for both API and worker must use TLS certificate and hostname verification (`sslmode=verify-full`). Do not use `sslmode=disable` or `require` for production. Store migration, API and worker credentials separately in the deployment platform's secret manager. Verify effective grants and forced RLS against the actual staging database before exposing the service.

## Production deployment rules

1. Set `ARIMA_ENV=production`; API startup requires OIDC issuer/audience, successful provider discovery, a reachable PostgreSQL server and the complete reviewed schema.
2. Configure the API and worker with separate database identities. Do not reuse the migration role.
3. Configure webhook receiver credentials through the worker secret store; do not put webhook secrets into the API container.
4. Keep API and worker behind trusted network boundaries. Expose only the necessary API ingress; keep worker health endpoints internal.
5. Set timeouts from observed request/receiver duration and measured service objectives; the defaults are safe starting points, not measured production SLOs.
6. Treat invalid configuration, failed discovery, incomplete migrations, failed readiness or missing worker receiver as a failed rollout.
7. Use protected environments, exact image digests, explicit approvals, smoke tests and a rehearsed provider-native rollback.

## Verification and rollback

Run `go mod verify`, `make check`, workflow lint, vulnerability scan and PostgreSQL integration tests before merge. The worker readiness endpoint confirms database connectivity only; separately monitor delivery failures, pending-event age, lease expiry, retry count and dead-letter volume. On rollout failure, restore the previous immutable image and approved configuration. Do not automatically run destructive database down-migrations.

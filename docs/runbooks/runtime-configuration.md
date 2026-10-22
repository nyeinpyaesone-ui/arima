# Runtime configuration runbook

## Scope

This runbook covers HTTP listener/timeouts and OIDC JWT validation. Tenant-scoped PostgreSQL schema and repository primitives exist, including RLS, audit, idempotency and outbox tables. Business routes, authorization policies, outbox delivery workers and end-to-end atomic business workflows are not implemented yet.

## Supported variables

| Variable | Default | Validation |
| --- | --- | --- |
| `ARIMA_ENV` | development | `production` requires OIDC configuration and successful discovery |
| `ARIMA_ADDR` | `:8080` | Must be `host:port` or `:port`; port 1–65535 |
| `ARIMA_READ_HEADER_TIMEOUT` | `5s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_READ_TIMEOUT` | `15s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_WRITE_TIMEOUT` | `15s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_IDLE_TIMEOUT` | `60s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_OIDC_ISSUER` | unset | Exact issuer URL from provider metadata |
| `ARIMA_OIDC_AUDIENCE` | unset | Expected API audience |
| `ARIMA_DATABASE_URL` | unset | PostgreSQL DSN; required in production |

Configure both OIDC variables or neither outside production. Production startup fails if either is absent or provider discovery fails. Discovery is attempted with a 10-second startup timeout.

## JWT validation boundary

ARIMA uses the maintained `github.com/coreos/go-oidc/v3/oidc` verifier. The provider's discovery metadata supplies the issuer and JWKS URI. The verifier checks JWT signature, exact issuer, configured audience, expiration and not-before; its remote key set supports signing-key rotation. The verified token must contain non-empty `sub` and `tenant_id` claims.

Use an access token intended for the ARIMA API, not an ID token issued to an unrelated client. Confirm with the identity-provider documentation which claims appear in access tokens. No real provider has yet been configured or tested in this repository; production rollout remains blocked until integration tests cover invalid signature, wrong issuer/audience, expiry, not-before, unknown/rotated key, missing tenant claim, provider outage and token revocation semantics.

## Configure and run locally

```sh
export ARIMA_ENV=development
export ARIMA_ADDR='127.0.0.1:8080'
export ARIMA_READ_HEADER_TIMEOUT='5s'
export ARIMA_READ_TIMEOUT='15s'
export ARIMA_WRITE_TIMEOUT='15s'
export ARIMA_IDLE_TIMEOUT='60s'

go test ./...
go vet ./...
test -z "$(gofmt -l ./cmd ./internal)"
go run ./cmd/erp-api
```

In another terminal:

```sh
curl --fail --silent --show-error http://127.0.0.1:8080/health/live
curl --fail --silent --show-error http://127.0.0.1:8080/health/ready
curl -i http://127.0.0.1:8080/api/v1/me
```

Health endpoints report process-level status only. Without OIDC configuration, the protected identity route returns 503. With a verifier configured, missing/invalid bearer credentials return 401.

## PostgreSQL setup and migration

The service requires `ARIMA_DATABASE_URL` in production and pings PostgreSQL before binding. The `/health/ready` endpoint returns 503 when the configured database ping fails. Use a dedicated migration identity to apply reviewed migrations; the application deliberately does not mutate schema at startup.

```sh
export ARIMA_DATABASE_URL='postgres://USER:PASSWORD@HOST:5432/DATABASE?sslmode=require'
psql "$ARIMA_DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0001_tenant_foundation.up.sql
```

Supply credentials through a secret manager, not shell history or committed files. Test migrations and rollback on a disposable PostgreSQL 15 database first. The down migration deletes tenant records, audit history, idempotency state and outbox events; use only after an approved backup and explicit data-loss approval. CI provisions a separate `arima_test` PostgreSQL database and runs the tenant-isolation integration test against it.

The repository's tenant transaction API sets `app.tenant_id` transaction-locally from the verified principal and the migration forces RLS on tenant-scoped tables. Do not issue queries outside `Postgres.WithinTenant` for tenant data. Ensure the application DB role is least-privileged and cannot bypass RLS; never grant `BYPASSRLS` or superuser to the runtime role. RLS is defense in depth, not a replacement for role/policy authorization.

## Deployment rules

1. Set `ARIMA_ENV=production`; startup requires OIDC issuer/audience, reachable provider discovery, and a working `ARIMA_DATABASE_URL`.
2. Set `ARIMA_OIDC_ISSUER` and `ARIMA_OIDC_AUDIENCE` from the identity provider's authoritative configuration. Never guess either value.
3. Keep the service behind a trusted TLS ingress and private network until authorization, tenant-scoped persistence and operational controls are implemented.
4. Change timeouts only from observed request-duration and traffic data; defaults are not measured production SLOs.
5. Treat invalid configuration or failed discovery as deployment failure; do not silently fall back.
6. The application does not automatically load dotenv files.

## Verification and rollback

- Require green GitHub Actions for the exact proposed commit before merging.
- Validate key rotation and outage behavior with the selected identity provider; do not assume revocation semantics beyond the provider's token format and policy.
- Verify bind address, health responses, logs and restart behavior in the target environment.
- On configuration rollout failure, restore the prior known-good environment and image. No database migrations are part of the runtime configuration step.

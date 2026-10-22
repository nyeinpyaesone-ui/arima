# Database migrations

Migrations are controlled deployment steps. The API never mutates the database schema at startup.

## Current migration

- `0001_tenant_foundation.up.sql`: tenants, tenant-scoped records, forced PostgreSQL RLS, audit events, idempotency keys and transactional outbox table.
- `0001_tenant_foundation.down.sql`: destructive rollback; removes all tables and data created by migration 0001.

Apply to a disposable/test database first, then a reviewed staging environment, before production:

```sh
psql "$ARIMA_MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0001_tenant_foundation.up.sql
```

Use a migration role with the privileges needed to create the extension and schema. The runtime role must be separate and least-privileged, must not be a superuser, and must not have `BYPASSRLS`. Verify actual grants and row-level security behavior under the runtime role before production.

## Tenant isolation contract

Tenant-scoped repository work must use `Postgres.WithinTenant`. It starts a SQL transaction and sets `app.tenant_id` transaction-locally. RLS policies match that value against each row's `tenant_id`; absent context matches no rows. The tenant ID must come from a verified principal, never from an untrusted request body or query parameter.

Mutations, idempotency reservation/completion, audit insertions and outbox enqueueing must occur within the same tenant transaction to preserve atomicity. Do not publish outbox events inside the database transaction; a separate worker must claim and publish committed outbox rows with retry and idempotent consumer semantics.

## Change discipline

- Prefer additive, backward-compatible expand/migrate/contract changes.
- Test upgrades, rollback, backup and restore before high-risk schema changes.
- Never store credentials or customer data in migration files.
- Never run the down migration in production without an approved backup and explicit data-loss authorization.
- Keep audit retention, idempotency expiry cleanup, outbox retry/dead-letter behavior and role grants as reviewed operational policy.

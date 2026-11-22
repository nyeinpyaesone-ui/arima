# Database migrations

Migrations are controlled deployment steps. The API never mutates the database schema at startup.

## Migration order

1. `0001_tenant_foundation.up.sql` creates tenants, tenant-scoped records, forced PostgreSQL RLS, audit events, idempotency keys and the transactional outbox.
2. `0002_outbox_leases.up.sql` adds worker lease ownership and expiry metadata so abandoned deliveries can be safely reclaimed.
3. `0003_tenant_authorization.up.sql` adds tenant-scoped roles, permissions and active/suspended/revoked memberships, all protected by forced RLS.


Apply in order to a disposable/test database, then a reviewed staging environment, before production:

```sh
psql "$ARIMA_MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0001_tenant_foundation.up.sql
psql "$ARIMA_MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0002_outbox_leases.up.sql
psql "$ARIMA_MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0003_tenant_authorization.up.sql
psql "$ARIMA_MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0004_customer_listing.up.sql
```

Use a dedicated migration role with the privileges needed to create the extension and schema. The runtime role must be separate and least-privileged, must not be a superuser, and must not have `BYPASSRLS`. Grant only the table operations required by the deployed application and worker. Verify actual grants and RLS behavior under the runtime role before production.

## Tenant isolation contract

Tenant-scoped repository work must use `Postgres.WithinTenant`. It starts a SQL transaction and sets `app.tenant_id` transaction-locally. RLS policies match that value against each row's `tenant_id`; absent context matches no rows. The tenant ID must come from a verified principal or trusted internal scheduler, never an untrusted request body or query parameter.

Mutations, idempotency reservation/completion, audit insertions and outbox enqueueing must occur within the same tenant transaction. Delivery occurs after commit: the worker claims one event using `FOR UPDATE SKIP LOCKED`, commits its lease, publishes outside the transaction, then acknowledges or schedules retry in a new tenant transaction. Delivery is at-least-once; handlers and downstream consumers must deduplicate by event ID. Expired leases are reclaimable, retries use bounded exponential backoff, and exhausted events move to `dead_letter`.

## Rollback and change discipline

- Migration 0002 down removes only lease metadata and preserves event rows, but must not be applied while workers are running.
- Migration 0001 down is destructive and removes tenant records, audit history, idempotency state and outbox events. Never run it in production without approved backup and explicit data-loss authorization.
- Prefer additive, backward-compatible expand/migrate/contract changes.
- Test upgrades, rollback, backup and restore before high-risk schema changes.
- Never store credentials or customer data in migration files.

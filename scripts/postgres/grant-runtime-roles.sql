-- LOCAL DEVELOPMENT ONLY.
-- The production migration/operator identities and credentials are managed separately.

SELECT format(
    'CREATE ROLE arima_api LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS',
    :'api_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'arima_api')
\gexec

SELECT format(
    'CREATE ROLE arima_outbox LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS',
    :'worker_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'arima_outbox')
\gexec

ALTER ROLE arima_api WITH LOGIN PASSWORD :'api_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
ALTER ROLE arima_outbox WITH LOGIN PASSWORD :'worker_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;

REVOKE ALL ON ALL TABLES IN SCHEMA public FROM arima_api, arima_outbox;
GRANT USAGE ON SCHEMA public TO arima_api, arima_outbox;

-- API: business data and idempotency, audit insert, outbox enqueue, read-only policy.
GRANT SELECT, INSERT, UPDATE ON tenant_records, idempotency_keys TO arima_api;
GRANT INSERT ON audit_events TO arima_api;
GRANT SELECT, INSERT ON outbox_events TO arima_api;
GRANT SELECT ON tenant_roles, tenant_role_permissions, tenant_memberships TO arima_api;

-- Worker: enumerate tenant IDs and mutate only leased outbox rows.
GRANT SELECT ON tenants TO arima_outbox;
GRANT SELECT, UPDATE ON outbox_events TO arima_outbox;

-- These are explicit invariants for local startup, not production credential provisioning.
DO $$
BEGIN
    IF (SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = 'arima_api') THEN
        RAISE EXCEPTION 'arima_api must not be superuser or BYPASSRLS';
    END IF;
    IF (SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = 'arima_outbox') THEN
        RAISE EXCEPTION 'arima_outbox must not be superuser or BYPASSRLS';
    END IF;
END $$;

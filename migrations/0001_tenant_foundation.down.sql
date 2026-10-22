-- Rollback removes tenant-scoped operational data. Take a verified backup first.
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS tenant_records;
DROP TABLE IF EXISTS tenants;

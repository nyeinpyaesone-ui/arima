-- Add recoverable leases for concurrent outbox workers.
-- Normalize legacy processing rows so they can be safely reclaimed after upgrade.
UPDATE outbox_events SET status = 'pending' WHERE status = 'processing';
ALTER TABLE outbox_events
    ADD COLUMN lease_owner text,
    ADD COLUMN lease_until timestamptz;

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_processing_lease_check
    CHECK (
        (status = 'processing' AND lease_owner IS NOT NULL AND lease_until IS NOT NULL)
        OR
        (status <> 'processing' AND lease_owner IS NULL AND lease_until IS NULL)
    );

CREATE INDEX outbox_expired_lease_idx
    ON outbox_events (tenant_id, lease_until, created_at)
    WHERE status = 'processing';

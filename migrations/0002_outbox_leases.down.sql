-- Revert lease metadata only; preserve all queued and delivered events.
DROP INDEX IF EXISTS outbox_expired_lease_idx;
ALTER TABLE outbox_events
    DROP CONSTRAINT IF EXISTS outbox_processing_lease_check,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS lease_owner;

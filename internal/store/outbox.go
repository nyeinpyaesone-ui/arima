package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNoOutboxEvent = errors.New("no outbox event is currently available")

type OutboxEvent struct {
	ID            string
	TenantID      string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Attempts      int
}

// ClaimOutbox atomically leases one event. The lease is committed before delivery,
// so network I/O never holds a database transaction open. Expired leases are reclaimable.
func (t *TenantTx) ClaimOutbox(ctx context.Context, workerID string, lease time.Duration) (OutboxEvent, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || len(workerID) > 128 || lease <= 0 {
		return OutboxEvent{}, errors.New("worker ID and positive lease duration are required")
	}
	var event OutboxEvent
	err := t.tx.QueryRowContext(ctx, `
WITH candidate AS (
    SELECT tenant_id, id
    FROM outbox_events
    WHERE tenant_id = $1::uuid
      AND (
          (status = 'pending' AND available_at <= now())
          OR (status = 'processing' AND lease_until <= now())
      )
    ORDER BY available_at, created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE outbox_events AS event
SET status = 'processing',
    attempts = event.attempts + 1,
    lease_owner = $2,
    lease_until = now() + ($3 * interval '1 second')
FROM candidate
WHERE event.tenant_id = candidate.tenant_id
  AND event.id = candidate.id
RETURNING event.id::text, event.tenant_id::text, event.aggregate_type,
          event.aggregate_id, event.event_type, event.payload, event.attempts
`, t.tenantID, workerID, lease.Seconds()).Scan(
		&event.ID, &event.TenantID, &event.AggregateType, &event.AggregateID,
		&event.EventType, &event.Payload, &event.Attempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return OutboxEvent{}, ErrNoOutboxEvent
	}
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("claim outbox event: %w", err)
	}
	return event, nil
}

// MarkOutboxPublished acknowledges only the event's current lease owner.
func (t *TenantTx) MarkOutboxPublished(ctx context.Context, eventID, workerID string) error {
	result, err := t.tx.ExecContext(ctx, `
UPDATE outbox_events
SET status = 'published', published_at = now(), lease_owner = NULL, lease_until = NULL
WHERE tenant_id = $1::uuid AND id = $2::uuid
  AND status = 'processing' AND lease_owner = $3
`, t.tenantID, eventID, workerID)
	if err != nil {
		return fmt.Errorf("acknowledge outbox event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("outbox lease lost before acknowledgement")
	}
	return nil
}

// FailOutbox schedules bounded exponential retry or moves the event to dead_letter.
// Error codes are deliberately stored instead of raw error text to avoid leaking secrets.
func (t *TenantTx) FailOutbox(ctx context.Context, eventID, workerID, errorCode string, maxAttempts int) (bool, error) {
	errorCode = strings.TrimSpace(errorCode)
	if errorCode == "" {
		errorCode = "delivery_failed"
	}
	if !safeOutboxErrorCode(errorCode) || maxAttempts < 1 {
		return false, errors.New("valid error code and positive max attempts are required")
	}
	var deadLetter bool
	err := t.tx.QueryRowContext(ctx, `
UPDATE outbox_events
SET status = CASE WHEN attempts >= $4 THEN 'dead_letter' ELSE 'pending' END,
    available_at = CASE
        WHEN attempts >= $4 THEN available_at
        ELSE now() + make_interval(secs => LEAST((5 * POWER(2, LEAST(attempts - 1, 10)))::integer, 3600))
    END,
    last_error_code = $5,
    lease_owner = NULL,
    lease_until = NULL
WHERE tenant_id = $1::uuid AND id = $2::uuid
  AND status = 'processing' AND lease_owner = $3
RETURNING status = 'dead_letter'
`, t.tenantID, eventID, workerID, maxAttempts, errorCode).Scan(&deadLetter)
	if errors.Is(err, sql.ErrNoRows) {
		return false, errors.New("outbox lease lost before failure acknowledgement")
	}
	if err != nil {
		return false, fmt.Errorf("record outbox delivery failure: %w", err)
	}
	return deadLetter, nil
}

func safeOutboxErrorCode(value string) bool {
	if value == "" || len(value) > 100 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

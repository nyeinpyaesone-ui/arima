package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidTenant       = errors.New("tenant ID is required")
	ErrVersionConflict     = errors.New("record version conflict")
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different request")
	ErrIdempotencyPending  = errors.New("idempotency key is already in progress")
	ErrRecordNotFound      = errors.New("record not found")
)

type Postgres struct {
	db *sql.DB
}

type TenantTx struct {
	tx       *sql.Tx
	tenantID string
}

type Record struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	ResourceType string          `json:"resource_type"`
	ExternalID   string          `json:"external_id"`
	Version      int64           `json:"version"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type IdempotencyResult struct {
	Created        bool
	Completed      bool
	ResponseStatus int
	ResponseBody   json.RawMessage
}

// NewPostgres wraps a configured database/sql connection. The caller owns db and
// must register and open a vetted PostgreSQL driver before constructing this store.
func NewPostgres(db *sql.DB) (*Postgres, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}
	return &Postgres{db: db}, nil
}

// WithinTenant establishes a transaction-local tenant context before any query.
// PostgreSQL RLS policies use this setting and default to denying rows when absent.
func (p *Postgres) WithinTenant(ctx context.Context, tenantID string, fn func(*TenantTx) error) error {
	if p == nil || p.db == nil {
		return errors.New("database store is not initialized")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !validUUID(tenantID) {
		return ErrInvalidTenant
	}
	if fn == nil {
		return errors.New("tenant transaction callback is required")
	}
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tenant transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}
	if err := fn(&TenantTx{tx: tx, tenantID: tenantID}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	return nil
}

func (t *TenantTx) UpsertRecord(ctx context.Context, resourceType, externalID string, expectedVersion int64, payload json.RawMessage) (Record, error) {
	if !validObject(payload) || strings.TrimSpace(resourceType) == "" || strings.TrimSpace(externalID) == "" {
		return Record{}, errors.New("resource type, external ID, and JSON object payload are required")
	}
	if expectedVersion < 0 {
		return Record{}, errors.New("expected version cannot be negative")
	}
	var query string
	var args []any
	if expectedVersion == 0 {
		query = `
INSERT INTO tenant_records (tenant_id, resource_type, external_id, version, payload)
VALUES ($1::uuid, $2, $3, 1, $4::jsonb)
ON CONFLICT (tenant_id, resource_type, external_id) DO NOTHING
RETURNING id::text, tenant_id::text, resource_type, external_id, version, payload, created_at, updated_at`
		args = []any{t.tenantID, resourceType, externalID, string(payload)}
	} else {
		query = `
UPDATE tenant_records
SET payload = $4::jsonb, version = version + 1, updated_at = now()
WHERE tenant_id = $1::uuid AND resource_type = $2 AND external_id = $3 AND version = $5
RETURNING id::text, tenant_id::text, resource_type, external_id, version, payload, created_at, updated_at`
		args = []any{t.tenantID, resourceType, externalID, string(payload), expectedVersion}
	}
	var record Record
	err := t.tx.QueryRowContext(ctx, query, args...).
		Scan(&record.ID, &record.TenantID, &record.ResourceType, &record.ExternalID, &record.Version, &record.Payload, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrVersionConflict
	}
	if err != nil {
		return Record{}, fmt.Errorf("upsert tenant record: %w", err)
	}
	return record, nil
}

func (t *TenantTx) GetRecord(ctx context.Context, resourceType, externalID string) (Record, error) {
	const query = `SELECT id::text, tenant_id::text, resource_type, external_id, version, payload, created_at, updated_at
FROM tenant_records WHERE tenant_id = $1::uuid AND resource_type = $2 AND external_id = $3`
	var record Record
	err := t.tx.QueryRowContext(ctx, query, t.tenantID, resourceType, externalID).
		Scan(&record.ID, &record.TenantID, &record.ResourceType, &record.ExternalID, &record.Version, &record.Payload, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrRecordNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("read tenant record: %w", err)
	}
	return record, nil
}

func (t *TenantTx) AppendAudit(ctx context.Context, actor, action, resourceType, resourceID, outcome, correlationID string, details json.RawMessage) error {
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(action) == "" || strings.TrimSpace(correlationID) == "" {
		return errors.New("audit actor, action, and correlation ID are required")
	}
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	} else if !validObject(details) {
		return errors.New("audit details must be a valid JSON object")
	}
	_, err := t.tx.ExecContext(ctx, `
INSERT INTO audit_events (tenant_id, actor_subject, action, resource_type, resource_id, outcome, correlation_id, details)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8::jsonb)`,
		t.tenantID, actor, action, resourceType, resourceID, outcome, correlationID, string(details))
	if err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

func (t *TenantTx) EnqueueOutbox(ctx context.Context, aggregateType, aggregateID, eventType string, payload json.RawMessage) (string, error) {
	if strings.TrimSpace(aggregateType) == "" || strings.TrimSpace(aggregateID) == "" || strings.TrimSpace(eventType) == "" || !validObject(payload) {
		return "", errors.New("outbox aggregate, event type, and JSON object payload are required")
	}
	var id string
	err := t.tx.QueryRowContext(ctx, `
INSERT INTO outbox_events (tenant_id, aggregate_type, aggregate_id, event_type, payload)
VALUES ($1::uuid, $2, $3, $4, $5::jsonb)
RETURNING id::text`, t.tenantID, aggregateType, aggregateID, eventType, string(payload)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("enqueue outbox event: %w", err)
	}
	return id, nil
}

// BeginIdempotency creates a tenant-scoped request key in the same transaction as
// the business change. Callers must commit completion, audit, outbox and mutation together.
func (t *TenantTx) BeginIdempotency(ctx context.Context, key string, requestBody []byte, ttl time.Duration) (IdempotencyResult, error) {
	if len(key) < 8 || len(key) > 200 || ttl <= 0 || len(requestBody) == 0 {
		return IdempotencyResult{}, errors.New("idempotency key, request body, and positive TTL are required")
	}
	hash := sha256.Sum256(requestBody)
	var id string
	err := t.tx.QueryRowContext(ctx, `
INSERT INTO idempotency_keys (tenant_id, key, request_hash, state, expires_at)
VALUES ($1::uuid, $2, $3, 'in_progress', now() + ($4 * interval '1 second'))
ON CONFLICT (tenant_id, key) DO NOTHING
RETURNING key`, t.tenantID, key, hash[:], ttl.Seconds()).Scan(&id)
	if err == nil {
		return IdempotencyResult{Created: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return IdempotencyResult{}, fmt.Errorf("reserve idempotency key: %w", err)
	}
	var existingHash []byte
	var state string
	var status sql.NullInt64
	var response []byte
	var expires time.Time
	err = t.tx.QueryRowContext(ctx, `
SELECT request_hash, state, response_status, response_body, expires_at
FROM idempotency_keys WHERE tenant_id = $1::uuid AND key = $2 FOR UPDATE`, t.tenantID, key).
		Scan(&existingHash, &state, &status, &response, &expires)
	if err != nil {
		return IdempotencyResult{}, fmt.Errorf("read idempotency key: %w", err)
	}
	if time.Now().After(expires) {
		return IdempotencyResult{}, errors.New("idempotency key expired; use a new key")
	}
	if string(existingHash) != string(hash[:]) {
		return IdempotencyResult{}, ErrIdempotencyConflict
	}
	if state != "completed" || !status.Valid {
		return IdempotencyResult{}, ErrIdempotencyPending
	}
	return IdempotencyResult{Completed: true, ResponseStatus: int(status.Int64), ResponseBody: json.RawMessage(response)}, nil
}

func (t *TenantTx) CompleteIdempotency(ctx context.Context, key string, status int, response json.RawMessage) error {
	if status < 100 || status > 599 || !validJSON(response) {
		return errors.New("valid response status and JSON response body are required")
	}
	result, err := t.tx.ExecContext(ctx, `
UPDATE idempotency_keys SET state = 'completed', response_status = $3, response_body = $4::jsonb
WHERE tenant_id = $1::uuid AND key = $2 AND state = 'in_progress'`, t.tenantID, key, status, string(response))
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("idempotency reservation not found or already completed")
	}
	return nil
}

func validObject(raw json.RawMessage) bool {
	if !validJSON(raw) {
		return false
	}
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func validJSON(raw json.RawMessage) bool {
	return len(raw) > 0 && json.Valid(raw)
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	if len(compact) != 32 {
		return false
	}
	_, err := hex.DecodeString(compact)
	return err == nil
}

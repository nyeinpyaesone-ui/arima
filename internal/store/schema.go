package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// VerifySchema checks the reviewed migration's required tables without applying DDL.
// Schema changes remain a separate, explicitly controlled deployment step.
func VerifySchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database connection is required")
	}
	const query = `
SELECT
	to_regclass('public.tenants') IS NOT NULL
	AND to_regclass('public.tenant_records') IS NOT NULL
	AND to_regclass('public.audit_events') IS NOT NULL
	AND to_regclass('public.idempotency_keys') IS NOT NULL
	AND to_regclass('public.outbox_events') IS NOT NULL`
	var ready bool
	if err := db.QueryRowContext(ctx, query).Scan(&ready); err != nil {
		return fmt.Errorf("verify PostgreSQL schema: %w", err)
	}
	if !ready {
		return errors.New("required ARIMA tables are missing; apply reviewed migrations before startup")
	}
	return nil
}

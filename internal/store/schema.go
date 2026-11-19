package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// VerifySchema checks required tables, forced tenant RLS policies and outbox
// lease metadata without applying DDL. Schema changes remain a deployment step.
func VerifySchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database connection is required")
	}
	const tablesQuery = `
SELECT
	to_regclass('public.tenants') IS NOT NULL
	AND to_regclass('public.tenant_records') IS NOT NULL
	AND to_regclass('public.audit_events') IS NOT NULL
	AND to_regclass('public.idempotency_keys') IS NOT NULL
	AND to_regclass('public.outbox_events') IS NOT NULL
	AND to_regclass('public.tenant_roles') IS NOT NULL
	AND to_regclass('public.tenant_role_permissions') IS NOT NULL
	AND to_regclass('public.tenant_memberships') IS NOT NULL`
	var tablesReady bool
	if err := db.QueryRowContext(ctx, tablesQuery).Scan(&tablesReady); err != nil {
		return fmt.Errorf("verify PostgreSQL schema: %w", err)
	}
	if !tablesReady {
		return errors.New("required ARIMA tables are missing; apply reviewed migrations before startup")
	}
	const rlsQuery = `
SELECT count(*) = 7
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relname IN ('tenant_records', 'audit_events', 'idempotency_keys', 'outbox_events', 'tenant_roles', 'tenant_role_permissions', 'tenant_memberships')
  AND c.relrowsecurity
  AND c.relforcerowsecurity
  AND EXISTS (
    SELECT 1 FROM pg_policies p
    WHERE p.schemaname = n.nspname
      AND p.tablename = c.relname
      AND p.policyname = 'tenant_isolation'
      AND p.qual IS NOT NULL
      AND p.with_check IS NOT NULL
  )`
	var rlsReady bool
	if err := db.QueryRowContext(ctx, rlsQuery).Scan(&rlsReady); err != nil {
		return fmt.Errorf("verify PostgreSQL tenant row-level security: %w", err)
	}
	if !rlsReady {
		return errors.New("tenant isolation is not production-ready: all tenant tables must have ENABLE and FORCE ROW LEVEL SECURITY plus tenant_isolation USING/WITH CHECK policy")
	}
	const leaseQuery = `
SELECT count(*) = 2
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'outbox_events'
  AND column_name IN ('lease_owner', 'lease_until')`
	var leaseReady bool
	if err := db.QueryRowContext(ctx, leaseQuery).Scan(&leaseReady); err != nil {
		return fmt.Errorf("verify outbox lease schema: %w", err)
	}
	if !leaseReady {
		return errors.New("outbox lease schema is missing; apply migration 0002_outbox_leases")
	}
	return nil
}


// VerifyOutboxWorkerSchema checks only the objects required by the restricted
// worker identity. It intentionally does not require access to business or
// tenant-authorization tables.
func VerifyOutboxWorkerSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database connection is required")
	}
	const query = `
SELECT
    to_regclass('public.tenants') IS NOT NULL
    AND to_regclass('public.outbox_events') IS NOT NULL
    AND COALESCE((
        SELECT relrowsecurity AND relforcerowsecurity
        FROM pg_class
        WHERE oid = to_regclass('public.outbox_events')
    ), false)
    AND EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'public'
          AND tablename = 'outbox_events'
          AND policyname = 'tenant_isolation'
          AND qual IS NOT NULL
          AND with_check IS NOT NULL
    )
    AND (
        SELECT count(*) = 2
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'outbox_events'
          AND column_name IN ('lease_owner', 'lease_until')
    )`
	var ready bool
	if err := db.QueryRowContext(ctx, query).Scan(&ready); err != nil {
		return fmt.Errorf("verify outbox worker schema: %w", err)
	}
	if !ready {
		return errors.New("outbox worker schema is not ready; apply reviewed migrations 0001 and 0002")
	}
	return nil
}

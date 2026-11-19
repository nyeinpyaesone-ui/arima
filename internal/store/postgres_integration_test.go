package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/customer"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresTenantIsolationAndAtomicControls(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ARIMA_TEST_DATABASE_URL"))
	runtimeDSN := strings.TrimSpace(os.Getenv("ARIMA_TEST_RUNTIME_DATABASE_URL"))
	if dsn == "" || runtimeDSN == "" {
		t.Skip("ARIMA_TEST_DATABASE_URL and ARIMA_TEST_RUNTIME_DATABASE_URL are required")
	}
	if !strings.Contains(dsn, "/arima_test") || !strings.Contains(runtimeDSN, "/arima_test") {
		t.Fatal("refusing to run migration integration test unless both database names include arima_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	for _, migrationName := range []string{"0001_tenant_foundation.up.sql", "0002_outbox_leases.up.sql", "0003_tenant_authorization.up.sql"} {
		migration, err := os.ReadFile("../../migrations/" + migrationName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s to isolated test database: %v", migrationName, err)
		}
	}
	// Seed tenant membership and permission fixtures through the migration identity.
	_, err = db.ExecContext(ctx, `
INSERT INTO tenants (id, slug) VALUES
 ('11111111-1111-4111-8111-111111111111', 'integration-tenant-a'),
 ('22222222-2222-4222-8222-222222222222', 'integration-tenant-b')
ON CONFLICT (id) DO NOTHING;
INSERT INTO tenant_roles (tenant_id, role_key, display_name)
VALUES
 ('11111111-1111-4111-8111-111111111111', 'operator', 'Operator'),
 ('22222222-2222-4222-8222-222222222222', 'operator', 'Operator')
ON CONFLICT DO NOTHING;
INSERT INTO tenant_role_permissions (tenant_id, role_key, permission)
VALUES ('11111111-1111-4111-8111-111111111111', 'operator', 'customer.read'),
       ('11111111-1111-4111-8111-111111111111', 'operator', 'customer.write')
ON CONFLICT DO NOTHING;
INSERT INTO tenant_memberships (tenant_id, subject, role_key, status)
VALUES
 ('11111111-1111-4111-8111-111111111111', 'user-a', 'operator', 'active'),
 ('22222222-2222-4222-8222-222222222222', 'user-a', 'operator', 'suspended')
ON CONFLICT (tenant_id, subject) DO UPDATE SET status = EXCLUDED.status;`)
	if err != nil {
		t.Fatalf("seed tenant authorization fixtures: %v", err)
	}
	if err := VerifySchema(ctx, db); err != nil {
		t.Fatalf("verify migrated schema and forced tenant RLS: %v", err)
	}
	_, err = db.ExecContext(ctx, `
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'arima_runtime') THEN
		CREATE ROLE arima_runtime LOGIN PASSWORD 'arima_test_password' NOSUPERUSER NOBYPASSRLS;
	END IF;
END $$;
GRANT CONNECT ON DATABASE arima_test TO arima_runtime;
GRANT USAGE ON SCHEMA public TO arima_runtime;
GRANT SELECT, INSERT, UPDATE ON tenant_records, audit_events, idempotency_keys, outbox_events TO arima_runtime;
GRANT SELECT ON tenant_roles, tenant_role_permissions, tenant_memberships TO arima_runtime;`)
	if err != nil {
		t.Fatalf("create restricted runtime test role: %v", err)
	}
	runtimeDB, err := sql.Open("pgx", runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeDB.Close()
	if err := runtimeDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	const tenantA = "11111111-1111-4111-8111-111111111111"
	const tenantB = "22222222-2222-4222-8222-222222222222"
	for _, tenant := range []struct{ id, slug string }{{tenantA, "integration-tenant-a"}, {tenantB, "integration-tenant-b"}} {
		if _, err := db.ExecContext(ctx, "INSERT INTO tenants (id, slug) VALUES ($1::uuid, $2) ON CONFLICT (id) DO NOTHING", tenant.id, tenant.slug); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := NewPostgres(runtimeDB)
	if err != nil {
		t.Fatal(err)
	}

	authorizer, err := auth.NewPostgresAuthorizer(runtimeDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := authorizer.Authorize(ctx, auth.Principal{Subject: "user-a", TenantID: tenantA}, "customer.read"); err != nil {
		t.Fatalf("active tenant membership should grant permission: %v", err)
	}

	customerService, err := customer.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	customerInput := customer.Customer{ExternalID: "customer-service-001", Name: "Service Test", Phone: "+959123456789"}
	created, err := customerService.Create(ctx, auth.Principal{Subject: "user-a", TenantID: tenantA}, "customer-create-key-01", "integration-correlation-1", customerInput)
	if err != nil {
		t.Fatalf("create customer through service: %v", err)
	}
	if created.ExternalID != customerInput.ExternalID || created.Name != customerInput.Name || created.Version != 1 {
		t.Fatalf("unexpected created customer: %#v", created)
	}
	replayed, err := customerService.Create(ctx, auth.Principal{Subject: "user-a", TenantID: tenantA}, "customer-create-key-01", "integration-correlation-2", customerInput)
	if err != nil {
		t.Fatalf("replay idempotent customer create: %v", err)
	}
	if replayed.ID != created.ID || replayed.Version != created.Version {
		t.Fatalf("idempotent replay changed customer result: first=%#v replay=%#v", created, replayed)
	}
	loaded, err := customerService.Get(ctx, auth.Principal{Subject: "user-a", TenantID: tenantA}, customerInput.ExternalID)
	if err != nil {
		t.Fatalf("read customer through service: %v", err)
	}
	if loaded.ID != created.ID || loaded.Name != created.Name {
		t.Fatalf("loaded customer mismatch: %#v", loaded)
	}
	if _, err := customerService.Create(ctx, auth.Principal{Subject: "user-a", TenantID: tenantB}, "customer-create-key-02", "integration-correlation-3", customerInput); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("suspended tenant membership must not create customer: %v", err)
	}
	if _, err := customerService.Get(ctx, auth.Principal{Subject: "user-a", TenantID: tenantB}, customerInput.ExternalID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("suspended tenant membership must not read customer: %v", err)
	}
	if err := authorizer.Authorize(ctx, auth.Principal{Subject: "user-a", TenantID: tenantB}, "customer.read"); !errors.Is(err, auth.ErrPermissionDenied) {
		t.Fatalf("suspended membership must be denied: %v", err)
	}
	if err := authorizer.Authorize(ctx, auth.Principal{Subject: "unknown-user", TenantID: tenantA}, "customer.read"); !errors.Is(err, auth.ErrPermissionDenied) {
		t.Fatalf("unknown tenant member must be denied: %v", err)
	}
	if err := authorizer.Authorize(ctx, auth.Principal{Subject: "user-a", TenantID: tenantA}, "customer.write"); !errors.Is(err, auth.ErrPermissionDenied) {
		t.Fatalf("ungranted permission must be denied: %v", err)
	}

	if err := repo.WithinTenant(ctx, tenantA, func(tx *TenantTx) error {
		if err := tx.RequirePermission(ctx, "user-a", "customer.read"); err != nil {
			return err
		}
		if err := tx.RequirePermission(ctx, "user-a", "customer.write"); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("ungranted permission in mutation transaction must be denied: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("transactional permission check: %v", err)
	}
	if err := repo.WithinTenant(ctx, tenantB, func(tx *TenantTx) error {
		if err := tx.RequirePermission(ctx, "user-a", "customer.read"); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("suspended membership must be denied inside transaction: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("tenant B permission check: %v", err)
	}
	if err := repo.WithinTenant(ctx, "not-a-uuid", func(*TenantTx) error { return nil }); !errors.Is(err, ErrInvalidTenant) {
		t.Fatalf("invalid tenant ID should be rejected before opening a transaction, got %v", err)
	}

	var recordID string
	var outboxID string
	var deadLetterID string
	err = repo.WithinTenant(ctx, tenantA, func(tx *TenantTx) error {
		record, err := tx.UpsertRecord(ctx, "customer", "customer-1", 0, json.RawMessage(`{"name":"Tenant A"}`))
		if err != nil {
			return err
		}
		recordID = record.ID
		if record.Version != 1 || record.TenantID != tenantA {
			t.Fatalf("unexpected created record: %#v", record)
		}
		if _, err := tx.UpsertRecord(ctx, "customer", "customer-1", 0, json.RawMessage(`{"name":"overwrite"}`)); !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("create with duplicate external ID should conflict, got %v", err)
		}
		updated, err := tx.UpsertRecord(ctx, "customer", "customer-1", 1, json.RawMessage(`{"name":"Updated"}`))
		if err != nil {
			return err
		}
		if updated.Version != 2 {
			t.Fatalf("version = %d, want 2", updated.Version)
		}
		if err := tx.AppendAudit(ctx, "user-1", "customer.updated", "customer", recordID, "succeeded", "correlation-1", json.RawMessage(`{"test":true}`)); err != nil {
			return err
		}
		outboxID, err = tx.EnqueueOutbox(ctx, "customer", recordID, "customer.updated", json.RawMessage(`{"record_id":"`+recordID+`"}`))
		if err != nil {
			return err
		}
		deadLetterID, err = tx.EnqueueOutbox(ctx, "customer", recordID, "customer.notification", json.RawMessage(`{"record_id":"`+recordID+`"}`))
		if err != nil {
			return err
		}
		if _, err := tx.tx.ExecContext(ctx, "UPDATE outbox_events SET available_at = now() - interval '1 second' WHERE tenant_id = $1::uuid AND id = $2::uuid", tenantA, outboxID); err != nil {
			return err
		}
		if _, err := tx.tx.ExecContext(ctx, "UPDATE outbox_events SET available_at = now() + interval '1 hour' WHERE tenant_id = $1::uuid AND id = $2::uuid", tenantA, deadLetterID); err != nil {
			return err
		}
		result, err := tx.BeginIdempotency(ctx, "request-key-0001", []byte(`{"action":"update","id":"customer-1"}`), time.Hour)
		if err != nil {
			return err
		}
		if !result.Created {
			t.Fatal("first idempotency reservation should be new")
		}
		return tx.CompleteIdempotency(ctx, "request-key-0001", 200, json.RawMessage(`{"accepted":true}`))
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.WithinTenant(ctx, tenantA, func(tx *TenantTx) error {
		event, err := tx.ClaimOutbox(ctx, "integration-worker", time.Minute)
		if err != nil {
			return err
		}
		if event.ID != outboxID || event.Attempts != 1 || event.TenantID != tenantA {
			t.Fatalf("unexpected claimed outbox event: %#v", event)
		}
		return tx.MarkOutboxPublished(ctx, event.ID, "integration-worker")
	})
	if err != nil {
		t.Fatalf("claim and acknowledge outbox event: %v", err)
	}
	err = repo.WithinTenant(ctx, tenantA, func(tx *TenantTx) error {
		if _, err := tx.tx.ExecContext(ctx, "UPDATE outbox_events SET available_at = now() WHERE tenant_id = $1::uuid AND id = $2::uuid", tenantA, deadLetterID); err != nil {
			return err
		}
		event, err := tx.ClaimOutbox(ctx, "integration-worker-2", time.Minute)
		if err != nil {
			return err
		}
		if event.ID != deadLetterID {
			t.Fatalf("claimed event ID = %q, want %q", event.ID, deadLetterID)
		}
		dead, err := tx.FailOutbox(ctx, event.ID, "integration-worker-2", "provider_unavailable", 1)
		if err != nil {
			return err
		}
		if !dead {
			t.Fatal("event at max attempts should enter dead_letter")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = repo.WithinTenant(ctx, tenantA, func(tx *TenantTx) error {
		if _, err := tx.ClaimOutbox(ctx, "integration-worker-3", time.Minute); !errors.Is(err, ErrNoOutboxEvent) {
			t.Fatalf("published/dead-letter events must not be claimed again, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.WithinTenant(ctx, tenantB, func(tx *TenantTx) error {
		if _, err := tx.GetRecord(ctx, "customer", "customer-1"); !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("tenant B must not read tenant A record; got %v", err)
		}
		var count int
		if err := tx.tx.QueryRowContext(ctx, "SELECT count(*) FROM audit_events").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Fatalf("tenant B sees %d tenant A audit rows", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.WithinTenant(ctx, tenantA, func(tx *TenantTx) error {
		result, err := tx.BeginIdempotency(ctx, "request-key-0001", []byte(`{"action":"update","id":"customer-1"}`), time.Hour)
		if err != nil {
			return err
		}
		var replayBody struct {
			Accepted bool `json:"accepted"`
		}
		if err := json.Unmarshal(result.ResponseBody, &replayBody); err != nil {
			return fmt.Errorf("decode replay response: %w", err)
		}
		if !result.Completed || result.ResponseStatus != 200 || !replayBody.Accepted {
			t.Fatalf("unexpected idempotent replay result: %#v", result)
		}
		_, err = tx.BeginIdempotency(ctx, "request-key-0001", []byte(`{"action":"delete","id":"customer-1"}`), time.Hour)
		if !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("different request payload must conflict, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

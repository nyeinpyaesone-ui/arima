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
	migration, err := os.ReadFile("../../migrations/0001_tenant_foundation.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply tenant migration to isolated test database: %v", err)
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
GRANT SELECT, INSERT, UPDATE ON tenant_records, audit_events, idempotency_keys, outbox_events TO arima_runtime;`)
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
	if err := repo.WithinTenant(ctx, "not-a-uuid", func(*TenantTx) error { return nil }); !errors.Is(err, ErrInvalidTenant) {
		t.Fatalf("invalid tenant ID should be rejected before opening a transaction, got %v", err)
	}

	var recordID string
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
		if _, err := tx.EnqueueOutbox(ctx, "customer", recordID, "customer.updated", json.RawMessage(`{"record_id":"`+recordID+`"}`)); err != nil {
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

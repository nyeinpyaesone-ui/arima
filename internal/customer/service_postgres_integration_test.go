package customer

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCustomerServicePostgresAtomicCreateAndAuthorization(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("ARIMA_CUSTOMER_TEST_DATABASE_URL"))
	runtimeDSN := strings.TrimSpace(os.Getenv("ARIMA_CUSTOMER_TEST_RUNTIME_DATABASE_URL"))
	if adminDSN == "" || runtimeDSN == "" {
		t.Skip("isolated customer PostgreSQL integration database is not configured")
	}
	if !strings.Contains(adminDSN, "/arima_customer_test") || !strings.Contains(runtimeDSN, "/arima_customer_test") {
		t.Fatal("refusing to run customer integration test outside arima_customer_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0001_tenant_foundation.up.sql", "0002_outbox_leases.up.sql", "0003_tenant_authorization.up.sql"} {
		migration, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := adminDB.ExecContext(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	if err := store.VerifySchema(ctx, adminDB); err != nil {
		t.Fatalf("verify customer integration schema: %v", err)
	}
	_, err = adminDB.ExecContext(ctx, `
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'arima_customer_runtime') THEN
        CREATE ROLE arima_customer_runtime LOGIN PASSWORD 'arima_customer_test_password' NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;
GRANT CONNECT ON DATABASE arima_customer_test TO arima_customer_runtime;
GRANT USAGE ON SCHEMA public TO arima_customer_runtime;
GRANT SELECT, INSERT, UPDATE ON tenant_records, idempotency_keys TO arima_customer_runtime;
GRANT SELECT, INSERT ON audit_events TO arima_customer_runtime;
GRANT INSERT ON outbox_events TO arima_customer_runtime;
GRANT SELECT ON tenant_roles, tenant_role_permissions, tenant_memberships TO arima_customer_runtime;`)
	if err != nil {
		t.Fatalf("provision restricted customer runtime role: %v", err)
	}

	const tenantA = "33333333-3333-4333-8333-333333333333"
	const tenantB = "44444444-4444-4444-8444-444444444444"
	for _, tenant := range []struct{ id, slug string }{{tenantA, "customer-integration-a"}, {tenantB, "customer-integration-b"}} {
		if _, err := adminDB.ExecContext(ctx, "INSERT INTO tenants (id, slug) VALUES ($1::uuid, $2)", tenant.id, tenant.slug); err != nil {
			t.Fatal(err)
		}
	}
	_, err = adminDB.ExecContext(ctx, `
INSERT INTO tenant_roles (tenant_id, role_key, display_name)
VALUES ($1::uuid, 'operator', 'Operator');
INSERT INTO tenant_role_permissions (tenant_id, role_key, permission)
VALUES ($1::uuid, 'operator', 'customer.read'),
       ($1::uuid, 'operator', 'customer.write');
INSERT INTO tenant_memberships (tenant_id, subject, role_key, status)
VALUES ($1::uuid, 'customer-user-1', 'operator', 'active');`, tenantA)
	if err != nil {
		t.Fatalf("seed customer permissions: %v", err)
	}

	runtimeDB, err := sql.Open("pgx", runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeDB.Close()
	if err := runtimeDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	repository, err := store.NewPostgres(runtimeDB)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	principal := auth.Principal{Subject: "customer-user-1", TenantID: tenantA}
	input := Customer{ExternalID: "cust-001", Name: "Integration Customer", Phone: "+959123456789"}

	created, err := service.Create(ctx, principal, "customer-create-0001", "customer-correlation-1", input)
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if created.ID == "" || created.TenantID != tenantA || created.Version != 1 || created.Name != input.Name {
		t.Fatalf("unexpected create result: %#v", created)
	}
	replayed, err := service.Create(ctx, principal, "customer-create-0001", "customer-correlation-2", input)
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if replayed.ID != created.ID || replayed.Version != created.Version {
		t.Fatalf("replay differs from original result: created=%#v replayed=%#v", created, replayed)
	}
	if _, err := service.Create(ctx, principal, "customer-create-0001", "customer-correlation-3", Customer{ExternalID: "cust-001", Name: "Different Payload"}); !errors.Is(err, store.ErrIdempotencyConflict) {
		t.Fatalf("same key with different request must conflict, got %v", err)
	}
	if _, err := service.Create(ctx, principal, "customer-create-0002", "customer-correlation-4", input); !errors.Is(err, store.ErrVersionConflict) {
		t.Fatalf("duplicate customer external ID must conflict, got %v", err)
	}
	loaded, err := service.Get(ctx, principal, input.ExternalID)
	if err != nil {
		t.Fatalf("read customer: %v", err)
	}
	if loaded.ID != created.ID || loaded.Name != created.Name {
		t.Fatalf("loaded customer differs from created customer: %#v", loaded)
	}
	if _, err := service.Get(ctx, auth.Principal{Subject: "unprovisioned-user", TenantID: tenantA}, input.ExternalID); !errors.Is(err, store.ErrPermissionDenied) {
		t.Fatalf("unprovisioned member must be denied: %v", err)
	}
	if _, err := service.Get(ctx, auth.Principal{Subject: "customer-user-1", TenantID: tenantB}, input.ExternalID); !errors.Is(err, store.ErrPermissionDenied) {
		t.Fatalf("membership must not cross tenant boundary: %v", err)
	}

	var auditCount, outboxCount, idempotencyCount int
	if err := adminDB.QueryRowContext(ctx, "SELECT count(*) FROM audit_events WHERE tenant_id = $1::uuid", tenantA).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := adminDB.QueryRowContext(ctx, "SELECT count(*) FROM outbox_events WHERE tenant_id = $1::uuid", tenantA).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if err := adminDB.QueryRowContext(ctx, "SELECT count(*) FROM idempotency_keys WHERE tenant_id = $1::uuid", tenantA).Scan(&idempotencyCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || outboxCount != 1 || idempotencyCount != 1 {
		t.Fatalf("failed/replayed operations left partial side effects: audit=%d outbox=%d idempotency=%d", auditCount, outboxCount, idempotencyCount)
	}
}

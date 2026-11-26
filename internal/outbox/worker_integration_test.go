package outbox

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type workerIntegrationHandler struct {
	fail  map[string]error
	calls []string
}

func (h *workerIntegrationHandler) Publish(_ context.Context, event store.OutboxEvent) error {
	h.calls = append(h.calls, event.ID)
	return h.fail[event.EventType]
}

type permanentDeliveryError struct{}

func (permanentDeliveryError) Error() string { return "permanent rejection" }
func (permanentDeliveryError) Code() string { return "test_rejected" }
func (permanentDeliveryError) Permanent() bool { return true }

type retryableDeliveryError struct{}

func (retryableDeliveryError) Error() string { return "temporary failure" }
func (retryableDeliveryError) Code() string  { return "test_unavailable" }

func TestWorkerProcessOneDeliveryAndLeaseRecovery(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("ARIMA_OUTBOX_TEST_DATABASE_URL"))
	runtimeDSN := strings.TrimSpace(os.Getenv("ARIMA_OUTBOX_TEST_RUNTIME_DATABASE_URL"))
	if adminDSN == "" || runtimeDSN == "" {
		t.Skip("outbox integration database URLs are required")
	}
	if !strings.Contains(adminDSN, "/arima_outbox_test") || !strings.Contains(runtimeDSN, "/arima_outbox_test") {
		t.Fatal("worker integration test requires the isolated arima_outbox_test database")
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

	for _, name := range []string{
		"0001_tenant_foundation.up.sql",
		"0002_outbox_leases.up.sql",
		"0003_tenant_authorization.up.sql",
	} {
		migration, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := adminDB.ExecContext(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}

	_, err = adminDB.ExecContext(ctx, `
GRANT CONNECT ON DATABASE arima_outbox_test TO arima_outbox_runtime;
GRANT USAGE ON SCHEMA public TO arima_outbox_runtime;
GRANT SELECT ON tenants TO arima_outbox_runtime;
GRANT SELECT, UPDATE ON outbox_events TO arima_outbox_runtime;`)
	if err != nil {
		t.Fatalf("grant restricted worker permissions: %v", err)
	}

	runtimeDB, err := sql.Open("pgx", runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeDB.Close()
	if err := runtimeDB.PingContext(ctx); err != nil {
		t.Fatalf("connect as worker: %v", err)
	}
	if err := store.VerifyOutboxWorkerSchema(ctx, runtimeDB); err != nil {
		t.Fatalf("verify worker schema: %v", err)
	}

	adminRepo, err := store.NewPostgres(adminDB)
	if err != nil {
		t.Fatal(err)
	}
	runtimeRepo, err := store.NewPostgres(runtimeDB)
	if err != nil {
		t.Fatal(err)
	}

	tenants := []struct {
		id   string
		slug string
	}{
		{"31111111-1111-4111-8111-111111111111", "outbox-worker-test-a"},
		{"32222222-2222-4222-8222-222222222222", "outbox-worker-test-b"},
		{"33333333-3333-4333-8333-333333333333", "outbox-worker-test-c"},
		{"34444444-4444-4444-8444-444444444444", "outbox-worker-test-d"},
		{"35555555-5555-4555-8555-555555555555", "outbox-worker-test-e"},
	}
	for _, tenant := range tenants {
		_, err := adminDB.ExecContext(
			ctx,
			"INSERT INTO tenants (id, slug) VALUES ($1::uuid, $2) ON CONFLICT (id) DO NOTHING",
			tenant.id,
			tenant.slug,
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := adminDB.ExecContext(ctx, "DELETE FROM outbox_events WHERE tenant_id = $1::uuid", tenant.id); err != nil {
			t.Fatal(err)
		}
	}

	enqueue := func(tenantID, eventType string) string {
		t.Helper()
		var id string
		err := adminRepo.WithinTenant(ctx, tenantID, func(tx *store.TenantTx) error {
			var enqueueErr error
			id, enqueueErr = tx.EnqueueOutbox(ctx, "test", "aggregate-1", eventType, []byte(`{"test":true}`))
			return enqueueErr
		})
		if err != nil {
			t.Fatalf("enqueue event: %v", err)
		}
		return id
	}

	successID := enqueue(tenants[0].id, "test.success")
	retryID := enqueue(tenants[1].id, "test.retry")
	rejectID := enqueue(tenants[2].id, "test.reject")
	leaseID := enqueue(tenants[3].id, "test.lease")
	_, err = adminDB.ExecContext(
		ctx,
		"UPDATE outbox_events SET status='processing', attempts=1, lease_owner='expired-worker', lease_until=now()-interval '1 second' WHERE tenant_id=$1::uuid AND id=$2::uuid",
		tenants[3].id,
		leaseID,
	)
	if err != nil {
		t.Fatal(err)
	}

	handler := &workerIntegrationHandler{
		fail: map[string]error{
			"test.retry":  retryableDeliveryError{},
			"test.reject": permanentDeliveryError{},
		},
	}
	worker, err := NewWorker(
		runtimeRepo,
		handler,
		time.Second,
		10*time.Millisecond,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		tenant   string
		id       string
		status   string
		attempts int
	}{
		{"success", tenants[0].id, successID, "published", 1},
		{"retry", tenants[1].id, retryID, "pending", 1},
		{"permanent rejection", tenants[2].id, rejectID, "dead_letter", 1},
		{"expired lease recovery", tenants[3].id, leaseID, "published", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			processed, err := worker.ProcessOne(ctx, tc.tenant)
			if err != nil || !processed {
				t.Fatalf("ProcessOne() = (%v, %v), want (true, nil)", processed, err)
			}

			var status string
			var attempts int
			var leaseOwner sql.NullString
			err = adminDB.QueryRowContext(
				ctx,
				"SELECT status, attempts, lease_owner FROM outbox_events WHERE tenant_id=$1::uuid AND id=$2::uuid",
				tc.tenant,
				tc.id,
			).Scan(&status, &attempts, &leaseOwner)
			if err != nil {
				t.Fatal(err)
			}
			if status != tc.status || attempts != tc.attempts {
				t.Fatalf("state=(%s,%d), want (%s,%d)", status, attempts, tc.status, tc.attempts)
			}
			if status != "processing" && leaseOwner.Valid {
				t.Fatalf("lease owner remains set: %s", leaseOwner.String)
			}
		})
	}

	processed, err := worker.ProcessOne(ctx, tenants[4].id)
	if err != nil || processed {
		t.Fatalf("empty queue = (%v,%v), want (false,nil)", processed, err)
	}
	if len(handler.calls) != 4 {
		t.Fatalf("handler calls=%d, want 4", len(handler.calls))
	}
}

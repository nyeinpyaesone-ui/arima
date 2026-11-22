package customer

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

func TestNewServiceRequiresRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("nil repository must be rejected")
	}
}

func TestCustomerServiceValidatesInputBeforeDatabaseAccess(t *testing.T) {
	service, err := NewService(&store.Postgres{})
	if err != nil {
		t.Fatal(err)
	}
	principal := auth.Principal{Subject: "user-1", TenantID: "11111111-1111-4111-8111-111111111111"}

	if _, err := service.Create(context.Background(), principal, "valid-key-0001", "corr-1", Customer{}); !errors.Is(err, ErrInvalidCustomer) {
		t.Fatalf("empty customer must be rejected before database access, got %v", err)
	}
	valid := Customer{ExternalID: "customer-1", Name: "Test Customer"}
	if _, err := service.Create(context.Background(), principal, "", "corr-1", valid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("missing idempotency key must be rejected before database access, got %v", err)
	}
	if _, err := service.List(context.Background(), principal, -1, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("negative limit must be rejected before database access: %v", err)
	}
	if _, err := service.List(context.Background(), principal, 101, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("limit above maximum must be rejected before database access: %v", err)
	}
	if _, err := service.List(context.Background(), principal, 25, "not-a-valid-cursor!"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid cursor must be rejected before database access: %v", err)
	}
	invalidUTF8Cursor := base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xfe})
	if _, err := service.List(context.Background(), principal, 25, invalidUTF8Cursor); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("cursor containing invalid UTF-8 must be rejected before database access: %v", err)
	}
	if _, err := service.List(context.Background(), principal, 25, strings.Repeat("a", 513)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("oversized cursor must be rejected before database access: %v", err)
	}
	if _, err := service.Get(context.Background(), principal, " "); !errors.Is(err, ErrInvalidCustomer) {
		t.Fatalf("empty external ID must be rejected before database access, got %v", err)
	}
}

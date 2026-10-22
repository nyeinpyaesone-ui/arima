package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type authorizerFunc func(context.Context, Principal, string) error

func (f authorizerFunc) Authorize(ctx context.Context, p Principal, permission string) error {
	return f(ctx, p, permission)
}

func TestRequirePermissionFailsClosedWithoutAuthorizerOrPrincipal(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next must not be called") })
	cases := []struct {
		name      string
		authorizer Authorizer
		handler   http.Handler
		want      int
	}{
		{name: "no authorizer", authorizer: nil, handler: next, want: http.StatusServiceUnavailable},
		{name: "no authenticated principal", authorizer: authorizerFunc(func(context.Context, Principal, string) error { return nil }), handler: next, want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			RequirePermission(tc.authorizer, "customer.read", tc.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestRequirePermissionPassesVerifiedPrincipalAndPermission(t *testing.T) {
	want := Principal{Subject: "user-a", TenantID: "tenant-a"}
	authorizer := authorizerFunc(func(_ context.Context, got Principal, permission string) error {
		if got != want {
			t.Fatalf("principal = %#v, want %#v", got, want)
		}
		if permission != "customer.read" {
			t.Fatalf("permission = %q", permission)
		}
		return nil
	})
	protected := RequirePermission(authorizer, "customer.read", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	handler := RequireBearer(verifierFunc(func(context.Context, string) (Principal, error) {
		return want, nil
	}), protected)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestRequirePermissionDeniesWithoutPolicyGrant(t *testing.T) {
	authorizer := authorizerFunc(func(_ context.Context, p Principal, permission string) error {
		if p.TenantID != "tenant-a" || permission != "customer.write" {
			return errors.New("denied")
		}
		return ErrPermissionDenied
	})
	protected := RequirePermission(authorizer, "customer.write", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("denied request reached handler")
	}))
	handler := RequireBearer(verifierFunc(func(context.Context, string) (Principal, error) {
		return Principal{Subject: "user-a", TenantID: "tenant-a"}, nil
	}), protected)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

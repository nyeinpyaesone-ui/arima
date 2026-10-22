package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
)

type testVerifier struct{}

func (testVerifier) Verify(_ context.Context, token string) (auth.Principal, error) {
	if token != "valid-test-token" {
		return auth.Principal{}, auth.ErrInvalidToken
	}
	return auth.Principal{Subject: "user-42", TenantID: "tenant-7"}, nil
}

func TestHealthEndpoints(t *testing.T) {
	handler := New()
	for _, path := range []string{"/health/live", "/health/ready"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["status"] == nil || body["service"] != "arima-erp-api" {
				t.Fatalf("unexpected body: %#v", body)
			}
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("security header missing")
			}
		})
	}
}

func TestProtectedIdentityRouteFailsClosedWithoutVerifier(t *testing.T) {
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestProtectedIdentityRouteRequiresValidBearer(t *testing.T) {
	handler := NewWithVerifier(testVerifier{})
	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{name: "missing token", want: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer invalid", want: http.StatusUnauthorized},
		{name: "valid token", header: "Bearer valid-test-token", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusOK {
				var body identityResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Subject != "user-42" || body.TenantID != "tenant-7" {
					t.Fatalf("unexpected identity response: %#v", body)
				}
			}
		})
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestReadinessFailsWhenDependencyIsUnavailable(t *testing.T) {
	handler := NewWithDependencies(testVerifier{}, func(context.Context) error {
		return errors.New("database unavailable")
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "not_ready" {
		t.Fatalf("status body = %q, want not_ready", body.Status)
	}
}

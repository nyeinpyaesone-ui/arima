package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	handler := New()
	for _, path := range []string{"/health/live", "/health/ready"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK { t.Fatalf("status = %d, want 200", rec.Code) }
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil { t.Fatalf("invalid JSON: %v", err) }
			if body["status"] == nil || body["service"] != "arima-erp-api" { t.Fatalf("unexpected body: %#v", body) }
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" { t.Fatal("security header missing") }
		})
	}
}
func TestUnknownPathIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound { t.Fatalf("status = %d, want 404", rec.Code) }
}

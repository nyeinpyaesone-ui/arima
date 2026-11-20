package outbox

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandlerLivenessAndFailClosedReadiness(t *testing.T) {
	handler := NewHealthHandler(nil)
	live := httptest.NewRecorder()
	handler.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if live.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want 200", live.Code)
	}
	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want 503 without database", ready.Code)
	}
}

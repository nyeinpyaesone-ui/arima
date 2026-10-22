package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
)

type healthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Timestamp time.Time `json:"timestamp"`
}

type identityResponse struct {
	Subject  string `json:"subject"`
	TenantID string `json:"tenant_id"`
}

func New() http.Handler {
	return NewWithVerifier(nil)
}

// NewWithVerifier registers protected API routes using the supplied trusted verifier.
// When verifier is nil, protected routes fail closed with 503 while health endpoints remain available.
func NewWithVerifier(verifier auth.TokenVerifier) http.Handler {
	return NewWithDependencies(verifier, nil)
}

// NewWithDependencies injects a dependency readiness check without coupling transport to storage.
func NewWithDependencies(verifier auth.TokenVerifier, readiness func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", health("live"))
	mux.HandleFunc("GET /health/ready", readinessHandler(readiness))
	mux.Handle("GET /api/v1/me", auth.RequireBearer(verifier, http.HandlerFunc(currentIdentity)))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"service": "arima-erp-api", "status": "running", "docs": "https://github.com/nyeinpyaesone-ui/arima/blob/main/api/openapi/openapi.yaml"})
	})
	return securityHeaders(mux)
}

func currentIdentity(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, identityResponse{Subject: principal.Subject, TenantID: principal.TenantID})
}

func health(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: status, Service: "arima-erp-api", Timestamp: time.Now().UTC()})
	}
}

func readinessHandler(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			if err := check(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Service: "arima-erp-api", Timestamp: time.Now().UTC()})
				return
			}
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ready", Service: "arima-erp-api", Timestamp: time.Now().UTC()})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

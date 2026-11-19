package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func NewHealthHandler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, http.StatusOK, healthResponse{Status: "live", Service: "arima-outbox-worker"})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeHealth(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Service: "arima-outbox-worker"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			writeHealth(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Service: "arima-outbox-worker"})
			return
		}
		writeHealth(w, http.StatusOK, healthResponse{Status: "ready", Service: "arima-outbox-worker"})
	})
	return mux
}

func writeHealth(w http.ResponseWriter, status int, body healthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

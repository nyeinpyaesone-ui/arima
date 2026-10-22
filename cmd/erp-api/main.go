package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/transport/httpapi"
)

func main() {
	addr := os.Getenv("ARIMA_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr: addr, Handler: httpapi.New(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("ARIMA ERP API listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/config"
	"github.com/nyeinpyaesone-ui/arima/internal/transport/httpapi"
)

func main() {
	cfg, err := config.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("invalid runtime configuration: %v", err)
	}
	verifier, err := auth.NewIntrospectionVerifierFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("invalid authentication configuration: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewWithVerifier(verifier),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	log.Printf("ARIMA ERP API listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

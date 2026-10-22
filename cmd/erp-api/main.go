package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/config"
	"github.com/nyeinpyaesone-ui/arima/internal/transport/httpapi"
)

func main() {
	cfg, err := config.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("invalid runtime configuration: %v", err)
	}

	discoveryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	verifier, err := auth.NewOIDCVerifierFromEnv(discoveryCtx, os.Getenv)
	cancel()
	if err != nil {
		log.Fatalf("invalid OIDC authentication configuration: %v", err)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ARIMA_ENV")), "production") && verifier == nil {
		log.Fatal("ARIMA_ENV=production requires ARIMA_OIDC_ISSUER and ARIMA_OIDC_AUDIENCE")
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

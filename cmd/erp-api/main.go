package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/config"
	"github.com/nyeinpyaesone-ui/arima/internal/store"
	"github.com/nyeinpyaesone-ui/arima/internal/transport/httpapi"
)

func main() {
	cfg, err := config.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("invalid runtime configuration: %v", err)
	}
	production := strings.EqualFold(strings.TrimSpace(os.Getenv("ARIMA_ENV")), "production")

	discoveryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	verifier, err := auth.NewOIDCVerifierFromEnv(discoveryCtx, os.Getenv)
	cancel()
	if err != nil {
		log.Fatalf("invalid OIDC authentication configuration: %v", err)
	}
	if production && verifier == nil {
		log.Fatal("ARIMA_ENV=production requires ARIMA_OIDC_ISSUER and ARIMA_OIDC_AUDIENCE")
	}

	var dbCloser interface{ Close() error }
	var dbPing func(context.Context) error
	dsn := strings.TrimSpace(os.Getenv("ARIMA_DATABASE_URL"))
	if production && dsn == "" {
		log.Fatal("ARIMA_ENV=production requires ARIMA_DATABASE_URL")
	}
	if dsn != "" {
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err := store.OpenPostgres(dbCtx, dsn)
		dbCancel()
		if err != nil {
			log.Fatalf("PostgreSQL startup check failed: %v", err)
		}
		dbCloser = db
		dbPing = db.PingContext
	}
	if dbCloser != nil {
		defer dbCloser.Close()
	}

	readiness := func(ctx context.Context) error {
		if dbPing != nil {
			return dbPing(ctx)
		}
		if production {
			return errors.New("PostgreSQL is required in production")
		}
		return nil
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewWithDependencies(verifier, readiness),
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

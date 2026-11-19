package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/config"
	"github.com/nyeinpyaesone-ui/arima/internal/customer"
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
	var customerService *customer.Service
	dsn := strings.TrimSpace(os.Getenv("ARIMA_DATABASE_URL"))
	if production && dsn == "" {
		log.Fatal("ARIMA_ENV=production requires ARIMA_DATABASE_URL")
	}
	if production {
		if err := store.ValidateProductionDSN(dsn); err != nil {
			log.Fatalf("invalid production PostgreSQL configuration: %v", err)
		}
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
		postgresStore, err := store.NewPostgres(db)
		if err != nil {
			log.Fatalf("initialize PostgreSQL repository: %v", err)
		}
		customerService, err = customer.NewService(postgresStore)
		if err != nil {
			log.Fatalf("initialize customer service: %v", err)
		}
		if production {
			schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := store.VerifySchema(schemaCtx, db)
			schemaCancel()
			if err != nil {
				log.Fatalf("PostgreSQL schema verification failed: %v", err)
			}
		}
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
		Handler:           httpapi.NewWithCustomerService(verifier, readiness, customerService),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("ARIMA ERP API listening on %s", cfg.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped with error: %v", err)
		}
	case <-shutdownSignal.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful HTTP shutdown failed: %v", err)
			_ = server.Close()
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped with error: %v", err)
		}
	}
}

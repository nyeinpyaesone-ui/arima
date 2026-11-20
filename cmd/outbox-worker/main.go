package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/outbox"
	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

func main() {
	getenv := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	dsn := getenv("ARIMA_OUTBOX_DATABASE_URL")
	endpoint := getenv("ARIMA_OUTBOX_WEBHOOK_URL")
	token := getenv("ARIMA_OUTBOX_WEBHOOK_TOKEN")
	if dsn == "" || endpoint == "" || token == "" {
		log.Fatal("ARIMA_OUTBOX_DATABASE_URL, ARIMA_OUTBOX_WEBHOOK_URL and ARIMA_OUTBOX_WEBHOOK_TOKEN are required")
	}
	if strings.EqualFold(getenv("ARIMA_ENV"), "production") {
		if err := store.ValidateProductionDSN(dsn); err != nil {
			log.Fatalf("invalid production outbox database configuration: %v", err)
		}
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := store.OpenPostgres(startupCtx, dsn)
	startupCancel()
	if err != nil {
		log.Fatalf("outbox database startup check failed: %v", err)
	}
	defer db.Close()

	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = store.VerifyOutboxWorkerSchema(verifyCtx, db)
	verifyCancel()
	if err != nil {
		log.Fatalf("outbox database schema verification failed: %v", err)
	}

	repository, err := store.NewPostgres(db)
	if err != nil {
		log.Fatalf("initialize outbox repository: %v", err)
	}
	handler, err := outbox.NewHTTPSWebhookHandler(endpoint, token, nil)
	if err != nil {
		log.Fatalf("invalid outbox webhook configuration: %v", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	worker, err := outbox.NewWorker(repository, handler, 30*time.Second, 2*time.Second, 10, logger)
	if err != nil {
		log.Fatalf("initialize outbox worker: %v", err)
	}

	healthAddr := getenv("ARIMA_OUTBOX_HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8081"
	}
	healthServer := &http.Server{
		Addr: healthAddr, Handler: outbox.NewHealthHandler(db),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	healthErrors := make(chan error, 1)
	go func() { healthErrors <- healthServer.ListenAndServe() }()
	workerErrors := make(chan error, 1)
	go func() { workerErrors <- worker.Run(ctx) }()

	logger.Info("outbox worker started", "health_addr", healthAddr)
	select {
	case err := <-workerErrors:
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("outbox worker stopped", "error", err)
		}
		stop()
	case err := <-healthErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("outbox health server stopped", "error", err)
		}
		stop()
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("outbox health server shutdown failed", "error", err)
		_ = healthServer.Close()
	}
	logger.Info("outbox worker stopped")
}

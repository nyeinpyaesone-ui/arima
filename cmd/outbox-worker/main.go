package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info("outbox worker started")
	if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("outbox worker stopped: %v", err)
	}
	logger.Info("outbox worker stopped")
}

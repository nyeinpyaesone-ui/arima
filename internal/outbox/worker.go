package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

type Handler interface {
	// Publish must be idempotent by event ID: delivery is at-least-once.
	Publish(context.Context, store.OutboxEvent) error
}

type Worker struct {
	repository  *store.Postgres
	handler     Handler
	workerID    string
	lease       time.Duration
	deliveryTimeout time.Duration
	maxAttempts int
	pollEvery   time.Duration
	logger      *slog.Logger
}

func NewWorker(repository *store.Postgres, handler Handler, lease, pollEvery time.Duration, maxAttempts int, logger *slog.Logger) (*Worker, error) {
	if repository == nil || handler == nil {
		return nil, errors.New("outbox repository and handler are required")
	}
	if lease < time.Second || pollEvery <= 0 || maxAttempts < 1 {
		return nil, errors.New("lease must be at least one second; poll interval and max attempts must be positive")
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		repository:  repository,
		handler:     handler,
		workerID:    hex.EncodeToString(randomID[:]),
		lease:       lease,
		deliveryTimeout: lease / 2,
		maxAttempts: maxAttempts,
		pollEvery:   pollEvery,
		logger:      logger,
	}, nil
}

// ProcessOne claims and delivers at most one event for a tenant. Claim and
// acknowledgement use short tenant-scoped transactions; delivery runs outside them.
func (w *Worker) ProcessOne(ctx context.Context, tenantID string) (bool, error) {
	var event store.OutboxEvent
	err := w.repository.WithinTenant(ctx, tenantID, func(tx *store.TenantTx) error {
		var claimErr error
		event, claimErr = tx.ClaimOutbox(ctx, w.workerID, w.lease)
		return claimErr
	})
	if errors.Is(err, store.ErrNoOutboxEvent) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	deliveryCtx, cancelDelivery := context.WithTimeout(ctx, w.deliveryTimeout)
	deliveryErr := w.handler.Publish(deliveryCtx, event)
	cancelDelivery()
	ackErr := w.repository.WithinTenant(ctx, tenantID, func(tx *store.TenantTx) error {
		if deliveryErr == nil {
			return tx.MarkOutboxPublished(ctx, event.ID, w.workerID)
		}
		code := "delivery_failed"
		var coded interface{ Code() string }
		if errors.As(deliveryErr, &coded) && safeErrorCode(coded.Code()) {
			code = coded.Code()
		}
		deadLetter, err := tx.FailOutbox(ctx, event.ID, w.workerID, code, w.maxAttempts)
		if err == nil {
			w.logger.WarnContext(ctx, "outbox delivery failed", "event_id", event.ID, "event_type", event.EventType, "attempt", event.Attempts, "dead_letter", deadLetter, "error_code", code)
		}
		return err
	})
	if ackErr != nil {
		return true, ackErr
	}
	return true, nil
}

// RunTenant polls one tenant until cancellation. Tenant enumeration is deliberately
// external so a worker never needs cross-tenant access to RLS-protected event data.
func (w *Worker) RunTenant(ctx context.Context, tenantID string) error {
	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()
	for {
		processed, err := w.ProcessOne(ctx, tenantID)
		if err != nil {
			w.logger.ErrorContext(ctx, "outbox worker iteration failed", "tenant_id", tenantID, "error", err)
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func safeErrorCode(value string) bool {
	if value == "" || len(value) > 100 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

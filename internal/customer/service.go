package customer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

var (
	ErrInvalidCustomer = errors.New("invalid customer")
	ErrInvalidRequest  = errors.New("invalid customer request metadata")
)

type Customer struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email,omitempty"`
	Notes      string `json:"notes,omitempty"`
	Version    int64  `json:"version"`
}

type Service struct {
	repository *store.Postgres
}

func NewService(repository *store.Postgres) (*Service, error) {
	if repository == nil {
		return nil, errors.New("customer repository is required")
	}
	return &Service{repository: repository}, nil
}

// Create persists the customer, audit record, idempotency result and outbox event
// atomically. Permission is checked in the same transaction as the mutation.
func (s *Service) Create(ctx context.Context, principal auth.Principal, idempotencyKey, correlationID string, input Customer) (Customer, error) {
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Email = strings.TrimSpace(input.Email)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.ExternalID == "" || len(input.ExternalID) > 200 || input.Name == "" || len(input.Name) > 200 ||
		len(input.Phone) > 50 || len(input.Email) > 254 || len(input.Notes) > 2000 {
		return Customer{}, ErrInvalidCustomer
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	correlationID = strings.TrimSpace(correlationID)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 || correlationID == "" || len(correlationID) > 128 || strings.ContainsAny(correlationID, "\r\n") {
		return Customer{}, ErrInvalidRequest
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return Customer{}, fmt.Errorf("encode customer: %w", err)
	}
	var result Customer
	err = s.repository.WithinTenant(ctx, principal.TenantID, func(tx *store.TenantTx) error {
		if err := tx.RequirePermission(ctx, principal.Subject, "customer.write"); err != nil {
			return err
		}
		replay, err := tx.BeginIdempotency(ctx, idempotencyKey, payload, 24*time.Hour)
		if err != nil {
			return err
		}
		if replay.Completed {
			if err := json.Unmarshal(replay.ResponseBody, &result); err != nil {
				return fmt.Errorf("decode stored idempotency response: %w", err)
			}
			return nil
		}
		record, err := tx.UpsertRecord(ctx, "customer", input.ExternalID, 0, payload)
		if err != nil {
			return err
		}
		result = Customer{
			ID: record.ID, TenantID: record.TenantID, ExternalID: input.ExternalID,
			Name: input.Name, Phone: input.Phone, Email: input.Email, Notes: input.Notes,
			Version: record.Version,
		}
		response, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode customer response: %w", err)
		}
		if err := tx.AppendAudit(ctx, principal.Subject, "customer.created", "customer", record.ID, "succeeded", correlationID, json.RawMessage(`{"version":1}`)); err != nil {
			return err
		}
		eventPayload, err := json.Marshal(map[string]string{"customer_id": record.ID, "external_id": input.ExternalID})
		if err != nil {
			return err
		}
		if _, err := tx.EnqueueOutbox(ctx, "customer", record.ID, "customer.created", eventPayload); err != nil {
			return err
		}
		return tx.CompleteIdempotency(ctx, idempotencyKey, 201, response)
	})
	if err != nil {
		return Customer{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, principal auth.Principal, externalID string) (Customer, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" || len(externalID) > 200 {
		return Customer{}, ErrInvalidCustomer
	}
	var result Customer
	err := s.repository.WithinTenant(ctx, principal.TenantID, func(tx *store.TenantTx) error {
		if err := tx.RequirePermission(ctx, principal.Subject, "customer.read"); err != nil {
			return err
		}
		record, err := tx.GetRecord(ctx, "customer", externalID)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(record.Payload, &result); err != nil {
			return fmt.Errorf("decode customer record: %w", err)
		}
		result.ID = record.ID
		result.TenantID = record.TenantID
		result.Version = record.Version
		return nil
	})
	if err != nil {
		return Customer{}, err
	}
	return result, nil
}

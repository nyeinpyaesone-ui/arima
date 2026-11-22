package customer

import (
	"context"
	"encoding/base64"
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

type CustomerPage struct {
	Items []Customer `json:"items"`
	NextCursor string `json:"next_cursor"`
}

type customerData struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email,omitempty"`
	Notes      string `json:"notes,omitempty"`
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
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 || correlationID == "" || len(correlationID) > 128 || strings.ContainsAny(correlationID, "\r
") {
		return Customer{}, ErrInvalidRequest
	}
	data := customerData{ExternalID: input.ExternalID, Name: input.Name, Phone: input.Phone, Email: input.Email, Notes: input.Notes}
	payload, err := json.Marshal(data)
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
		eventPayload, err := json.Marshal(map[string]string{"customer_id": record.ID})
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

// List returns a bounded keyset page of customers for the authenticated tenant.
// Cursors are opaque base64url encodings of the last external ID and grant no access.
func (s *Service) List(ctx context.Context, principal auth.Principal, limit int, cursor string) (CustomerPage, error) {
	if limit == 0 { limit = 25 }
	if limit < 1 || limit > 100 { return CustomerPage{}, ErrInvalidRequest }
	after := ""
	if cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil { return CustomerPage{}, ErrInvalidRequest }
		after = string(decoded)
		if after == "" || len(after) > 200 || strings.TrimSpace(after) != after { return CustomerPage{}, ErrInvalidRequest }
	}
	page := CustomerPage{Items: make([]Customer, 0, limit)}
	err := s.repository.WithinTenant(ctx, principal.TenantID, func(tx *store.TenantTx) error {
		if err := tx.RequirePermission(ctx, principal.Subject, "customer.read"); err != nil { return err }
		records, err := tx.ListRecords(ctx, "customer", after, limit+1)
		if err != nil { return err }
		more := len(records) > limit
		if more { records = records[:limit] }
		for _, record := range records {
			var data customerData
			if err := json.Unmarshal(record.Payload, &data); err != nil { return fmt.Errorf("decode customer record: %w", err) }
			page.Items = append(page.Items, Customer{ID: record.ID, TenantID: record.TenantID, ExternalID: data.ExternalID, Name: data.Name, Phone: data.Phone, Email: data.Email, Notes: data.Notes, Version: record.Version})
		}
		if more && len(page.Items) > 0 { page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(page.Items[len(page.Items)-1].ExternalID)) }
		return nil
	})
	if err != nil { return CustomerPage{}, err }
	return page, nil
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
		var data customerData
		if err := json.Unmarshal(record.Payload, &data); err != nil {
			return fmt.Errorf("decode customer record: %w", err)
		}
		result = Customer{
			ID: record.ID, TenantID: record.TenantID, ExternalID: data.ExternalID,
			Name: data.Name, Phone: data.Phone, Email: data.Email, Notes: data.Notes,
			Version: record.Version,
		}
		return nil
	})
	if err != nil {
		return Customer{}, err
	}
	return result, nil
}

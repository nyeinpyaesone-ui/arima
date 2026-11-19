package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

type HTTPSWebhookHandler struct {
	endpoint string
	token    string
	client   *http.Client
}

type webhookEnvelope struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	CreatedAt     time.Time       `json:"created_at"`
	Payload       json.RawMessage `json:"payload"`
}

type deliveryError struct {
	code string
}

func (e deliveryError) Error() string { return "outbox webhook delivery failed: " + e.code }
func (e deliveryError) Code() string  { return e.code }
func (e deliveryError) Permanent() bool { return e.code == "webhook_rejected" || e.code == "event_too_large" }

// NewHTTPSWebhookHandler constructs an authenticated HTTPS publisher. Redirects
// are disabled so bearer credentials cannot be forwarded to another host.
func NewHTTPSWebhookHandler(endpoint, token string, client *http.Client) (*HTTPSWebhookHandler, error) {
	endpoint = strings.TrimSpace(endpoint)
	token = strings.TrimSpace(token)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("outbox webhook endpoint must be an absolute HTTPS URL without user info or fragment")
	}
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("outbox webhook bearer token is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	} else {
		clone := *client
		client = &clone
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &HTTPSWebhookHandler{endpoint: parsed.String(), token: token, client: client}, nil
}

func (h *HTTPSWebhookHandler) Publish(ctx context.Context, event store.OutboxEvent) error {
	if h == nil || h.client == nil {
		return errors.New("outbox webhook handler is not initialized")
	}
	envelope := webhookEnvelope{
		ID: event.ID, TenantID: event.TenantID, AggregateType: event.AggregateType,
		AggregateID: event.AggregateID, EventType: event.EventType,
		CreatedAt: event.CreatedAt, Payload: event.Payload,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode outbox webhook event: %w", err)
	}
	if len(body) > 256*1024 {
		return deliveryError{code: "event_too_large"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create outbox webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Idempotency-Key", event.ID)
	req.Header.Set("X-ARIMA-Event-ID", event.ID)
	response, err := h.client.Do(req)
	if err != nil {
		return deliveryError{code: "webhook_unreachable"}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
		return deliveryError{code: "webhook_retryable"}
	}
	return deliveryError{code: "webhook_rejected"}
}

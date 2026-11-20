package outbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

func TestHTTPSWebhookHandlerPublishesIdempotentEnvelope(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("authorization header was not set")
		}
		if r.Header.Get("Idempotency-Key") != "event-123" || r.Header.Get("X-ARIMA-Event-ID") != "event-123" {
			t.Errorf("event idempotency headers missing")
		}
		var payload webhookEnvelope
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode webhook payload: %v", err)
		}
		if payload.ID != "event-123" || payload.EventType != "customer.created" || string(payload.Payload) != `{"customer_id":"customer-1"}` {
			t.Errorf("unexpected webhook envelope: %#v", payload)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	handler, err := NewHTTPSWebhookHandler(server.URL, "test-secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	err = handler.Publish(context.Background(), store.OutboxEvent{
		ID: "event-123", TenantID: "11111111-1111-4111-8111-111111111111",
		AggregateType: "customer", AggregateID: "customer-1",
		EventType: "customer.created", Payload: []byte(`{"customer_id":"customer-1"}`),
	})
	if err != nil {
		t.Fatalf("publish event: %v", err)
	}
}

func TestHTTPSWebhookHandlerRequiresHTTPSAndToken(t *testing.T) {
	if _, err := NewHTTPSWebhookHandler("http://example.test/events", "secret", nil); err == nil {
		t.Fatal("HTTP endpoint must be rejected")
	}
	if _, err := NewHTTPSWebhookHandler("https://example.test/events", "", nil); err == nil {
		t.Fatal("empty token must be rejected")
	}
}

func TestHTTPSWebhookHandlerClassifiesRetryableAndPermanentResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		wantCode string
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, wantCode: "webhook_retryable"},
		{name: "server error", status: http.StatusBadGateway, wantCode: "webhook_retryable"},
		{name: "bad request", status: http.StatusBadRequest, wantCode: "webhook_rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			handler, err := NewHTTPSWebhookHandler(server.URL, "test-secret", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			err = handler.Publish(context.Background(), store.OutboxEvent{ID: "event-456", Payload: []byte(`{}`)})
			if err == nil {
				t.Fatal("non-2xx response must return an error")
			}
			coded, ok := err.(interface{ Code() string })
			if !ok || coded.Code() != tc.wantCode {
				t.Fatalf("error code = %v, want %s", err, tc.wantCode)
			}
		})
	}
}

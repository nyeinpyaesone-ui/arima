package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/nyeinpyaesone-ui/arima/internal/auth"
	"github.com/nyeinpyaesone-ui/arima/internal/customer"
	"github.com/nyeinpyaesone-ui/arima/internal/store"
)

type customerHandler struct {
	service *customer.Service
}

type createCustomerRequest struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

func registerCustomerRoutes(mux *http.ServeMux, verifier auth.TokenVerifier, service *customer.Service) {
	h := &customerHandler{service: service}
	mux.Handle("POST /api/v1/customers", auth.RequireBearer(verifier, http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/customers/{externalID}", auth.RequireBearer(verifier, http.HandlerFunc(h.get)))
}

func (h *customerHandler) create(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		http.Error(w, "customer service is not configured", http.StatusServiceUnavailable)
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input createCustomerRequest
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid customer request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "request must contain exactly one JSON object", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	correlationID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if correlationID == "" {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			http.Error(w, "request initialization failed", http.StatusInternalServerError)
			return
		}
		correlationID = hex.EncodeToString(raw[:])
	}
	if len(correlationID) > 128 || strings.ContainsAny(correlationID, "\r\n") {
		http.Error(w, "invalid request correlation ID", http.StatusBadRequest)
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	result, err := h.service.Create(r.Context(), principal, key, correlationID, customer.Customer{
		ExternalID: input.ExternalID, Name: input.Name, Phone: input.Phone,
		Email: input.Email, Notes: input.Notes,
	})
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *customerHandler) get(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		http.Error(w, "customer service is not configured", http.StatusServiceUnavailable)
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	result, err := h.service.Get(r.Context(), principal, r.PathValue("externalID"))
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeCustomerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, customer.ErrInvalidCustomer):
		http.Error(w, "invalid customer", http.StatusBadRequest)
	case errors.Is(err, store.ErrInvalidTenant):
		http.Error(w, "invalid tenant identity", http.StatusUnauthorized)
	case errors.Is(err, store.ErrPermissionDenied):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, store.ErrVersionConflict):
		http.Error(w, "customer already exists", http.StatusConflict)
	case errors.Is(err, store.ErrIdempotencyConflict):
		http.Error(w, "idempotency key was used with a different request", http.StatusConflict)
	case errors.Is(err, store.ErrIdempotencyPending):
		http.Error(w, "request with this idempotency key is still in progress", http.StatusConflict)
	case errors.Is(err, store.ErrRecordNotFound):
		http.Error(w, "customer not found", http.StatusNotFound)
	default:
		http.Error(w, "customer operation failed", http.StatusInternalServerError)
	}
}

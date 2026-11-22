package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
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
	mux.Handle("GET /api/v1/customers", auth.RequireBearer(verifier, http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/customers/{externalID}", auth.RequireBearer(verifier, http.HandlerFunc(h.get)))
}

func (h *customerHandler) create(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		http.Error(w, "customer service is not configured", http.StatusServiceUnavailable)
		return
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input createCustomerRequest
	if err := decoder.Decode(&input); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
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
	if len(correlationID) > 128 || !safeHeaderToken(correlationID) {
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

func (h *customerHandler) list(w http.ResponseWriter, r *http.Request) {
	if h.service == nil { http.Error(w, "customer service is not configured", http.StatusServiceUnavailable); return }
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 { http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest); return }
		limit = parsed
	}
	cursor := r.URL.Query().Get("after")
	if len(cursor) > 512 { http.Error(w, "invalid pagination cursor", http.StatusBadRequest); return }
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
	result, err := h.service.List(r.Context(), principal, limit, cursor)
	if err != nil { writeCustomerError(w, err); return }
	writeJSON(w, http.StatusOK, result)
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

func safeHeaderToken(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return value != ""
}

func writeCustomerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, customer.ErrInvalidCustomer), errors.Is(err, customer.ErrInvalidRequest):
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

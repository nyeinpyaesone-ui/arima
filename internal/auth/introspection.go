package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrVerifierNotConfigured = errors.New("OIDC introspection configuration is incomplete")

type IntrospectionConfig struct {
	Endpoint string
	Issuer   string
	Audience string
	ClientID string
	Secret   string
}

type IntrospectionVerifier struct {
	endpoint string
	issuer   string
	audience string
	clientID string
	secret   string
	client   *http.Client
	now      func() time.Time
}

type introspectionResponse struct {
	Active    bool            `json:"active"`
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	Subject   string          `json:"sub"`
	TenantID  string          `json:"tenant_id"`
	Expires   json.Number     `json:"exp"`
	NotBefore json.Number     `json:"nbf"`
}

func NewIntrospectionVerifier(cfg IntrospectionConfig) (*IntrospectionVerifier, error) {
	fields := []string{cfg.Endpoint, cfg.Issuer, cfg.Audience, cfg.ClientID, cfg.Secret}
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return nil, ErrVerifierNotConfigured
		}
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("introspection endpoint must be an absolute HTTPS URL without userinfo or fragment")
	}
	if net.ParseIP(u.Hostname()) != nil {
		return nil, fmt.Errorf("introspection endpoint must use a DNS hostname")
	}
	return &IntrospectionVerifier{
		endpoint: u.String(),
		issuer: strings.TrimSpace(cfg.Issuer),
		audience: strings.TrimSpace(cfg.Audience),
		clientID: strings.TrimSpace(cfg.ClientID),
		secret: cfg.Secret,
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
	}, nil
}

// NewIntrospectionVerifierFromEnv treats a wholly absent configuration as disabled,
// but rejects partial configuration so deployments cannot accidentally appear authenticated.
func NewIntrospectionVerifierFromEnv(getenv func(string) string) (TokenVerifier, error) {
	if getenv == nil {
		return nil, fmt.Errorf("environment reader must not be nil")
	}
	cfg := IntrospectionConfig{
		Endpoint: strings.TrimSpace(getenv("ARIMA_OIDC_INTROSPECTION_URL")),
		Issuer: strings.TrimSpace(getenv("ARIMA_OIDC_ISSUER")),
		Audience: strings.TrimSpace(getenv("ARIMA_OIDC_AUDIENCE")),
		ClientID: strings.TrimSpace(getenv("ARIMA_OIDC_CLIENT_ID")),
		Secret: getenv("ARIMA_OIDC_CLIENT_SECRET"),
	}
	values := []string{cfg.Endpoint, cfg.Issuer, cfg.Audience, cfg.ClientID, cfg.Secret}
	set := 0
	for _, value := range values {
		if value != "" {
			set++
		}
	}
	if set == 0 {
		return nil, nil
	}
	verifier, err := NewIntrospectionVerifier(cfg)
	if err != nil {
		return nil, err
	}
	return verifier, nil
}

// Verify delegates token validation to the configured authorization server over TLS,
// then independently checks issuer, audience, time, subject, and tenant claims.
func (v *IntrospectionVerifier) Verify(ctx context.Context, token string) (Principal, error) {
	if v == nil {
		return Principal{}, ErrVerifierNotConfigured
	}
	form := url.Values{"token": []string{token}, "token_type_hint": []string{"access_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(v.clientID, v.secret)

	resp, err := v.client.Do(req)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Principal{}, ErrInvalidToken
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return Principal{}, ErrInvalidToken
	}
	var data introspectionResponse
	if err := json.Unmarshal(body, &data); err != nil || !data.Active {
		return Principal{}, ErrInvalidToken
	}
	if data.Issuer != v.issuer || !audienceContains(data.Audience, v.audience) {
		return Principal{}, ErrInvalidToken
	}
	exp, err := strconv.ParseInt(string(data.Expires), 10, 64)
	if err != nil || exp <= v.now().Unix() {
		return Principal{}, ErrInvalidToken
	}
	if data.NotBefore != "" {
		nbf, err := strconv.ParseInt(string(data.NotBefore), 10, 64)
		if err != nil || nbf > v.now().Unix() {
			return Principal{}, ErrInvalidToken
		}
	}
	if strings.TrimSpace(data.Subject) == "" || strings.TrimSpace(data.TenantID) == "" {
		return Principal{}, ErrInvalidToken
	}
	return Principal{Subject: data.Subject, TenantID: data.TenantID}, nil
}

func audienceContains(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) == nil {
		for _, value := range multiple {
			if value == expected {
				return true
			}
		}
	}
	return false
}

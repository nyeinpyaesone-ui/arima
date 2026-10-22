package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func verifierForResponse(status int, body string) *IntrospectionVerifier {
	v := &IntrospectionVerifier{
		endpoint: "https://idp.example.test/oauth/introspect",
		issuer: "https://idp.example.test/",
		audience: "arima-api",
		clientID: "arima-client",
		secret: "test-secret",
		now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	}
	v.client = &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		user, pass, ok := req.BasicAuth()
		if !ok || user != "arima-client" || pass != "test-secret" {
			return nil, fmt.Errorf("unexpected client authentication")
		}
		if req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			return nil, fmt.Errorf("unexpected introspection request")
		}
		raw, err := io.ReadAll(req.Body)
		if err != nil || !strings.Contains(string(raw), "token=example-token") {
			return nil, fmt.Errorf("token was not sent as form data")
		}
		return &http.Response{
			StatusCode: status,
			Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(body)),
			Request: req,
		}, nil
	})}
	return v
}

func TestIntrospectionVerifierAcceptsActiveMatchingToken(t *testing.T) {
	body := `{"active":true,"iss":"https://idp.example.test/","aud":["other","arima-api"],"sub":"user-42","tenant_id":"tenant-7","exp":1800000100,"nbf":1799999900}`
	got, err := verifierForResponse(http.StatusOK, body).Verify(context.Background(), "example-token")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Principal{Subject: "user-42", TenantID: "tenant-7"}) {
		t.Fatalf("principal = %#v", got)
	}
}

func TestIntrospectionVerifierRejectsInvalidResponses(t *testing.T) {
	cases := []struct {
		name string
		status int
		body string
	}{
		{"inactive", http.StatusOK, `{"active":false}`},
		{"issuer mismatch", http.StatusOK, `{"active":true,"iss":"https://attacker.example/","aud":"arima-api","sub":"u","tenant_id":"t","exp":1800000100}`},
		{"audience mismatch", http.StatusOK, `{"active":true,"iss":"https://idp.example.test/","aud":"other-api","sub":"u","tenant_id":"t","exp":1800000100}`},
		{"expired", http.StatusOK, `{"active":true,"iss":"https://idp.example.test/","aud":"arima-api","sub":"u","tenant_id":"t","exp":1799999999}`},
		{"not yet valid", http.StatusOK, `{"active":true,"iss":"https://idp.example.test/","aud":"arima-api","sub":"u","tenant_id":"t","exp":1800000100,"nbf":1800000001}`},
		{"missing tenant", http.StatusOK, `{"active":true,"iss":"https://idp.example.test/","aud":"arima-api","sub":"u","exp":1800000100}`},
		{"provider error", http.StatusUnauthorized, `{"active":true}`},
		{"invalid json", http.StatusOK, `not-json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := verifierForResponse(tc.status, tc.body).Verify(context.Background(), "example-token"); err == nil {
				t.Fatal("expected token to be rejected")
			}
		})
	}
}

func TestNewIntrospectionVerifierRequiresHTTPSAndCompleteConfiguration(t *testing.T) {
	base := IntrospectionConfig{
		Endpoint: "https://idp.example.test/introspect",
		Issuer: "https://idp.example.test/",
		Audience: "arima-api",
		ClientID: "arima-client",
		Secret: "secret",
	}
	if _, err := NewIntrospectionVerifier(base); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	base.Endpoint = "http://idp.example.test/introspect"
	if _, err := NewIntrospectionVerifier(base); err == nil {
		t.Fatal("expected non-HTTPS endpoint to be rejected")
	}
	base.Endpoint = "https://idp.example.test/introspect"
	base.Secret = ""
	if _, err := NewIntrospectionVerifier(base); err == nil {
		t.Fatal("expected incomplete configuration to be rejected")
	}
}

func TestVerifierEnvironmentMustBeAllOrNone(t *testing.T) {
	if verifier, err := NewIntrospectionVerifierFromEnv(func(string) string { return "" }); err != nil || verifier != nil {
		t.Fatalf("empty config: verifier=%v err=%v", verifier, err)
	}
	if _, err := NewIntrospectionVerifierFromEnv(func(key string) string {
		if key == "ARIMA_OIDC_ISSUER" { return "https://idp.example.test/" }
		return ""
	}); err == nil {
		t.Fatal("expected partial configuration to fail")
	}
}

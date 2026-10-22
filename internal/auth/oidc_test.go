package auth

import (
	"context"
	"testing"
)

func TestNewOIDCVerifierFromEnvRequiresIssuerAndAudienceTogether(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   bool
	}{
		{name: "disabled when both absent", values: map[string]string{}, want: false},
		{name: "issuer only rejected", values: map[string]string{"ARIMA_OIDC_ISSUER": "https://issuer.example.test"}, want: true},
		{name: "audience only rejected", values: map[string]string{"ARIMA_OIDC_AUDIENCE": "arima-api"}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verifier, err := NewOIDCVerifierFromEnv(context.Background(), func(key string) string {
				return tc.values[key]
			})
			if tc.want {
				if err == nil || verifier != nil {
					t.Fatalf("expected configuration error; verifier=%v err=%v", verifier, err)
				}
				return
			}
			if err != nil || verifier != nil {
				t.Fatalf("expected disabled verifier; verifier=%v err=%v", verifier, err)
			}
		})
	}
}

func TestNewOIDCVerifierRejectsMissingConfiguration(t *testing.T) {
	if _, err := NewOIDCVerifier(context.Background(), "", "arima-api"); err == nil {
		t.Fatal("expected missing issuer to be rejected")
	}
	if _, err := NewOIDCVerifier(context.Background(), "https://issuer.example.test", ""); err == nil {
		t.Fatal("expected missing audience to be rejected")
	}
}

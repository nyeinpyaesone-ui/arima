package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

var ErrOIDCNotConfigured = errors.New("OIDC issuer and audience must both be configured")

// OIDCVerifier delegates JWT signature verification to the maintained go-oidc
// implementation. Its remote key set refreshes provider keys as signing keys rotate.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, issuer, audience string) (*OIDCVerifier, error) {
	issuer = strings.TrimSpace(issuer)
	audience = strings.TrimSpace(audience)
	if issuer == "" || audience == "" {
		return nil, ErrOIDCNotConfigured
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return &OIDCVerifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
	}, nil
}

// Verify validates the JWT signature, exact issuer, audience, expiry and not-before
// using the provider's discovery metadata and rotating JWKS, then requires tenant identity.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (Principal, error) {
	if v == nil || v.verifier == nil {
		return Principal{}, ErrOIDCNotConfigured
	}
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	var claims struct {
		TenantID string `json:"tenant_id"`
	}
	if err := token.Claims(&claims); err != nil {
		return Principal{}, ErrInvalidToken
	}
	if strings.TrimSpace(token.Subject) == "" || strings.TrimSpace(claims.TenantID) == "" {
		return Principal{}, ErrInvalidToken
	}
	return Principal{Subject: token.Subject, TenantID: claims.TenantID}, nil
}

// NewOIDCVerifierFromEnv treats wholly absent configuration as disabled outside
// production; partial configuration is rejected to prevent silent misconfiguration.
func NewOIDCVerifierFromEnv(ctx context.Context, getenv func(string) string) (TokenVerifier, error) {
	if getenv == nil {
		return nil, errors.New("environment reader must not be nil")
	}
	issuer := strings.TrimSpace(getenv("ARIMA_OIDC_ISSUER"))
	audience := strings.TrimSpace(getenv("ARIMA_OIDC_AUDIENCE"))
	if issuer == "" && audience == "" {
		return nil, nil
	}
	if issuer == "" || audience == "" {
		return nil, ErrOIDCNotConfigured
	}
	return NewOIDCVerifier(ctx, issuer, audience)
}

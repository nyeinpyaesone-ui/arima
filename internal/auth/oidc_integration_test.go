package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

func TestOIDCVerifierValidatesClaimsAndRefreshesRotatedKeys(t *testing.T) {
	keyOne, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyTwo, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.RWMutex
	currentKey := publicJWK("key-one", &keyOne.PublicKey)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                "https://" + r.Host,
				"jwks_uri":                              "https://" + r.Host + "/keys",
				"response_types_supported":              []string{"id_token"},
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			mu.RLock()
			key := currentKey
			mu.RUnlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{key}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	issuer := server.URL
	// Discovery's issuer must exactly match the HTTPS test server URL.
	verifierContext := oidc.ClientContext(context.Background(), server.Client())
	verifier, err := NewOIDCVerifier(verifierContext, issuer, "arima-api")
	if err != nil {
		t.Fatalf("create OIDC verifier: %v", err)
	}

	tenantID := "11111111-1111-4111-8111-111111111111"
	validToken := signOIDCTestToken(t, keyOne, "key-one", issuer, "arima-api", "subject-1", tenantID, time.Now().Add(5*time.Minute))
	principal, err := verifier.Verify(context.Background(), validToken)
	if err != nil {
		t.Fatalf("verify valid token: %v", err)
	}
	if principal.Subject != "subject-1" || principal.TenantID != tenantID {
		t.Fatalf("unexpected verified principal: %#v", principal)
	}

	mu.Lock()
	currentKey = publicJWK("key-two", &keyTwo.PublicKey)
	mu.Unlock()
	rotatedToken := signOIDCTestToken(t, keyTwo, "key-two", issuer, "arima-api", "subject-2", tenantID, time.Now().Add(5*time.Minute))
	rotatedPrincipal, err := verifier.Verify(context.Background(), rotatedToken)
	if err != nil {
		t.Fatalf("verify token after JWKS rotation: %v", err)
	}
	if rotatedPrincipal.Subject != "subject-2" {
		t.Fatalf("rotated token subject = %q", rotatedPrincipal.Subject)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "wrong audience", token: signOIDCTestToken(t, keyTwo, "key-two", issuer, "other-api", "subject-3", tenantID, time.Now().Add(5*time.Minute))},
		{name: "expired", token: signOIDCTestToken(t, keyTwo, "key-two", issuer, "arima-api", "subject-4", tenantID, time.Now().Add(-time.Minute))},
		{name: "missing tenant", token: signOIDCTestToken(t, keyTwo, "key-two", issuer, "arima-api", "subject-5", "", time.Now().Add(5*time.Minute))},
		{name: "wrong issuer", token: signOIDCTestToken(t, keyTwo, "key-two", "https://wrong-issuer.example", "arima-api", "subject-6", tenantID, time.Now().Add(5*time.Minute))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := verifier.Verify(context.Background(), tc.token); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("invalid token must be rejected as ErrInvalidToken, got %v", err)
			}
		})
	}
}

func publicJWK(kid string, publicKey *rsa.PublicKey) map[string]string {
	exponent := big.NewInt(int64(publicKey.E)).Bytes()
	return map[string]string{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(exponent),
	}
}

func signOIDCTestToken(t *testing.T, key *rsa.PrivateKey, kid, issuer, audience, subject, tenantID string, expiry time.Time) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{
		"iss": issuer, "aud": audience, "sub": subject,
		"exp": expiry.Unix(), "iat": time.Now().Unix(),
	}
	if tenantID != "" {
		claims["tenant_id"] = tenantID
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

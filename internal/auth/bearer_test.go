package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type verifierFunc func(context.Context, string) (Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token string) (Principal, error) {
	return f(ctx, token)
}

func TestRequireBearerRejectsMissingOrMalformedCredentials(t *testing.T) {
	handler := RequireBearer(verifierFunc(func(context.Context, string) (Principal, error) {
		t.Fatal("verifier must not be called for malformed credentials")
		return Principal{}, nil
	}), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next must not be called") }))
	for _, header := range []string{"", "Basic abc", "Bearer", "Bearer abc def"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", header)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Errorf("header %q: got %d, want %d", header, res.Code, http.StatusUnauthorized)
		}
	}
}

func TestRequireBearerFailsClosedWhenVerifierMissing(t *testing.T) {
	res := httptest.NewRecorder()
	RequireBearer(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next must not be called") })).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestRequireBearerRejectsInvalidOrIncompletePrincipal(t *testing.T) {
	cases := []struct {
		name      string
		principal Principal
		err       error
	}{
		{name: "invalid token", principal: Principal{}, err: ErrInvalidToken},
		{name: "missing subject", principal: Principal{TenantID: "tenant-a"}},
		{name: "missing tenant", principal: Principal{Subject: "user-a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := RequireBearer(verifierFunc(func(context.Context, string) (Principal, error) { return tc.principal, tc.err }), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next must not be called") }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer token-value")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("got %d, want %d", res.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestRequireBearerAddsVerifiedPrincipalToContext(t *testing.T) {
	want := Principal{Subject: "user-1", TenantID: "tenant-1"}
	handler := RequireBearer(verifierFunc(func(_ context.Context, token string) (Principal, error) {
		if token != "good-token" {
			return Principal{}, errors.New("unexpected token")
		}
		return want, nil
	}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := PrincipalFromContext(r.Context())
		if !ok || got != want {
			t.Errorf("principal = %#v, ok = %v; want %#v", got, ok, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("got %d, want %d", res.Code, http.StatusNoContent)
	}
}

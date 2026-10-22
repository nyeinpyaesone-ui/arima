package auth

import (
    "context"
    "errors"
    "net/http"
    "strings"
)

var ErrInvalidToken = errors.New("invalid bearer token")

type Principal struct {
    Subject  string
    TenantID string
}

type principalContextKey struct{}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
    p, ok := ctx.Value(principalContextKey{}).(Principal)
    return p, ok
}

type TokenVerifier interface {
    Verify(ctx context.Context, token string) (Principal, error)
}

// RequireBearer verifies credentials through an explicitly supplied trusted verifier.
// A nil verifier fails closed; this package intentionally does not implement JWT parsing.
func RequireBearer(verifier TokenVerifier, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if verifier == nil {
            http.Error(w, "authentication is not configured", http.StatusServiceUnavailable)
            return
        }
        header := r.Header.Get("Authorization")
        scheme, token, ok := strings.Cut(header, " ")
        if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" || strings.ContainsAny(token, " \t\r\n") {
            w.Header().Set("WWW-Authenticate", "Bearer")
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        principal, err := verifier.Verify(r.Context(), token)
        if err != nil || strings.TrimSpace(principal.Subject) == "" || strings.TrimSpace(principal.TenantID) == "" {
            w.Header().Set("WWW-Authenticate", "Bearer")
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

var ErrPermissionDenied = errors.New("permission denied")

// Authorizer evaluates a permission for an authenticated principal.
// Implementations must enforce tenant membership and role/policy rules server-side.
type Authorizer interface {
	Authorize(ctx context.Context, principal Principal, permission string) error
}

// RequirePermission must wrap a handler inside RequireBearer. Missing policy
// configuration fails closed; authorization errors never grant access.
func RequirePermission(authorizer Authorizer, permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authorizer == nil || strings.TrimSpace(permission) == "" {
			http.Error(w, "authorization is not configured", http.StatusServiceUnavailable)
			return
		}
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := authorizer.Authorize(r.Context(), principal, permission); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

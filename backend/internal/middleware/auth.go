package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

type authUserKey struct{}

// WithAuthUser stores the principal in the context.
func WithAuthUser(ctx context.Context, u *model.AuthUser) context.Context {
	return context.WithValue(ctx, authUserKey{}, u)
}

// AuthUserFrom extracts the principal. The second return is false when absent.
func AuthUserFrom(ctx context.Context) (*model.AuthUser, bool) {
	u, ok := ctx.Value(authUserKey{}).(*model.AuthUser)
	return u, ok
}

// RequireAuth rejects requests without a valid Bearer token (401).
func RequireAuth(parse func(string) (*model.AuthUser, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || strings.TrimSpace(token) == "" {
				apierror.Write(w, r, http.StatusUnauthorized, "MISSING_TOKEN", "Token autentikasi wajib diisi.", nil)
				return
			}
			u, err := parse(strings.TrimSpace(token))
			if err != nil {
				apierror.Write(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "Token tidak valid atau kedaluwarsa.", nil)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAuthUser(r.Context(), u)))
		})
	}
}

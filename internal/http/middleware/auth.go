package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// Authenticator resolves a raw session token.
//
// It is an interface rather than *auth.Service so this package stays
// independent of the authentication implementation and can be tested with a
// double.
type Authenticator interface {
	Authenticate(ctx context.Context, rawToken string) (*domain.Session, *domain.User, error)
}

// RequireAuth rejects requests that do not carry a usable session and publishes
// the verified user and session into the request context.
//
// The session is re-resolved on every request rather than trusted from a signed
// token, which is what makes revocation take effect immediately: once the
// session is revoked, the very next request fails here.
func RequireAuth(authenticator Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := handler.BearerToken(r)
			if !ok {
				unauthorized(w)
				return
			}

			session, user, err := authenticator.Authenticate(r.Context(), token)
			if err != nil {
				if errors.Is(err, domain.ErrUnauthorized) {
					unauthorized(w)
					return
				}
				handler.RespondWithError(w, http.StatusInternalServerError,
					"internal_error", "Ocurrió un error inesperado.", nil)
				return
			}

			ctx := handler.WithAuthenticated(r.Context(), user, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// unauthorized answers with the uniform error contract. Expired, revoked and
// unknown tokens produce the same response, so a caller cannot tell which of
// them applies.
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	handler.RespondWithError(w, http.StatusUnauthorized, "unauthorized",
		"Se requiere una sesión válida.", nil)
}

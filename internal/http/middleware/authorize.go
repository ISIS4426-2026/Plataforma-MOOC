package middleware

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// RequireRole rejects an authenticated request whose user does not hold one of
// the given roles.
//
// It must be composed inside RequireAuth: it reads the user that middleware
// published, and a request that never went through it has none. An
// unauthenticated request therefore answers 401 from RequireAuth rather than
// reaching this check at all, which is what keeps 403 meaning "you are known
// and not allowed" instead of "you are not known".
//
// The refusal is logged: an account attempting an operation its role does not
// permit is exactly the kind of thing an audit review looks for.
func RequireRole(logger *slog.Logger, roles ...domain.Role) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := handler.UserFromContext(r.Context())
			if !ok {
				// Reaching here means the chain was assembled wrong. Answering
				// 401 is the safe outcome, and the log says why.
				logger.ErrorContext(r.Context(), "role check ran on an unauthenticated request",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("path", r.URL.Path),
				)
				handler.RespondWithError(w, http.StatusUnauthorized, "unauthorized",
					"Se requiere una sesión válida.", nil)
				return
			}

			if !slices.Contains(roles, user.Role) {
				logger.WarnContext(r.Context(), "role not permitted for this operation",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("user_id", user.ID),
					slog.String("role", string(user.Role)),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				handler.RespondWithError(w, http.StatusForbidden, "forbidden",
					"No tienes permiso para realizar esta operación.", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is the common case: administrators only.
func RequireAdmin(logger *slog.Logger) Middleware {
	return RequireRole(logger, domain.RoleAdmin)
}

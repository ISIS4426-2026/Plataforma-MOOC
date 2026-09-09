package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// Recoverer turns a panic in any handler into a uniform 500 response instead of
// letting net/http drop the connection with an empty reply.
//
// It must sit inside RequestLogger: the logger records the status only after
// next.ServeHTTP returns, so a panic escaping past it would skip the access log
// entry entirely. Recovering first lets the logger observe the 500 it produced.
// The stack trace is written to the log only, while the response body carries
// the generic error contract so internals never reach the client.
func Recoverer(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				// http.ErrAbortHandler is the documented way for a handler to
				// abort a response on purpose; suppressing it here would hide
				// an intentional signal, so it is re-raised for net/http.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				logger.ErrorContext(r.Context(), "panic recovered in http handler",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)

				handler.RespondWithError(w, http.StatusInternalServerError,
					"internal_error", "An unexpected error occurred.", nil)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

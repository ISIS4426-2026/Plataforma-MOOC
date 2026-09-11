package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// RequestLogger emits one structured record per request with the fields the
// operational criteria expect: correlation id, method, route, status and
// duration.
//
// Only the URL path is recorded, never the raw query string: email verification
// and password recovery links carry single-use tokens as query parameters, and
// writing them to the access log would turn the log into a credential store.
// Request and response bodies are likewise never logged, which is what keeps
// passwords out of them (issue #10).
func RequestLogger(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := newResponseRecorder(w)

			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			attrs := []any{
				slog.String("request_id", RequestIDFrom(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.Int("bytes", recorder.bytes),
				slog.String("duración", duration.String()),
				slog.Int64("duración_ms", duration.Milliseconds()),
			}
			// Present only when Tracing runs earlier in the chain and started
			// a span for this request; ties this log line to that span
			// (issue #21: "logs y trazas correlacionados mediante un
			// identificador común").
			if traceID := TraceIDFrom(r.Context()); traceID != "" {
				attrs = append(attrs, slog.String("trace_id", traceID))
			}

			// Server-side failures are the ones an operator needs to see by
			// default; client errors stay at info so a scan of 404s does not
			// drown real incidents.
			if recorder.status >= http.StatusInternalServerError {
				logger.ErrorContext(r.Context(), "http request failed", attrs...)
				return
			}

			logger.InfoContext(r.Context(), "http request", attrs...)
		})
	}
}

package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// HeaderRequestID carries the correlation identifier in and out of the API.
const HeaderRequestID = "X-Request-ID"

// contextKey is unexported so no other package can collide with our keys in the
// request context.
type contextKey struct{ name string }

var requestIDKey = &contextKey{name: "request-id"}

// RequestID assigns every request a correlation identifier, reusing an inbound
// X-Request-ID when the caller supplies one so a trace survives across the
// frontend, the API and the workers.
//
// The identifier is echoed back on the response, which is what lets the demo
// evidence tie an API response to its logs (section 10.1 of the specification).
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(HeaderRequestID)
			if !isValidRequestID(id) {
				id = uuid.NewString()
			}

			w.Header().Set(HeaderRequestID, id)
			ctx := context.WithValue(r.Context(), requestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFrom returns the correlation identifier stored in ctx, or an empty
// string when the request did not pass through the RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// isValidRequestID guards against a client injecting arbitrary content into the
// logs or the response header. Anything that is not a plausible identifier is
// discarded in favour of a freshly generated one.
func isValidRequestID(id string) bool {
	const maxLen = 128
	if id == "" || len(id) > maxLen {
		return false
	}

	for _, c := range id {
		isAllowed := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.'
		if !isAllowed {
			return false
		}
	}

	return true
}

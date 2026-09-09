// Package middleware provides the cross-cutting HTTP layer shared by every
// /api/v1 endpoint: request correlation, structured access logging and panic
// recovery.
//
// Security middleware that is specific to authentication (CSRF, rate limiting)
// is layered on top of this chain rather than replacing it.
package middleware

import (
	"net/http"
	"slices"
)

// Middleware decorates an http.Handler with behaviour that runs around it.
type Middleware func(http.Handler) http.Handler

// Chain composes middlewares around a handler so that the first argument is the
// outermost layer.
//
// Chain(h, A, B) yields A(B(h)): A sees the request first and the response last,
// which is what makes ordering such as Recoverer before Logger meaningful.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	// Applied back to front so the earliest listed middleware ends up wrapping
	// everything else.
	for _, mw := range slices.Backward(middlewares) {
		h = mw(h)
	}
	return h
}

// responseRecorder captures the status code and response size, which the
// ResponseWriter interface does not expose after the fact.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	// A handler that writes a body without calling WriteHeader implicitly
	// produces 200, so that is the correct starting value.
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(status int) {
	// net/http ignores repeated WriteHeader calls; mirroring that here keeps the
	// recorded status equal to the one actually sent.
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

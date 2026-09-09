package handler

import (
	"context"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// authContextKey is unexported so no other package can write these values into
// a request context. Only the authentication middleware can populate them,
// which means a handler reading them is reading something the middleware
// actually verified.
type authContextKey struct{ name string }

var (
	userContextKey    = &authContextKey{name: "user"}
	sessionContextKey = &authContextKey{name: "session"}
)

// WithAuthenticated returns a context carrying the verified user and session.
// It is called by the authentication middleware, never by a handler.
func WithAuthenticated(ctx context.Context, user *domain.User, session *domain.Session) context.Context {
	ctx = context.WithValue(ctx, userContextKey, user)
	return context.WithValue(ctx, sessionContextKey, session)
}

// UserFromContext returns the authenticated user. The second result is false on
// an unauthenticated request, which handlers must treat as 401 rather than
// dereferencing a nil user.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(userContextKey).(*domain.User)
	return user, ok && user != nil
}

// SessionFromContext returns the session backing the current request.
func SessionFromContext(ctx context.Context) (*domain.Session, bool) {
	session, ok := ctx.Value(sessionContextKey).(*domain.Session)
	return session, ok && session != nil
}

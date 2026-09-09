package domain

import (
	"context"
	"time"
)

// Session is an authenticated login, addressable only by the hash of the token
// handed to the client.
//
// The raw token exists in exactly two places: the response to a successful
// login and the client that stores it. Persisting only its SHA-256 hash means a
// database or cache dump cannot be replayed as a valid credential.
type Session struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	TokenHash      string     `json:"-"`
	UserAgent      string     `json:"user_agent"`
	IPAddress      string     `json:"ip_address"`
	IsRevoked      bool       `json:"is_revoked"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	LastActivityAt time.Time  `json:"last_activity_at"`
}

// IsUsable reports whether the session may still authenticate a request at the
// given instant.
func (s *Session) IsUsable(now time.Time) bool {
	return !s.IsRevoked && now.Before(s.ExpiresAt)
}

// SessionRepository is the durable record of sessions.
//
// PostgreSQL is the source of truth: it survives a cache flush, backs the
// administrative listing and revocation of sessions, and keeps the audit trail
// of when each session was created and revoked.
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	ListActiveByUser(ctx context.Context, userID string) ([]*Session, error)
	// Revoke marks a single session revoked and reports ErrNotFound when the
	// session does not exist or was already revoked.
	Revoke(ctx context.Context, sessionID string) error
	// RevokeAllForUser revokes every active session of a user and returns how
	// many were affected. Changing a password relies on this.
	RevokeAllForUser(ctx context.Context, userID string) (int, error)
}

// SessionCache is the fast path consulted on every authenticated request.
//
// Redis holds live sessions with a native TTL so expiry needs no sweeper, and a
// deletion takes effect on the next request, which is what makes revocation
// immediate. It is a cache, not the source of truth: a miss falls back to
// SessionRepository, so losing Redis costs latency rather than logging every
// user out.
type SessionCache interface {
	Save(ctx context.Context, session *Session, ttl time.Duration) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	Delete(ctx context.Context, tokenHash string) error
	// DeleteAllForUser drops every cached session of a user, so a password
	// change or an administrative revocation cannot be outlived by a warm cache
	// entry.
	DeleteAllForUser(ctx context.Context, userID string) error
}

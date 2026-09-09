package domain

import (
	"context"
	"time"
)

// VerificationToken is a single-use credential mailed to a user, used both to
// confirm an email address at registration and to authorise a password reset.
//
// Only the SHA-256 hash is stored; the raw value lives in the message sent to
// the user. Consumption is recorded with ConsumedAt rather than deleting the
// row, so "this token was already used" stays distinguishable from "this token
// never existed" for auditing.
type VerificationToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// VerificationTokenRepository stores the single-use tokens for one purpose.
// Email verification and password reset use separate implementations backed by
// separate tables, so a token issued for one flow can never be replayed in the
// other.
type VerificationTokenRepository interface {
	Create(ctx context.Context, token *VerificationToken) error

	// Consume atomically marks the token used and returns it.
	//
	// The check for "unused and unexpired" and the write that marks it used must
	// happen in a single statement: reading first and updating afterwards lets
	// two concurrent requests both observe an unused token and both succeed,
	// which would defeat the single-use guarantee. Returns ErrNotFound when the
	// token is unknown, already consumed or expired, so callers cannot tell the
	// three apart.
	Consume(ctx context.Context, tokenHash string, now time.Time) (*VerificationToken, error)

	// InvalidateForUser consumes every outstanding token of a user, so issuing a
	// new one silently retires the previous links.
	InvalidateForUser(ctx context.Context, userID string, now time.Time) error
}

// Mailer delivers transactional messages. It is a port so the domain never
// depends on SMTP: development points it at Mailpit and tests substitute a
// recorder.
type Mailer interface {
	Send(ctx context.Context, to string, subject string, body string) error
}

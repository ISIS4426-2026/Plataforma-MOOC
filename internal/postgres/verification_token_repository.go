package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// Table names are package constants rather than a constructor argument: they are
// interpolated into the SQL text, so keeping callers from choosing them removes
// any path by which a table name could come from input.
const (
	tableEmailVerificationTokens = "email_verification_tokens"
	tablePasswordResetTokens     = "password_reset_tokens"
)

// VerificationTokenRepository stores single-use tokens in one of the token
// tables. Email verification and password reset share this implementation but
// never share a table, so a token minted for one flow cannot be replayed in the
// other.
type VerificationTokenRepository struct {
	db    *sql.DB
	table string
}

var _ domain.VerificationTokenRepository = (*VerificationTokenRepository)(nil)

// NewEmailVerificationTokenRepository backs the registration flow of issue #9.
func NewEmailVerificationTokenRepository(db *sql.DB) *VerificationTokenRepository {
	return &VerificationTokenRepository{db: db, table: tableEmailVerificationTokens}
}

// NewPasswordResetTokenRepository backs the recovery flow of issue #11.
func NewPasswordResetTokenRepository(db *sql.DB) *VerificationTokenRepository {
	return &VerificationTokenRepository{db: db, table: tablePasswordResetTokens}
}

const verificationTokenColumns = `id, user_id, token_hash, expires_at, consumed_at, created_at`

func (r *VerificationTokenRepository) Create(ctx context.Context, token *domain.VerificationToken) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`, r.table)

	err := r.db.QueryRowContext(ctx, query,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("create %s: %w", r.table, err)
	}

	return nil
}

// Consume marks the token used and returns it, in a single statement.
//
// The conditions live in the WHERE clause rather than in a preceding SELECT: a
// read-then-write pair lets two concurrent redemptions both see an unused token
// and both proceed. Here the database serialises them, so exactly one UPDATE
// matches a row and the loser gets no row back.
//
// Unknown, already consumed and expired tokens all yield ErrNotFound, so a
// caller cannot probe which case it hit.
func (r *VerificationTokenRepository) Consume(ctx context.Context, tokenHash string, now time.Time) (*domain.VerificationToken, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET consumed_at = $2
		WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING %s`, r.table, verificationTokenColumns)

	var (
		token      domain.VerificationToken
		consumedAt sql.NullTime
	)

	err := r.db.QueryRowContext(ctx, query, tokenHash, now).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&consumedAt,
		&token.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consume %s: %w", r.table, err)
	}

	if consumedAt.Valid {
		at := consumedAt.Time
		token.ConsumedAt = &at
	}

	return &token, nil
}

// InvalidateForUser retires every outstanding token of a user, so requesting a
// new link silently disables the previous ones instead of leaving several valid
// at once.
func (r *VerificationTokenRepository) InvalidateForUser(ctx context.Context, userID string, now time.Time) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET consumed_at = $2
		WHERE user_id = $1 AND consumed_at IS NULL`, r.table)

	if _, err := r.db.ExecContext(ctx, query, userID, now); err != nil {
		return fmt.Errorf("invalidate %s for user %s: %w", r.table, userID, err)
	}

	return nil
}

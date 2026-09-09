package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// SessionRepository is the PostgreSQL implementation of
// domain.SessionRepository, the durable record behind every login.
type SessionRepository struct {
	db *sql.DB
}

var _ domain.SessionRepository = (*SessionRepository)(nil)

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

const sessionColumns = `id, user_id, token_hash, user_agent, ip_address, is_revoked, expires_at, created_at, revoked_at, last_activity_at`

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	const query = `
		INSERT INTO user_sessions (user_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, is_revoked, created_at, last_activity_at`

	err := r.db.QueryRowContext(ctx, query,
		session.UserID,
		session.TokenHash,
		nullString(session.UserAgent),
		nullString(session.IPAddress),
		session.ExpiresAt,
	).Scan(&session.ID, &session.IsRevoked, &session.CreatedAt, &session.LastActivityAt)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// GetByTokenHash returns the session regardless of whether it is still usable.
// Deciding what to do with a revoked or expired session belongs to the caller,
// which needs to tell those cases apart from an unknown token.
func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	query := `SELECT ` + sessionColumns + ` FROM user_sessions WHERE token_hash = $1`

	session, err := scanSession(r.db.QueryRowContext(ctx, query, tokenHash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session by token hash: %w", err)
	}

	return session, nil
}

// ListActiveByUser backs the self-service and administrative session listings.
func (r *SessionRepository) ListActiveByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	query := `SELECT ` + sessionColumns + `
		FROM user_sessions
		WHERE user_id = $1 AND is_revoked = FALSE AND expires_at > NOW()
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	// rows.Err surfaces failures that ended iteration early, which would
	// otherwise be indistinguishable from an empty result.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

// Revoke marks one session revoked.
//
// The WHERE clause requires the session to still be active, so revoking twice
// reports ErrNotFound instead of silently rewriting revoked_at and losing the
// original revocation timestamp from the audit trail.
func (r *SessionRepository) Revoke(ctx context.Context, sessionID string) error {
	const query = `
		UPDATE user_sessions
		SET is_revoked = TRUE, revoked_at = NOW()
		WHERE id = $1 AND is_revoked = FALSE
		RETURNING id`

	var id string
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke session %s: %w", sessionID, err)
	}

	return nil
}

// RevokeAllForUser revokes every active session and reports how many were
// affected, which is what a password change needs in order to log every other
// device out.
func (r *SessionRepository) RevokeAllForUser(ctx context.Context, userID string) (int, error) {
	const query = `
		UPDATE user_sessions
		SET is_revoked = TRUE, revoked_at = NOW()
		WHERE user_id = $1 AND is_revoked = FALSE`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions for user %s: %w", userID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count revoked sessions: %w", err)
	}

	return int(affected), nil
}

// rowScanner covers both *sql.Row and *sql.Rows so single-row and multi-row
// queries share the same scanning code.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner rowScanner) (*domain.Session, error) {
	var (
		session   domain.Session
		userAgent sql.NullString
		ipAddress sql.NullString
		revokedAt sql.NullTime
	)

	err := scanner.Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&userAgent,
		&ipAddress,
		&session.IsRevoked,
		&session.ExpiresAt,
		&session.CreatedAt,
		&revokedAt,
		&session.LastActivityAt,
	)
	if err != nil {
		return nil, err
	}

	session.UserAgent = userAgent.String
	session.IPAddress = ipAddress.String
	if revokedAt.Valid {
		at := revokedAt.Time
		session.RevokedAt = &at
	}

	return &session, nil
}

// nullString maps an empty string to SQL NULL, keeping "not recorded" distinct
// from "recorded as empty" for the nullable audit columns.
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

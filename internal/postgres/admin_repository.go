package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// AdminRepository is the PostgreSQL implementation of domain.AdminRepository.
type AdminRepository struct {
	db *sql.DB
}

var _ domain.AdminRepository = (*AdminRepository)(nil)

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// ListUsers returns one page of accounts, newest first.
//
// Ordering is by (created_at, id) rather than created_at alone: timestamps
// collide, and a cursor built on a non-unique key silently skips or repeats the
// rows that share one.
func (r *AdminRepository) ListUsers(ctx context.Context, filter domain.UserFilter) (*domain.UserPage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = domain.DefaultUserPageSize
	}
	limit = min(limit, domain.MaxUserPageSize)

	conditions := []string{"TRUE"}
	args := []any{}

	if filter.Role != nil {
		args = append(args, string(*filter.Role))
		conditions = append(conditions, fmt.Sprintf("role = $%d", len(args)))
	}
	if filter.Status != nil {
		args = append(args, string(*filter.Status))
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}

	if filter.Cursor != "" {
		createdAt, id, err := decodeUserCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
		args = append(args, createdAt, id)
		// The row tuple comparison is what makes the cursor exact across ties.
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	// One extra row is requested to learn whether another page exists without a
	// second COUNT query over the whole table.
	args = append(args, limit+1)

	query := `SELECT ` + userColumns + ` FROM users WHERE ` +
		strings.Join(conditions, " AND ") +
		` ORDER BY created_at DESC, id DESC LIMIT $` + fmt.Sprint(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	page := &domain.UserPage{Users: users}
	if len(users) > limit {
		page.Users = users[:limit]
		page.HasMore = true
		last := page.Users[len(page.Users)-1]
		page.NextCursor = encodeUserCursor(last.CreatedAt, last.ID)
	}

	return page, nil
}

// ChangeRole assigns a new role and records the audit entry in one transaction.
func (r *AdminRepository) ChangeRole(ctx context.Context, userID string, role domain.Role, entry *domain.AuditEntry) (*domain.User, error) {
	return r.mutate(ctx, userID, entry, func(current *domain.User) (bool, string, []any) {
		// The previous value is recorded here because this is the only place
		// that sees it inside the transaction; read outside, it could already
		// be stale by the time the change lands.
		recordTransition(entry, string(current.Role), string(role))

		// Losing the administrator role removes the user from the active
		// administrator set; keeping it does not.
		removesAdmin := role != domain.RoleAdmin
		return removesAdmin,
			`UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1 RETURNING ` + userColumns,
			[]any{userID, string(role)}
	})
}

// ChangeStatus suspends or reactivates an account and records the audit entry in
// one transaction.
func (r *AdminRepository) ChangeStatus(ctx context.Context, userID string, status domain.UserStatus, entry *domain.AuditEntry) (*domain.User, error) {
	return r.mutate(ctx, userID, entry, func(current *domain.User) (bool, string, []any) {
		recordTransition(entry, string(current.Status), string(status))

		// Any status other than active takes the user out of the active
		// administrator set.
		removesAdmin := status != domain.UserStatusActive
		return removesAdmin,
			`UPDATE users SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING ` + userColumns,
			[]any{userID, string(status)}
	})
}

// mutate runs a single administrative change under the last-administrator rule
// and records its audit entry, all in one transaction.
//
// plan reports whether the change would remove the target from the set of
// active administrators, and returns the statement to run.
func (r *AdminRepository) mutate(
	ctx context.Context,
	userID string,
	entry *domain.AuditEntry,
	plan func(current *domain.User) (removesAdmin bool, query string, args []any),
) (*domain.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback after a successful Commit is a no-op, so this is safe to defer
	// unconditionally and guarantees the transaction is never left open.
	defer func() { _ = tx.Rollback() }()

	// Lock the active administrators before deciding anything.
	//
	// Without this lock two concurrent requests could each read two active
	// administrators, each conclude that removing one is safe, and between them
	// leave the platform with none. Taking the lock first serialises every
	// change that could affect the set, and Postgres re-evaluates the predicate
	// after the lock is granted, so the second transaction sees the first one's
	// result rather than the stale count it started with.
	admins, err := lockActiveAdmins(ctx, tx)
	if err != nil {
		return nil, err
	}

	current, err := lockUser(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	removesAdmin, query, args := plan(current)

	isLastActiveAdmin := current.Role == domain.RoleAdmin &&
		current.Status == domain.UserStatusActive &&
		len(admins) == 1

	if removesAdmin && isLastActiveAdmin {
		return nil, fmt.Errorf("user %s: %w", userID, domain.ErrLastAdminProtected)
	}

	updated, err := scanUser(tx.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("apply administrative change to %s: %w", userID, err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit administrative change: %w", err)
	}

	return updated, nil
}

// recordTransition adds the before and after values to the audit entry without
// discarding whatever the caller already put in Details.
func recordTransition(entry *domain.AuditEntry, from, to string) {
	if entry.Details == nil {
		entry.Details = make(map[string]any, 2)
	}
	entry.Details["from"] = from
	entry.Details["to"] = to
}

// lockActiveAdmins takes a row lock on every active administrator and returns
// their identifiers.
func lockActiveAdmins(ctx context.Context, tx *sql.Tx) ([]string, error) {
	const query = `SELECT id FROM users WHERE role = $1 AND status = $2 FOR UPDATE`

	rows, err := tx.QueryContext(ctx, query, domain.RoleAdmin, domain.UserStatusActive)
	if err != nil {
		return nil, fmt.Errorf("lock active administrators: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan administrator: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate administrators: %w", err)
	}

	return ids, nil
}

func lockUser(ctx context.Context, tx *sql.Tx, userID string) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1 FOR UPDATE`

	user, err := scanUser(tx.QueryRowContext(ctx, query, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load user %s: %w", userID, err)
	}

	return user, nil
}

// encodeUserCursor packs the ordering key into an opaque token.
//
// It is base64 of "<timestamp>|<id>". Opaque so clients treat it as a token
// rather than something to construct by hand, which would tie the API to the
// current ordering.
func encodeUserCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeUserCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("cursor is not valid: %w", domain.ErrInvalidInput)
	}

	createdAt, id, found := strings.Cut(string(decoded), "|")
	if !found {
		return time.Time{}, "", fmt.Errorf("cursor is malformed: %w", domain.ErrInvalidInput)
	}

	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("cursor timestamp is not valid: %w", domain.ErrInvalidInput)
	}

	return parsed, id, nil
}

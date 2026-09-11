package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// querier is the subset shared by *sql.DB and *sql.Tx, so the same insert can
// run standalone or inside a transaction alongside the change it records.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// AuditRepository is the PostgreSQL implementation of domain.AuditRepository.
type AuditRepository struct {
	db *sql.DB
}

var _ domain.AuditRepository = (*AuditRepository)(nil)

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Record appends a standalone entry.
//
// Use it only for actions with nothing to commit alongside them. An action that
// changes state must record its entry in the same transaction as the change,
// which is what the AdminRepository methods do.
func (r *AuditRepository) Record(ctx context.Context, entry *domain.AuditEntry) error {
	return insertAuditEntry(ctx, r.db, entry)
}

const auditColumns = `id, actor_id, action, target_resource, details, ip_address, user_agent, created_at`

// List returns one page of the audit trail, newest first, narrowed by
// filter. It mirrors AdminRepository.ListUsers: cursor pagination on
// (created_at, id) so a tie between two timestamps can't make a caller skip
// or repeat an entry, and one extra row is fetched to learn whether another
// page exists without a second COUNT query.
func (r *AuditRepository) List(ctx context.Context, filter domain.AuditFilter) (*domain.AuditPage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = domain.DefaultAuditPageSize
	}
	limit = min(limit, domain.MaxAuditPageSize)

	conditions := []string{"TRUE"}
	args := []any{}

	if filter.ActorID != "" {
		args = append(args, filter.ActorID)
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", len(args)))
	}
	if filter.ActionPrefix != "" {
		args = append(args, filter.ActionPrefix+"%")
		conditions = append(conditions, fmt.Sprintf("action LIKE $%d", len(args)))
	}
	if filter.TargetResource != "" {
		args = append(args, filter.TargetResource)
		conditions = append(conditions, fmt.Sprintf("target_resource = $%d", len(args)))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	if filter.Cursor != "" {
		createdAt, id, err := decodeAuditCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
		args = append(args, createdAt, id)
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	args = append(args, limit+1)

	query := `SELECT ` + auditColumns + ` FROM audit_logs WHERE ` +
		strings.Join(conditions, " AND ") +
		` ORDER BY created_at DESC, id DESC LIMIT $` + fmt.Sprint(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit entries: %w", err)
	}

	page := &domain.AuditPage{Entries: entries}
	if len(entries) > limit {
		page.Entries = entries[:limit]
		page.HasMore = true
		last := page.Entries[len(page.Entries)-1]
		page.NextCursor = encodeAuditCursor(last.CreatedAt, last.ID)
	}

	return page, nil
}

func scanAuditEntry(scanner rowScanner) (*domain.AuditEntry, error) {
	var (
		entry     domain.AuditEntry
		actorID   sql.NullString
		ipAddress sql.NullString
		userAgent sql.NullString
		details   []byte
	)

	err := scanner.Scan(
		&entry.ID,
		&actorID,
		&entry.Action,
		&entry.TargetResource,
		&details,
		&ipAddress,
		&userAgent,
		&entry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if actorID.Valid {
		entry.ActorID = &actorID.String
	}
	entry.IPAddress = ipAddress.String
	entry.UserAgent = userAgent.String

	if len(details) > 0 {
		if err := json.Unmarshal(details, &entry.Details); err != nil {
			return nil, fmt.Errorf("decode audit details: %w", err)
		}
	}

	return &entry, nil
}

// encodeAuditCursor and decodeAuditCursor mirror the user and course cursor
// helpers: opaque base64 of "<timestamp>|<id>".
func encodeAuditCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeAuditCursor(cursor string) (time.Time, string, error) {
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

func insertAuditEntry(ctx context.Context, q querier, entry *domain.AuditEntry) error {
	const query = `
		INSERT INTO audit_logs (actor_id, action, target_resource, details, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	var details any
	if len(entry.Details) > 0 {
		encoded, err := json.Marshal(entry.Details)
		if err != nil {
			return fmt.Errorf("encode audit details: %w", err)
		}
		details = encoded
	}

	var actorID any
	if entry.ActorID != nil {
		actorID = *entry.ActorID
	}

	err := q.QueryRowContext(ctx, query,
		actorID,
		string(entry.Action),
		nullString(entry.TargetResource),
		details,
		nullString(entry.IPAddress),
		nullString(entry.UserAgent),
	).Scan(&entry.ID, &entry.CreatedAt)

	if err != nil {
		return fmt.Errorf("record audit entry %q: %w", entry.Action, err)
	}

	return nil
}

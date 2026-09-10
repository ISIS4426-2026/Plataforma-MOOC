package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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

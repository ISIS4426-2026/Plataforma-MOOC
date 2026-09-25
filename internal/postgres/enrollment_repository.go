package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// This file backs issue #111. It follows the conventions of the other
// repositories here: the audit entry lands in the same transaction as the write
// it describes, so the trail cannot disagree with the data.
//
// Unlike the structure repositories there is no parent chain to lock. The
// uniqueness of (student_id, course_stable_id) is what serializes two
// concurrent enrollments, and ON CONFLICT is what turns the loser of that race
// into the idempotent answer rather than an error.

const enrollmentColumns = `id, student_id, course_stable_id, status, enrolled_at, withdrawn_at`

type EnrollmentRepository struct{ db *sql.DB }

var _ domain.EnrollmentRepository = (*EnrollmentRepository)(nil)

func NewEnrollmentRepository(db *sql.DB) *EnrollmentRepository {
	return &EnrollmentRepository{db: db}
}

// Enroll creates the enrollment or brings a withdrawn one back.
//
// The upsert does both in one statement, which is what makes it safe under
// concurrency: two simultaneous enrollments cannot produce two rows, and the
// second one reads back the first one's result instead of failing on the unique
// constraint.
func (r *EnrollmentRepository) Enroll(ctx context.Context, studentID, courseStableID string, entry *domain.AuditEntry) (*domain.Enrollment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Whether this is a first enrollment or a return changes the audit action,
	// so the prior state is read before it is overwritten.
	var previous domain.EnrollmentStatus
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM enrollments WHERE student_id = $1 AND course_stable_id = $2 FOR UPDATE`,
		studentID, courseStableID,
	).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read existing enrollment: %w", err)
	}
	existed := !errors.Is(err, sql.ErrNoRows)

	// An already active enrollment is left exactly as it was, timestamps
	// included: re-enrolling is not an event, and moving enrolled_at would
	// rewrite when the student actually joined.
	const query = `
		INSERT INTO enrollments (student_id, course_stable_id, status)
		VALUES ($1, $2, 'active')
		ON CONFLICT (student_id, course_stable_id) DO UPDATE
		SET status       = 'active',
		    enrolled_at  = CASE WHEN enrollments.status = 'withdrawn' THEN NOW() ELSE enrollments.enrolled_at END
		RETURNING ` + enrollmentColumns

	enrollment, err := scanEnrollment(tx.QueryRowContext(ctx, query, studentID, courseStableID))
	if err != nil {
		return nil, fmt.Errorf("enroll student %s: %w", studentID, err)
	}

	// Nothing changed, so nothing is worth recording. Writing an entry here
	// would fill the trail with noise from clients that retry.
	if existed && previous == domain.EnrollmentActive {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit enrollment: %w", err)
		}
		return enrollment, nil
	}

	if entry != nil {
		entry.Action = domain.AuditActionEnrollmentCreated
		if existed {
			entry.Action = domain.AuditActionEnrollmentReactivated
		}
		if err := insertAuditEntry(ctx, tx, entry); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit enrollment: %w", err)
	}
	return enrollment, nil
}

// Withdraw marks the enrollment withdrawn without touching the student's
// progress, which is what makes re-enrolling later preserve it.
func (r *EnrollmentRepository) Withdraw(ctx context.Context, studentID, courseStableID string, entry *domain.AuditEntry) (*domain.Enrollment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `
		UPDATE enrollments
		SET status = 'withdrawn', withdrawn_at = NOW()
		WHERE student_id = $1 AND course_stable_id = $2 AND status = 'active'
		RETURNING ` + enrollmentColumns

	enrollment, err := scanEnrollment(tx.QueryRowContext(ctx, query, studentID, courseStableID))
	if errors.Is(err, sql.ErrNoRows) {
		// Either they never enrolled or they already withdrew. Both mean the
		// caller's goal is already true, so the current state is returned rather
		// than an error -- but only if a row exists at all.
		existing, getErr := r.get(ctx, tx, studentID, courseStableID)
		if getErr != nil {
			return nil, getErr
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit withdrawal: %w", err)
		}
		return existing, nil
	}
	if err != nil {
		return nil, fmt.Errorf("withdraw student %s: %w", studentID, err)
	}

	if entry != nil {
		entry.Action = domain.AuditActionEnrollmentWithdrawn
		if err := insertAuditEntry(ctx, tx, entry); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal: %w", err)
	}
	return enrollment, nil
}

func (r *EnrollmentRepository) Get(ctx context.Context, studentID, courseStableID string) (*domain.Enrollment, error) {
	return r.get(ctx, r.db, studentID, courseStableID)
}

func (r *EnrollmentRepository) get(ctx context.Context, q querier, studentID, courseStableID string) (*domain.Enrollment, error) {
	const query = `SELECT ` + enrollmentColumns + ` FROM enrollments WHERE student_id = $1 AND course_stable_id = $2`
	enrollment, err := scanEnrollment(q.QueryRowContext(ctx, query, studentID, courseStableID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get enrollment: %w", err)
	}
	return enrollment, nil
}

// IsActive answers the access check without materialising the row.
func (r *EnrollmentRepository) IsActive(ctx context.Context, studentID, courseStableID string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM enrollments
			WHERE student_id = $1 AND course_stable_id = $2 AND status = 'active'
		)`
	var active bool
	if err := r.db.QueryRowContext(ctx, query, studentID, courseStableID).Scan(&active); err != nil {
		return false, fmt.Errorf("check enrollment: %w", err)
	}
	return active, nil
}

func (r *EnrollmentRepository) ListByStudent(ctx context.Context, studentID string) ([]*domain.Enrollment, error) {
	const query = `SELECT ` + enrollmentColumns + ` FROM enrollments WHERE student_id = $1 ORDER BY enrolled_at DESC`
	return r.list(ctx, query, studentID)
}

func (r *EnrollmentRepository) ListByCourse(ctx context.Context, courseStableID string) ([]*domain.Enrollment, error) {
	const query = `SELECT ` + enrollmentColumns + ` FROM enrollments WHERE course_stable_id = $1 ORDER BY enrolled_at DESC`
	return r.list(ctx, query, courseStableID)
}

func (r *EnrollmentRepository) list(ctx context.Context, query, arg string) ([]*domain.Enrollment, error) {
	rows, err := r.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*domain.Enrollment
	for rows.Next() {
		enrollment, err := scanEnrollment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan enrollment: %w", err)
		}
		out = append(out, enrollment)
	}
	return out, rows.Err()
}

func scanEnrollment(scanner rowScanner) (*domain.Enrollment, error) {
	var (
		e           domain.Enrollment
		status      string
		withdrawnAt sql.NullTime
	)
	if err := scanner.Scan(&e.ID, &e.StudentID, &e.CourseStableID, &status, &e.EnrolledAt, &withdrawnAt); err != nil {
		return nil, err
	}
	e.Status = domain.EnrollmentStatus(status)
	if withdrawnAt.Valid {
		e.WithdrawnAt = &withdrawnAt.Time
	}
	return &e, nil
}

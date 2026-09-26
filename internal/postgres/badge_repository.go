package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// This file backs the badge half of issue #113, and it only reads.
//
// Issuing a badge lives in progress_repository.go, inside the transaction that
// marks the student approved, because "approved implies a badge" is an invariant
// and two transactions cannot hold it. What belongs here is the other side: the
// public verification of a badge someone was handed.

type BadgeRepository struct{ db *sql.DB }

var _ domain.BadgeRepository = (*BadgeRepository)(nil)

func NewBadgeRepository(db *sql.DB) *BadgeRepository {
	return &BadgeRepository{db: db}
}

// Verify resolves a verification code to what the badge certifies.
//
// The course title comes from the course's latest version. A badge is keyed by
// stable id, so it survives new versions, and the title a verifier should see is
// the course as it is called now -- not as it was called when the badge was
// issued.
func (r *BadgeRepository) Verify(ctx context.Context, code string) (*domain.BadgeCredential, error) {
	// users is deliberately not joined. Nothing about the student leaves this
	// query, so there is nothing here for a leak to escape through.
	const query = `
		SELECT (SELECT c.title FROM courses c
		         WHERE c.stable_id = b.course_stable_id
		         ORDER BY c.version DESC
		         LIMIT 1) AS course_title,
		       b.issued_at,
		       b.is_revoked
		FROM badges b
		WHERE b.verification_code = $1`

	var (
		credential domain.BadgeCredential
		title      sql.NullString
	)
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&title, &credential.IssuedAt, &credential.IsRevoked,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify badge: %w", err)
	}
	// A badge whose course rows were all deleted still verifies: the student did
	// the work. Naming no course is more honest than refusing to confirm it.
	credential.CourseTitle = title.String
	return &credential, nil
}

// GetByStudentAndCourse returns the student's badge for a course.
func (r *BadgeRepository) GetByStudentAndCourse(ctx context.Context, studentID, courseStableID string) (*domain.Badge, error) {
	const query = `
		SELECT id, student_id, course_stable_id, verification_code, image_key, is_revoked, issued_at
		FROM badges
		WHERE student_id = $1 AND course_stable_id = $2`

	badge := &domain.Badge{}
	err := r.db.QueryRowContext(ctx, query, studentID, courseStableID).Scan(
		&badge.ID, &badge.StudentID, &badge.CourseStableID, &badge.VerificationCode,
		&badge.ImageKey, &badge.IsRevoked, &badge.IssuedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get badge: %w", err)
	}
	return badge, nil
}

// GetByID returns one badge by its own id.
//
// Who may see it is not decided here. The service asks that question, because the
// answer depends on the caller, and a repository that refused rows would have to
// know about actors.
func (r *BadgeRepository) GetByID(ctx context.Context, badgeID string) (*domain.Badge, error) {
	const query = `
		SELECT id, student_id, course_stable_id, verification_code, image_key, is_revoked, issued_at
		FROM badges
		WHERE id = $1`

	badge := &domain.Badge{}
	err := r.db.QueryRowContext(ctx, query, badgeID).Scan(
		&badge.ID, &badge.StudentID, &badge.CourseStableID, &badge.VerificationCode,
		&badge.ImageKey, &badge.IsRevoked, &badge.IssuedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get badge %s: %w", badgeID, err)
	}
	return badge, nil
}

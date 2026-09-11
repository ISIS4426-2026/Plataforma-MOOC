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

// CourseRepository is the PostgreSQL implementation of domain.CourseRepository.
type CourseRepository struct {
	db *sql.DB
}

var _ domain.CourseRepository = (*CourseRepository)(nil)

func NewCourseRepository(db *sql.DB) *CourseRepository {
	return &CourseRepository{db: db}
}

const courseColumns = `id, stable_id, title, description, version, status, author_id, created_at, updated_at`

// Create inserts a new course draft and records its audit entry in the same
// transaction: "every authoring action is reflected in the audit trail"
// (issue #18) only holds if the two can never exist without each other.
func (r *CourseRepository) Create(ctx context.Context, course *domain.Course, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `
		INSERT INTO courses (id, stable_id, title, description, version, status, author_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`

	err = tx.QueryRowContext(ctx, query,
		course.ID,
		course.StableID,
		course.Title,
		course.Description,
		course.Version,
		course.Status,
		course.AuthorID,
	).Scan(&course.CreatedAt, &course.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create course: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit course creation: %w", err)
	}

	return nil
}

// GetByID returns a single course row — one specific version.
func (r *CourseRepository) GetByID(ctx context.Context, id string) (*domain.Course, error) {
	query := `SELECT ` + courseColumns + ` FROM courses WHERE id = $1`

	course, err := scanCourse(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get course %s: %w", id, err)
	}
	return course, nil
}

// GetByStableIDAndVersion returns one version of a course by its stable
// identity.
func (r *CourseRepository) GetByStableIDAndVersion(ctx context.Context, stableID string, version int) (*domain.Course, error) {
	query := `SELECT ` + courseColumns + ` FROM courses WHERE stable_id = $1 AND version = $2`

	course, err := scanCourse(r.db.QueryRowContext(ctx, query, stableID, version))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get course %s version %d: %w", stableID, version, err)
	}
	return course, nil
}

// ListPublished mirrors AdminRepository.ListUsers: cursor pagination on
// (created_at, id) so a tie between two timestamps can't make the client
// skip or repeat a row, and one extra row is fetched to learn whether
// another page exists without a second COUNT query.
func (r *CourseRepository) ListPublished(ctx context.Context, filter domain.CourseFilter) (*domain.CoursePage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = domain.DefaultCoursePageSize
	}
	limit = min(limit, domain.MaxCoursePageSize)

	conditions := []string{"status = $1"}
	args := []any{string(domain.CourseStatusPublished)}

	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", len(args), len(args)))
	}

	if filter.Cursor != "" {
		createdAt, id, err := decodeCourseCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
		args = append(args, createdAt, id)
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	args = append(args, limit+1)

	query := `SELECT ` + courseColumns + ` FROM courses WHERE ` +
		strings.Join(conditions, " AND ") +
		` ORDER BY created_at DESC, id DESC LIMIT $` + fmt.Sprint(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list published courses: %w", err)
	}
	defer rows.Close()

	var courses []*domain.Course
	for rows.Next() {
		course, err := scanCourse(rows)
		if err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}
		courses = append(courses, course)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate courses: %w", err)
	}

	page := &domain.CoursePage{Courses: courses}
	if len(courses) > limit {
		page.Courses = courses[:limit]
		page.HasMore = true
		last := page.Courses[len(page.Courses)-1]
		page.NextCursor = encodeCourseCursor(last.CreatedAt, last.ID)
	}

	return page, nil
}

// Update changes a draft's title and description inside a transaction that
// also records the audit entry.
//
// The published check and the ETag precondition are both verified here,
// holding the row lock, rather than by the caller before the call: checking
// outside leaves a window in which another writer's change (or publication)
// lands between the check and the update, and this write would silently
// clobber or resurrect a version the caller never saw.
func (r *CourseRepository) Update(ctx context.Context, courseID string, title, description string, opts domain.ChangeOptions, entry *domain.AuditEntry) (*domain.Course, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := lockCourse(ctx, tx, courseID)
	if err != nil {
		return nil, err
	}

	if opts.ExpectedETag != "" && opts.ExpectedETag != current.ETag() {
		return nil, fmt.Errorf("course %s: %w", courseID, domain.ErrPreconditionFailed)
	}

	if current.Status == domain.CourseStatusPublished {
		return nil, fmt.Errorf("course %s: %w", courseID, domain.ErrCourseImmutable)
	}

	const query = `
		UPDATE courses SET title = $2, description = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + courseColumns

	updated, err := scanCourse(tx.QueryRowContext(ctx, query, courseID, title, description))
	if err != nil {
		return nil, fmt.Errorf("update course %s: %w", courseID, err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit course update: %w", err)
	}

	return updated, nil
}

// UpdateStatus transitions a course's status (issue #20: publish and
// unpublish) inside a transaction that also records the audit entry. It does
// not itself validate the transition or lock+recheck any precondition beyond
// the row's existence -- course.Service decides whether a transition is
// allowed before calling this.
func (r *CourseRepository) UpdateStatus(ctx context.Context, courseID string, status domain.CourseStatus, entry *domain.AuditEntry) (*domain.Course, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := lockCourse(ctx, tx, courseID); err != nil {
		return nil, err
	}

	const query = `
		UPDATE courses SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + courseColumns

	updated, err := scanCourse(tx.QueryRowContext(ctx, query, courseID, string(status)))
	if err != nil {
		return nil, fmt.Errorf("update status of course %s: %w", courseID, err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit course status change: %w", err)
	}

	return updated, nil
}

// lockCourse takes a row lock on the course being edited, so a concurrent
// update or publish cannot interleave with the precondition and immutability
// checks that follow it.
func lockCourse(ctx context.Context, tx *sql.Tx, courseID string) (*domain.Course, error) {
	query := `SELECT ` + courseColumns + ` FROM courses WHERE id = $1 FOR UPDATE`

	course, err := scanCourse(tx.QueryRowContext(ctx, query, courseID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load course %s: %w", courseID, err)
	}
	return course, nil
}

func scanCourse(scanner rowScanner) (*domain.Course, error) {
	var course domain.Course
	err := scanner.Scan(
		&course.ID,
		&course.StableID,
		&course.Title,
		&course.Description,
		&course.Version,
		&course.Status,
		&course.AuthorID,
		&course.CreatedAt,
		&course.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &course, nil
}

// encodeCourseCursor and decodeCourseCursor mirror the user cursor helpers in
// admin_repository.go: opaque base64 of "<timestamp>|<id>", so a client
// treats the cursor as a token rather than something to construct by hand.
func encodeCourseCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCourseCursor(cursor string) (time.Time, string, error) {
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

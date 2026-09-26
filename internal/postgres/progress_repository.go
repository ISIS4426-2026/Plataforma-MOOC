package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/lib/pq"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// This file backs issue #113.
//
// Two tables with two different jobs. progress_events is an append-only log:
// every heartbeat is a fact that happened, written once and never revised.
// student_progress is the projection clients read, and it is *derived* -- every
// heartbeat recomputes it from scratch rather than incrementing a counter.
//
// That is what makes repeated heartbeats harmless. An increment would count the
// same resource twice; recomputing a set and intersecting it with the course's
// current resources cannot, however many times the same report arrives.

type ProgressRepository struct{ db *sql.DB }

var _ domain.ProgressRepository = (*ProgressRepository)(nil)

func NewProgressRepository(db *sql.DB) *ProgressRepository {
	return &ProgressRepository{db: db}
}

// Locate resolves a resource row id up through its unit and module to the
// course version it belongs to.
//
// The caller needs this before recording anything: the heartbeat names only a
// resource, and whether the student may report progress on it depends on the
// course it sits in.
func (r *ProgressRepository) Locate(ctx context.Context, resourceID string) (*domain.ResourceLocation, error) {
	const query = `
		SELECT r.stable_id, c.id, c.stable_id, c.status, c.author_id
		FROM resources r
		JOIN units u   ON u.id = r.unit_id
		JOIN modules m ON m.id = u.module_id
		JOIN courses c ON c.id = m.course_id
		WHERE r.id = $1`

	var (
		at     domain.ResourceLocation
		status string
	)
	err := r.db.QueryRowContext(ctx, query, resourceID).Scan(
		&at.ResourceStableID, &at.CourseID, &at.CourseStableID, &status, &at.CourseAuthorID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("locate resource %s: %w", resourceID, err)
	}
	at.CourseStatus = domain.CourseStatus(status)
	return &at, nil
}

// RecordHeartbeat appends the event and recomputes the projection in one
// transaction, issuing the course's badge when this is the heartbeat that
// completes it.
//
// The badge is written here, in the same transaction, rather than by a caller
// afterwards. "A student marked approved has a badge" is an invariant, and a
// crash between two transactions is exactly what would break it.
func (r *ProgressRepository) RecordHeartbeat(
	ctx context.Context, hb domain.Heartbeat, at *domain.ResourceLocation, entry *domain.AuditEntry,
) (*domain.StudentProgress, *domain.Badge, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// The event goes in first and unconditionally. It is the raw record, and it
	// is worth keeping even for a heartbeat that changes the projection not at
	// all: "the student was on this resource for 30 more seconds" is the fact
	// the capacity analysis of scenario 1 is built on.
	const insertEvent = `
		INSERT INTO progress_events (student_id, course_stable_id, resource_stable_id, dwell_time_seconds, completed)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, insertEvent,
		hb.StudentID, at.CourseStableID, at.ResourceStableID, hb.DwellTimeSeconds, hb.Completed,
	); err != nil {
		return nil, nil, fmt.Errorf("record progress event: %w", err)
	}

	// Make sure the projection row exists, then lock it. Creating and locking in
	// two steps is what lets concurrent heartbeats from the same student
	// serialize on the row instead of racing to compute it: whoever gets the
	// lock second sees the first one's committed set, not a stale snapshot.
	const ensureRow = `
		INSERT INTO student_progress (student_id, course_stable_id)
		VALUES ($1, $2)
		ON CONFLICT (student_id, course_stable_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensureRow, hb.StudentID, at.CourseStableID); err != nil {
		return nil, nil, fmt.Errorf("ensure progress row: %w", err)
	}

	const lockRow = `
		SELECT completed_resources, is_approved
		FROM student_progress
		WHERE student_id = $1 AND course_stable_id = $2
		FOR UPDATE`
	var (
		rawSet      []byte
		wasApproved bool
	)
	if err := tx.QueryRowContext(ctx, lockRow, hb.StudentID, at.CourseStableID).Scan(&rawSet, &wasApproved); err != nil {
		return nil, nil, fmt.Errorf("lock progress row: %w", err)
	}
	completed, err := decodeStableIDs(rawSet)
	if err != nil {
		return nil, nil, err
	}
	if hb.Completed {
		completed = addStableID(completed, at.ResourceStableID)
	}

	counts, err := countCourseResources(ctx, tx, at.CourseID, completed)
	if err != nil {
		return nil, nil, err
	}

	encodedSet, err := json.Marshal(completed)
	if err != nil {
		return nil, nil, fmt.Errorf("encode completed resources: %w", err)
	}

	const updateRow = `
		UPDATE student_progress
		SET completed_resources = $3::jsonb,
		    percent_completed   = $4,
		    is_approved         = $5,
		    updated_at          = NOW()
		WHERE student_id = $1 AND course_stable_id = $2
		RETURNING updated_at`

	progress := &domain.StudentProgress{
		StudentID:          hb.StudentID,
		CourseStableID:     at.CourseStableID,
		CompletedResources: completed,
		CompletedCount:     counts.done,
		TotalCount:         counts.total,
		PercentCompleted:   counts.percent(),
		IsApproved:         counts.approved(),
	}
	if err := tx.QueryRowContext(ctx, updateRow,
		hb.StudentID, at.CourseStableID, encodedSet, progress.PercentCompleted, progress.IsApproved,
	).Scan(&progress.UpdatedAt); err != nil {
		return nil, nil, fmt.Errorf("update progress: %w", err)
	}

	// Completing the course is a transition, so it is recorded once: on the
	// heartbeat that crosses the line, not on every one that follows it.
	if progress.IsApproved && !wasApproved && entry != nil {
		completion := *entry
		completion.Action = domain.AuditActionCourseCompleted
		if err := insertAuditEntry(ctx, tx, &completion); err != nil {
			return nil, nil, err
		}
	}

	var badge *domain.Badge
	if progress.IsApproved {
		// Attempted on every approved heartbeat, not only the first. The insert
		// is a no-op once the badge exists, and trying again is what makes the
		// invariant self-healing: a student who somehow ended up approved
		// without a badge gets one on their next heartbeat.
		badge, err = issueBadge(ctx, tx, hb.StudentID, at.CourseStableID)
		if err != nil {
			return nil, nil, err
		}
		if badge != nil && entry != nil {
			issued := *entry
			issued.Action = domain.AuditActionBadgeIssued
			issued.TargetResource = "badge:" + badge.ID
			if err := insertAuditEntry(ctx, tx, &issued); err != nil {
				return nil, nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit progress: %w", err)
	}
	return progress, badge, nil
}

// GetStudentProgress reads the projection, recomputing the counts against the
// course's resources as they are now.
//
// The stored percentage is not trusted, for the same reason the counts are
// recomputed on write: the course may have gained or lost resources since the
// last heartbeat, and a number that was right last week is simply wrong.
func (r *ProgressRepository) GetStudentProgress(ctx context.Context, studentID, courseID string) (*domain.StudentProgress, error) {
	var courseStableID string
	err := r.db.QueryRowContext(ctx, `SELECT stable_id FROM courses WHERE id = $1`, courseID).Scan(&courseStableID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve course %s: %w", courseID, err)
	}

	const query = `
		SELECT completed_resources, is_approved, updated_at
		FROM student_progress
		WHERE student_id = $1 AND course_stable_id = $2`

	progress := &domain.StudentProgress{StudentID: studentID, CourseStableID: courseStableID}
	var rawSet []byte
	err = r.db.QueryRowContext(ctx, query, studentID, courseStableID).Scan(&rawSet, &progress.IsApproved, &progress.UpdatedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Never reported anything. That is a progress of zero, not a missing
		// resource: the question "how far am I" has an answer before the first
		// heartbeat, and a 404 would force every client to special-case it.
		progress.CompletedResources = []string{}
	case err != nil:
		return nil, fmt.Errorf("get progress: %w", err)
	default:
		if progress.CompletedResources, err = decodeStableIDs(rawSet); err != nil {
			return nil, err
		}
	}

	counts, err := countCourseResources(ctx, r.db, courseID, progress.CompletedResources)
	if err != nil {
		return nil, err
	}
	progress.CompletedCount = counts.done
	progress.TotalCount = counts.total
	progress.PercentCompleted = counts.percent()
	return progress, nil
}

// ---- shared -----------------------------------------------------------

// resourceCounts is the numerator and denominator of a percentage.
type resourceCounts struct{ done, total int }

func (c resourceCounts) percent() float64 {
	if c.total == 0 {
		return 0
	}
	// Two decimals, matching NUMERIC(5,2) in the schema, so what a client reads
	// back is what was stored.
	return math.Round(float64(c.done)/float64(c.total)*10000) / 100
}

// approved is completion: every mandatory resource done.
//
// An empty course is not completed. Zero of zero is arguably everything, but a
// badge certifying a course with no content certifies nothing, so the
// denominator has to be real for the answer to be yes.
//
// Quizzes (issue #112) are not part of this condition yet. When grading lands,
// approval gains a second term; until then the only thing a student can do in a
// course is work through its resources.
func (c resourceCounts) approved() bool {
	return c.total > 0 && c.done >= c.total
}

// countCourseResources counts a course version's mandatory, visible resources
// and how many of them the given set covers.
//
// The intersection is the point. The set is keyed by stable ids that outlive
// course versions, so it can name a resource that was since deleted or that
// belongs to a version this one replaced. Counting matches rather than the set's
// own length is what keeps such a leftover from inflating the percentage -- and
// from carrying a student to 100% on a course they never finished.
func countCourseResources(ctx context.Context, q querier, courseID string, completed []string) (resourceCounts, error) {
	const query = `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE r.stable_id::text = ANY($2::text[])) AS done
		FROM resources r
		JOIN units u   ON u.id = r.unit_id
		JOIN modules m ON m.id = u.module_id
		WHERE m.course_id = $1 AND r.is_mandatory AND r.is_visible`

	var counts resourceCounts
	if err := q.QueryRowContext(ctx, query, courseID, pq.Array(completed)).Scan(&counts.total, &counts.done); err != nil {
		return resourceCounts{}, fmt.Errorf("count course resources: %w", err)
	}
	return counts, nil
}

// issueBadge creates the student's badge for a course, or returns nil when they
// already have one.
//
// UNIQUE (student_id, course_stable_id) plus ON CONFLICT DO NOTHING is what
// makes this safe to call on every completed heartbeat: at most one badge is
// ever inserted, and only the call that inserts it gets a row back.
func issueBadge(ctx context.Context, q querier, studentID, courseStableID string) (*domain.Badge, error) {
	const query = `
		INSERT INTO badges (student_id, course_stable_id, image_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (student_id, course_stable_id) DO NOTHING
		RETURNING id, student_id, course_stable_id, verification_code, image_key, is_revoked, issued_at`

	badge := &domain.Badge{}
	err := q.QueryRowContext(ctx, query, studentID, courseStableID, domain.BadgeImageKey(studentID, courseStableID)).Scan(
		&badge.ID, &badge.StudentID, &badge.CourseStableID, &badge.VerificationCode,
		&badge.ImageKey, &badge.IsRevoked, &badge.IssuedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("issue badge: %w", err)
	}
	return badge, nil
}

// decodeStableIDs reads the JSONB set. A NULL or empty column is an empty set,
// not a failure.
func decodeStableIDs(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode completed resources: %w", err)
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// addStableID adds an id to the set, leaving it untouched when already present.
// This is the "repeated heartbeats do not inflate the percentage" criterion, at
// its smallest.
func addStableID(set []string, id string) []string {
	for _, existing := range set {
		if existing == id {
			return set
		}
	}
	return append(set, id)
}

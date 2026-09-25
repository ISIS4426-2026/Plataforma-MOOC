package domain

import (
	"context"
	"time"
)

// EnrollmentStatus is where a student stands with a course. The two values
// match the CHECK constraint in migrations/000006_enrollments.up.sql.
//
// There is no third value for "finished": completion lives in
// student_progress.is_approved. An enrollment answers whether the student may
// reach the content, which stays true after they finish it.
type EnrollmentStatus string

const (
	EnrollmentActive    EnrollmentStatus = "active"
	EnrollmentWithdrawn EnrollmentStatus = "withdrawn"
)

// Enrollment ties a student to a course.
//
// It keys on the course's stable id, not its row id, the same way
// student_progress and badges do. That is what lets a student keep their
// progress across a new published version of the course, which section 5.1 of
// the spec asks for.
type Enrollment struct {
	ID             string           `json:"id"`
	StudentID      string           `json:"student_id"`
	CourseStableID string           `json:"course_stable_id"`
	Status         EnrollmentStatus `json:"status"`
	EnrolledAt     time.Time        `json:"enrolled_at"`

	// WithdrawnAt is set only while the enrollment is withdrawn, and cleared
	// when the student returns: an active enrollment carrying a withdrawal date
	// is a row that contradicts itself, and a client would have to know which
	// field wins. The history of comings and goings lives in audit_logs, which
	// is the append-only trail built for it.
	WithdrawnAt *time.Time `json:"withdrawn_at,omitempty"`
}

// Active reports whether this enrollment currently grants access.
func (e *Enrollment) Active() bool {
	return e != nil && e.Status == EnrollmentActive
}

type EnrollmentRepository interface {
	// Enroll makes the student's enrollment active, creating it if this is the
	// first time and reactivating it if they had withdrawn.
	//
	// It is idempotent: enrolling twice returns the same enrollment rather than
	// creating a second one or failing, because the caller asked to be enrolled
	// and they are.
	Enroll(ctx context.Context, studentID, courseStableID string, entry *AuditEntry) (*Enrollment, error)

	// Withdraw marks the enrollment withdrawn. The row stays, and so does the
	// student's progress.
	Withdraw(ctx context.Context, studentID, courseStableID string, entry *AuditEntry) (*Enrollment, error)

	// Get returns the enrollment, or ErrNotFound when the student never enrolled.
	Get(ctx context.Context, studentID, courseStableID string) (*Enrollment, error)

	// IsActive is the access check, asked on every read of course content. It is
	// separate from Get so the common case is one indexed existence check rather
	// than loading a row to look at one field.
	IsActive(ctx context.Context, studentID, courseStableID string) (bool, error)

	// ListByStudent returns the student's enrollments, most recent first.
	ListByStudent(ctx context.Context, studentID string) ([]*Enrollment, error)

	// ListByCourse returns a course's roster.
	ListByCourse(ctx context.Context, courseStableID string) ([]*Enrollment, error)
}

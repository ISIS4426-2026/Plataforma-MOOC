// Package enrollment implements who may take a course: enrolling, withdrawing,
// re-enrolling, and the access check the rest of the platform asks before
// serving course content.
//
// Section 5.1 of the spec asks for "inscripcion, retiro y reinscripcion
// conservando progreso y resultados", and that last clause is what shapes the
// package: withdrawing is a status change, never a delete, so the student's
// progress survives it.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// CourseLookup is the slice of the course repository this service reads: it
// needs a course's status and author, nothing else.
type CourseLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Course, error)
}

// Deps are the ports the service depends on.
type Deps struct {
	Courses     CourseLookup
	Enrollments domain.EnrollmentRepository
}

// Actor is who is performing the action, matching the shape the other services
// use.
type Actor struct {
	ID        string
	Role      domain.Role
	IPAddress string
	UserAgent string
}

type Service struct {
	deps   Deps
	logger *slog.Logger
}

func NewService(deps Deps, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{deps: deps, logger: logger}
}

// Enroll signs the actor up for a course.
//
// Enrolling twice is not an error: the caller asked to be enrolled and they
// are, so the existing enrollment comes back untouched. That is what keeps a
// retried request -- or a double-clicked button -- from being a failure the
// student has to interpret.
func (s *Service) Enroll(ctx context.Context, actor Actor, courseID string) (*domain.Enrollment, error) {
	course, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	// Only a published course accepts enrollments. A draft is unfinished and an
	// unpublished one was deliberately withdrawn from the catalog; letting a
	// student in either way would give them content the author has not released.
	if course.Status != domain.CourseStatusPublished {
		return nil, fmt.Errorf("course %s is %s, not published: %w", courseID, course.Status, domain.ErrConflict)
	}

	// The author reading their own course does not need an enrollment, and
	// giving them one would put them on their own roster.
	if course.AuthorID == actor.ID {
		return nil, fmt.Errorf("an author cannot enrol in their own course: %w", domain.ErrForbidden)
	}

	enrollment, err := s.deps.Enrollments.Enroll(ctx, actor.ID, course.StableID, newEntry(actor, "course:"+course.StableID))
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "student enrolled",
		slog.String("student_id", actor.ID),
		slog.String("course_stable_id", course.StableID),
		slog.String("status", string(enrollment.Status)),
	)
	return enrollment, nil
}

// Withdraw takes the actor off a course without discarding what they did in it.
func (s *Service) Withdraw(ctx context.Context, actor Actor, courseID string) (*domain.Enrollment, error) {
	course, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	// Withdrawing is allowed whatever the course's status. A student enrolled in
	// a course that was later unpublished must still be able to leave it.
	enrollment, err := s.deps.Enrollments.Withdraw(ctx, actor.ID, course.StableID, newEntry(actor, "course:"+course.StableID))
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "student withdrew",
		slog.String("student_id", actor.ID),
		slog.String("course_stable_id", course.StableID),
	)
	return enrollment, nil
}

// Mine lists the actor's own enrollments.
func (s *Service) Mine(ctx context.Context, actor Actor) ([]*domain.Enrollment, error) {
	return s.deps.Enrollments.ListByStudent(ctx, actor.ID)
}

// Roster lists a course's enrollments. Only the author and administrators may
// read it: it is a list of who is taking the course, which is nobody else's
// business.
func (s *Service) Roster(ctx context.Context, actor Actor, courseID string) ([]*domain.Enrollment, error) {
	course, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if actor.Role != domain.RoleAdmin && course.AuthorID != actor.ID {
		return nil, domain.ErrForbidden
	}
	return s.deps.Enrollments.ListByCourse(ctx, course.StableID)
}

// Status returns the actor's enrollment in a course, or nil when they never
// enrolled. A missing enrollment is an answer, not a failure, so callers do not
// have to treat "not enrolled" as an error path.
func (s *Service) Status(ctx context.Context, actor Actor, courseID string) (*domain.Enrollment, error) {
	course, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	enrollment, err := s.deps.Enrollments.Get(ctx, actor.ID, course.StableID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return enrollment, err
}

// CanRead answers whether viewerID may read the content of a course.
//
// This is the access check the structure service asks before listing modules,
// units or resources, and it is deliberately the only place the rule lives:
//
//   - an administrator may read anything;
//   - the author may read their own course at any status, which is what makes
//     previewing a draft work;
//   - anyone else needs an active enrollment, and the course must be published.
//
// An anonymous viewer -- empty id -- never passes, which is what turns the
// catalog into the only thing a visitor can see.
func (s *Service) CanRead(ctx context.Context, viewerID string, role domain.Role, course *domain.Course) (bool, error) {
	if course == nil {
		return false, domain.ErrNotFound
	}
	if role == domain.RoleAdmin {
		return true, nil
	}
	if viewerID != "" && course.AuthorID == viewerID {
		return true, nil
	}
	if course.Status != domain.CourseStatusPublished {
		return false, nil
	}
	if viewerID == "" {
		return false, nil
	}
	return s.deps.Enrollments.IsActive(ctx, viewerID, course.StableID)
}

// newEntry builds the audit record. The action is left to the repository, which
// is the only layer that knows whether an enrollment was created or reactivated.
func newEntry(actor Actor, target string) *domain.AuditEntry {
	actorID := actor.ID
	return &domain.AuditEntry{
		ActorID:        &actorID,
		TargetResource: target,
		IPAddress:      actor.IPAddress,
		UserAgent:      actor.UserAgent,
	}
}

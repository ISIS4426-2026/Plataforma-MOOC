// Package progress implements how far a student has got in a course: the
// heartbeats they report, the percentage derived from them, and the badge that
// completing a course earns.
//
// Section 5.1 of the spec asks for progress that survives re-enrollment, which
// is why everything here is keyed by a course's stable id rather than by the row
// id of one of its versions.
package progress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// EnrollmentCheck is the slice of enrollment this service reads: whether the
// student may currently work through a course. Declared here so the dependency
// points inwards -- the rule belongs to enrollment, the requirement belongs
// here.
type EnrollmentCheck interface {
	IsActive(ctx context.Context, studentID, courseStableID string) (bool, error)
}

// Deps are the ports the service depends on.
type Deps struct {
	Progress domain.ProgressRepository
	Badges   domain.BadgeRepository

	// Enrollments gates recording. A nil value refuses every heartbeat rather
	// than accepting one: a service wired without its check should fail visibly,
	// not credit progress to anyone who asks.
	Enrollments EnrollmentCheck
}

// Actor is who is reporting or reading, matching the shape the other services
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

// Report records one heartbeat and returns the recomputed progress, along with
// the course's badge once the student has earned it.
//
// Recording requires an active enrollment, and that is a deliberate exclusion of
// the author and the administrator: they can read the course without enrolling,
// but crediting them with progress would put a badge for the course on its own
// author.
func (s *Service) Report(ctx context.Context, actor Actor, hb domain.Heartbeat) (*domain.StudentProgress, *domain.Badge, error) {
	if hb.DwellTimeSeconds < 0 || hb.DwellTimeSeconds > domain.MaxHeartbeatDwellSeconds {
		return nil, nil, fmt.Errorf("dwell time must be between 0 and %d seconds: %w",
			domain.MaxHeartbeatDwellSeconds, domain.ErrInvalidInput)
	}
	if hb.ResourceID == "" {
		return nil, nil, fmt.Errorf("a heartbeat must name a resource: %w", domain.ErrInvalidInput)
	}
	hb.StudentID = actor.ID

	at, err := s.deps.Progress.Locate(ctx, hb.ResourceID)
	if err != nil {
		return nil, nil, err
	}

	// Progress only accrues in a course that is actually on offer. A draft has
	// not been released and an unpublished one was withdrawn from the catalog;
	// crediting work in either would record progress against content no student
	// is supposed to be working through.
	if at.CourseStatus != domain.CourseStatusPublished {
		return nil, nil, fmt.Errorf("course %s is %s, not published: %w",
			at.CourseStableID, at.CourseStatus, domain.ErrConflict)
	}

	if s.deps.Enrollments == nil {
		return nil, nil, fmt.Errorf("progress service has no enrollment check wired: %w", domain.ErrForbidden)
	}
	active, err := s.deps.Enrollments.IsActive(ctx, actor.ID, at.CourseStableID)
	if err != nil {
		return nil, nil, err
	}
	if !active {
		return nil, nil, fmt.Errorf("student %s has no active enrollment in course %s: %w",
			actor.ID, at.CourseStableID, domain.ErrForbidden)
	}

	updated, badge, err := s.deps.Progress.RecordHeartbeat(ctx, hb, at, newEntry(actor, "course:"+at.CourseStableID))
	if err != nil {
		return nil, nil, err
	}

	if badge != nil {
		s.logger.InfoContext(ctx, "course completed and badge issued",
			slog.String("student_id", actor.ID),
			slog.String("course_stable_id", at.CourseStableID),
			slog.String("badge_id", badge.ID),
		)
	}

	// The badge comes back from the repository only on the call that created it.
	// Looking it up otherwise is what lets the response be the same shape on
	// every completed heartbeat, so a client that retried does not have to infer
	// its credential from a missing field.
	if badge == nil && updated.IsApproved {
		badge, err = s.badgeFor(ctx, actor.ID, at.CourseStableID)
		if err != nil {
			return nil, nil, err
		}
	}
	return updated, badge, nil
}

// Course returns the actor's own progress in a course, and their badge for it
// when they have one.
//
// No enrollment is required to read it. It is the caller's own record, so it
// reveals nothing they are not entitled to, and a student who withdrew has every
// reason to look at what they had done.
func (s *Service) Course(ctx context.Context, actor Actor, courseID string) (*domain.StudentProgress, *domain.Badge, error) {
	current, err := s.deps.Progress.GetStudentProgress(ctx, actor.ID, courseID)
	if err != nil {
		return nil, nil, err
	}
	badge, err := s.badgeFor(ctx, actor.ID, current.CourseStableID)
	if err != nil {
		return nil, nil, err
	}
	return current, badge, nil
}

// Badge returns one of the caller's own badges.
//
// A badge that is not theirs answers ErrNotFound, not ErrForbidden, and the
// difference matters: this response carries the verification code, which is the
// shareable half of the credential. Answering "forbidden" would confirm that the
// id names a real badge, and anyone who learned an id could then tell a verifier
// that the badge behind it is theirs. Not finding it says nothing either way.
//
// Administrators are the exception. They already read the audit trail, where the
// issuance of every badge is recorded, so withholding the badge itself would
// protect nothing.
func (s *Service) Badge(ctx context.Context, actor Actor, badgeID string) (*domain.Badge, error) {
	if badgeID == "" {
		return nil, fmt.Errorf("a badge id is required: %w", domain.ErrInvalidInput)
	}
	badge, err := s.deps.Badges.GetByID(ctx, badgeID)
	if err != nil {
		return nil, err
	}
	if badge.StudentID != actor.ID && actor.Role != domain.RoleAdmin {
		return nil, domain.ErrNotFound
	}
	return badge, nil
}

// Verify resolves a badge's public code. It takes no actor: the code is the
// credential, and requiring an account to check someone else's badge would
// defeat the purpose of issuing a verifiable one.
func (s *Service) Verify(ctx context.Context, code string) (*domain.BadgeCredential, error) {
	if code == "" {
		return nil, fmt.Errorf("a verification code is required: %w", domain.ErrInvalidInput)
	}
	return s.deps.Badges.Verify(ctx, code)
}

// badgeFor returns the student's badge for a course, or nil when they have none.
// Not having earned a badge is an answer, not a failure.
func (s *Service) badgeFor(ctx context.Context, studentID, courseStableID string) (*domain.Badge, error) {
	badge, err := s.deps.Badges.GetByStudentAndCourse(ctx, studentID, courseStableID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return badge, nil
}

// newEntry builds the audit record. The action is left to the repository, which
// is the only layer that knows whether this heartbeat completed the course or
// issued a badge.
func newEntry(actor Actor, target string) *domain.AuditEntry {
	actorID := actor.ID
	return &domain.AuditEntry{
		ActorID:        &actorID,
		TargetResource: target,
		IPAddress:      actor.IPAddress,
		UserAgent:      actor.UserAgent,
	}
}

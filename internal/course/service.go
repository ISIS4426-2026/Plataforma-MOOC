// Package course implements course authoring: creating a draft, reading it
// back, listing the published catalog, and editing a draft's metadata
// (issue #17).
package course

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// Deps are the ports the service depends on.
type Deps struct {
	Courses domain.CourseRepository
}

// Actor is who is performing an authoring action, and from where. It is
// recorded on every audit entry, the same role Actor plays in the admin
// package.
type Actor struct {
	ID        string
	Role      domain.Role
	IPAddress string
	UserAgent string
}

// Service exposes course authoring operations.
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

// Create makes a new course draft.
//
// The role check is defence in depth: the route is already restricted to
// professors and administrators by middleware.RequireRole, but a service that
// only trusts its transport layer breaks the moment a second caller (a batch
// job, another handler) reuses it without going through that middleware.
func (s *Service) Create(ctx context.Context, actor Actor, title, description string) (*domain.Course, error) {
	if !canAuthor(actor.Role) {
		return nil, domain.ErrForbidden
	}
	if err := validateMetadata(title, description); err != nil {
		return nil, err
	}

	course := &domain.Course{
		ID:          uuid.NewString(),
		StableID:    uuid.NewString(),
		Title:       title,
		Description: description,
		Version:     1,
		Status:      domain.CourseStatusDraft,
		AuthorID:    actor.ID,
	}

	// Generated up front, rather than read back after the insert, so the
	// audit entry recorded alongside the insert (same transaction) can name
	// the resource it created instead of an empty target.
	entry := newEntry(actor, domain.AuditActionCourseCreated, "course:"+course.ID)
	if err := s.deps.Courses.Create(ctx, course, entry); err != nil {
		return nil, err
	}

	return course, nil
}

// Get returns a single course so a client can read its ETag before
// attempting a conditional write.
func (s *Service) Get(ctx context.Context, courseID string) (*domain.Course, error) {
	return s.deps.Courses.GetByID(ctx, courseID)
}

// ListPublished returns one page of the public course catalog. Unlike Create
// and Update, it carries no role check: the catalog is public.
func (s *Service) ListPublished(ctx context.Context, filter domain.CourseFilter) (*domain.CoursePage, error) {
	return s.deps.Courses.ListPublished(ctx, filter)
}

// Update changes a draft's title and description.
//
// Ownership is checked here rather than inside the repository transaction,
// unlike the published-status and ETag checks Update itself enforces: an
// author_id assigned at creation never changes underneath a concurrent
// request, so there is no race for a row lock to close, only a permission to
// verify before spending a round trip on a write that would be refused
// anyway.
func (s *Service) Update(ctx context.Context, actor Actor, courseID, title, description string, opts domain.ChangeOptions) (*domain.Course, error) {
	if !canAuthor(actor.Role) {
		return nil, domain.ErrForbidden
	}
	if err := validateMetadata(title, description); err != nil {
		return nil, err
	}

	current, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if actor.Role != domain.RoleAdmin && current.AuthorID != actor.ID {
		return nil, domain.ErrForbidden
	}

	entry := newEntry(actor, domain.AuditActionCourseUpdated, "course:"+courseID)

	return s.deps.Courses.Update(ctx, courseID, title, description, opts, entry)
}

// canAuthor reports whether role may create or edit course drafts (issue #17,
// acceptance criterion: "Solo profesores autorizados y administradores pueden
// crear/editar cursos").
func canAuthor(role domain.Role) bool {
	return role == domain.RoleProfessor || role == domain.RoleAdmin
}

func validateMetadata(title, description string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title is required: %w", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("description is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

func newEntry(actor Actor, action domain.AuditAction, targetResource string) *domain.AuditEntry {
	actorID := actor.ID
	return &domain.AuditEntry{
		ActorID:        &actorID,
		Action:         action,
		TargetResource: targetResource,
		IPAddress:      actor.IPAddress,
		UserAgent:      actor.UserAgent,
	}
}

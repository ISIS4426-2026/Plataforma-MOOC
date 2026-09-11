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

	// Modules, Units and Resources back Publish's structural validation
	// (issue #20): confirming the course has at least one module containing
	// a unit containing a visible, available resource, with a mandatory one
	// somewhere in that path to define what completion requires.
	Modules   domain.ModuleRepository
	Units     domain.UnitRepository
	Resources domain.ResourceRepository
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
	if err := validateMetadata(title, description); err != nil {
		return nil, err
	}

	current, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(actor, current); err != nil {
		return nil, err
	}

	entry := newEntry(actor, domain.AuditActionCourseUpdated, "course:"+courseID)

	return s.deps.Courses.Update(ctx, courseID, title, description, opts, entry)
}

// Publish makes a draft the course's published version.
//
// Validation collects every problem before returning, rather than stopping
// at the first (issue #20's third acceptance criterion): a professor
// filling in a course for the first time would otherwise spend one round
// trip per missing field instead of seeing the whole list at once. A
// non-empty domain.ValidationErrors is returned as the error itself --
// callers use errors.As to recover the list.
func (s *Service) Publish(ctx context.Context, actor Actor, courseID string) (*domain.Course, error) {
	current, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(actor, current); err != nil {
		return nil, err
	}
	if current.Status == domain.CourseStatusPublished {
		return nil, fmt.Errorf("course %s is already published: %w", courseID, domain.ErrConflict)
	}

	validationErrs, err := s.validateForPublish(ctx, current)
	if err != nil {
		return nil, err
	}
	if len(validationErrs) > 0 {
		return nil, validationErrs
	}

	entry := newEntry(actor, domain.AuditActionCourseNewVersion, "course:"+courseID)
	return s.deps.Courses.UpdateStatus(ctx, courseID, domain.CourseStatusPublished, entry)
}

// Unpublish takes a published course off the catalog and makes it editable
// again. This is the MVP's answer to "editing a published course" (section
// 5.1 of the spec): rather than forking a new draft version, the same row
// is temporarily unpublished, edited, and republished. Forking a real new
// version with change classification is explicitly optional/Should scope
// (section 5.2), not something issue #20 asks for.
func (s *Service) Unpublish(ctx context.Context, actor Actor, courseID string) (*domain.Course, error) {
	current, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(actor, current); err != nil {
		return nil, err
	}
	if current.Status != domain.CourseStatusPublished {
		return nil, fmt.Errorf("course %s is not published: %w", courseID, domain.ErrConflict)
	}

	entry := newEntry(actor, domain.AuditActionCourseUnpublished, "course:"+courseID)
	return s.deps.Courses.UpdateStatus(ctx, courseID, domain.CourseStatusUnpublished, entry)
}

// validateForPublish checks issue #20's publication requirements: complete
// metadata, minimal structure (at least one module containing a unit
// containing a visible, available resource), and approval criteria defined.
//
// "Criterios de aprobación definidos" has no dedicated field in this domain
// model -- course approval (issue outside #15-#21) is computed from which
// resources are marked mandatory, so "defined" is read as: at least one
// resource on the path that satisfies the structural minimum is mandatory.
// Zero mandatory resources would mean nothing determines whether a student
// has completed the course.
//
// A resource counts as "disponible" when its processing has finished or it
// never needed any (ProcessingStatus is only meaningful for media types;
// text and quiz resources are available immediately).
func (s *Service) validateForPublish(ctx context.Context, course *domain.Course) (domain.ValidationErrors, error) {
	var errs domain.ValidationErrors

	if strings.TrimSpace(course.Title) == "" {
		errs = append(errs, domain.FieldError{Field: "title", Message: "El curso debe tener un título."})
	}
	if strings.TrimSpace(course.Description) == "" {
		errs = append(errs, domain.FieldError{Field: "description", Message: "El curso debe tener una descripción."})
	}

	modules, err := s.deps.Modules.ListByCourse(ctx, course.ID)
	if err != nil {
		return nil, fmt.Errorf("list modules of course %s: %w", course.ID, err)
	}

	hasMinimalStructure := false
	hasApprovalCriteria := false

	for _, m := range modules {
		units, err := s.deps.Units.ListByModule(ctx, m.ID)
		if err != nil {
			return nil, fmt.Errorf("list units of module %s: %w", m.ID, err)
		}
		for _, u := range units {
			resources, err := s.deps.Resources.ListByUnit(ctx, u.ID)
			if err != nil {
				return nil, fmt.Errorf("list resources of unit %s: %w", u.ID, err)
			}
			for _, res := range resources {
				available := res.ProcessingStatus == "" || res.ProcessingStatus == "completed"
				if !res.IsVisible || !available {
					continue
				}
				hasMinimalStructure = true
				if res.IsMandatory {
					hasApprovalCriteria = true
				}
			}
		}
	}

	if !hasMinimalStructure {
		errs = append(errs, domain.FieldError{
			Field:   "structure",
			Message: "El curso debe tener al menos un módulo con una unidad con un recurso visible y disponible.",
		})
	}
	if !hasApprovalCriteria {
		errs = append(errs, domain.FieldError{
			Field:   "approval_criteria",
			Message: "El curso debe tener al menos un recurso obligatorio visible y disponible que defina el criterio de aprobación.",
		})
	}

	return errs, nil
}

// authorize enforces #17's rule, carried over here since publishing and
// unpublishing are also authoring actions: only the course's author, or an
// administrator, may perform them.
func (s *Service) authorize(actor Actor, course *domain.Course) error {
	if !canAuthor(actor.Role) {
		return domain.ErrForbidden
	}
	if actor.Role != domain.RoleAdmin && course.AuthorID != actor.ID {
		return domain.ErrForbidden
	}
	return nil
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

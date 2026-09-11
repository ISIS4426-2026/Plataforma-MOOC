// Package structure implements CRUD for the content inside a course --
// Module, Unit and Resource -- respecting the Curso -> Módulo -> Unidad ->
// Recurso hierarchy (issue #19).
package structure

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
	Courses   domain.CourseRepository
	Modules   domain.ModuleRepository
	Units     domain.UnitRepository
	Resources domain.ResourceRepository
}

// Actor is who is performing an authoring action, and from where -- the same
// role Actor plays in the course and admin packages.
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

// ---- Modules ----------------------------------------------------------

// CreateModule adds a module to the end of a course.
func (s *Service) CreateModule(ctx context.Context, actor Actor, courseID, title string) (*domain.Module, error) {
	if err := s.authorizeOnCourse(ctx, actor, courseID); err != nil {
		return nil, err
	}
	if err := requireTitle(title); err != nil {
		return nil, err
	}

	module := &domain.Module{ID: uuid.NewString(), StableID: uuid.NewString(), CourseID: courseID, Title: title}
	entry := newEntry(actor, domain.AuditActionModuleCreated, "module:"+module.ID)
	if err := s.deps.Modules.Create(ctx, module, entry); err != nil {
		return nil, err
	}
	return module, nil
}

func (s *Service) ListModules(ctx context.Context, courseID string) ([]*domain.Module, error) {
	return s.deps.Modules.ListByCourse(ctx, courseID)
}

func (s *Service) UpdateModule(ctx context.Context, actor Actor, moduleID, title string) (*domain.Module, error) {
	if err := requireTitle(title); err != nil {
		return nil, err
	}
	module, err := s.deps.Modules.GetByID(ctx, moduleID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnCourse(ctx, actor, module.CourseID); err != nil {
		return nil, err
	}

	entry := newEntry(actor, domain.AuditActionModuleUpdated, "module:"+moduleID)
	return s.deps.Modules.Update(ctx, moduleID, title, entry)
}

func (s *Service) DeleteModule(ctx context.Context, actor Actor, moduleID string) error {
	module, err := s.deps.Modules.GetByID(ctx, moduleID)
	if err != nil {
		return err
	}
	if err := s.authorizeOnCourse(ctx, actor, module.CourseID); err != nil {
		return err
	}

	entry := newEntry(actor, domain.AuditActionModuleDeleted, "module:"+moduleID)
	return s.deps.Modules.Delete(ctx, moduleID, entry)
}

// ---- Units --------------------------------------------------------------

func (s *Service) CreateUnit(ctx context.Context, actor Actor, moduleID, title string) (*domain.Unit, error) {
	if err := requireTitle(title); err != nil {
		return nil, err
	}
	module, err := s.deps.Modules.GetByID(ctx, moduleID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnCourse(ctx, actor, module.CourseID); err != nil {
		return nil, err
	}

	unit := &domain.Unit{ID: uuid.NewString(), StableID: uuid.NewString(), ModuleID: moduleID, Title: title}
	entry := newEntry(actor, domain.AuditActionUnitCreated, "unit:"+unit.ID)
	if err := s.deps.Units.Create(ctx, unit, entry); err != nil {
		return nil, err
	}
	return unit, nil
}

func (s *Service) ListUnits(ctx context.Context, moduleID string) ([]*domain.Unit, error) {
	return s.deps.Units.ListByModule(ctx, moduleID)
}

func (s *Service) UpdateUnit(ctx context.Context, actor Actor, unitID, title string) (*domain.Unit, error) {
	if err := requireTitle(title); err != nil {
		return nil, err
	}
	courseID, err := s.courseIDForUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnCourse(ctx, actor, courseID); err != nil {
		return nil, err
	}

	entry := newEntry(actor, domain.AuditActionUnitUpdated, "unit:"+unitID)
	return s.deps.Units.Update(ctx, unitID, title, entry)
}

func (s *Service) DeleteUnit(ctx context.Context, actor Actor, unitID string) error {
	courseID, err := s.courseIDForUnit(ctx, unitID)
	if err != nil {
		return err
	}
	if err := s.authorizeOnCourse(ctx, actor, courseID); err != nil {
		return err
	}

	entry := newEntry(actor, domain.AuditActionUnitDeleted, "unit:"+unitID)
	return s.deps.Units.Delete(ctx, unitID, entry)
}

func (s *Service) courseIDForUnit(ctx context.Context, unitID string) (string, error) {
	unit, err := s.deps.Units.GetByID(ctx, unitID)
	if err != nil {
		return "", err
	}
	module, err := s.deps.Modules.GetByID(ctx, unit.ModuleID)
	if err != nil {
		return "", err
	}
	return module.CourseID, nil
}

// ---- Resources ------------------------------------------------------------

// NewResourceInput carries what a caller supplies when adding a resource.
// IsVisible and IsMandatory default to true, matching how a professor
// authors a course incrementally: a resource is visible and required to
// pass unless explicitly marked otherwise, not the other way around.
type NewResourceInput struct {
	Title         string
	Type          domain.ResourceType
	IsVisible     bool
	IsMandatory   bool
	AllowDownload bool
	ContentText   string
}

func (s *Service) CreateResource(ctx context.Context, actor Actor, unitID string, input NewResourceInput) (*domain.Resource, error) {
	if err := requireTitle(input.Title); err != nil {
		return nil, err
	}
	if !isKnownResourceType(input.Type) {
		return nil, fmt.Errorf("unknown resource type %q: %w", input.Type, domain.ErrInvalidInput)
	}

	courseID, err := s.courseIDForUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnCourse(ctx, actor, courseID); err != nil {
		return nil, err
	}

	resource := &domain.Resource{
		ID: uuid.NewString(), StableID: uuid.NewString(), UnitID: unitID,
		Title: input.Title, Type: input.Type,
		IsVisible: input.IsVisible, IsMandatory: input.IsMandatory, AllowDownload: input.AllowDownload,
		ContentText: input.ContentText,
	}
	entry := newEntry(actor, domain.AuditActionResourceCreated, "resource:"+resource.ID)
	if err := s.deps.Resources.Create(ctx, resource, entry); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *Service) ListResources(ctx context.Context, unitID string) ([]*domain.Resource, error) {
	return s.deps.Resources.ListByUnit(ctx, unitID)
}

func (s *Service) UpdateResource(ctx context.Context, actor Actor, resourceID string, fields domain.ResourceUpdate) (*domain.Resource, error) {
	if err := requireTitle(fields.Title); err != nil {
		return nil, err
	}
	courseID, err := s.courseIDForResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnCourse(ctx, actor, courseID); err != nil {
		return nil, err
	}

	entry := newEntry(actor, domain.AuditActionResourceUpdated, "resource:"+resourceID)
	return s.deps.Resources.Update(ctx, resourceID, fields, entry)
}

func (s *Service) DeleteResource(ctx context.Context, actor Actor, resourceID string) error {
	courseID, err := s.courseIDForResource(ctx, resourceID)
	if err != nil {
		return err
	}
	if err := s.authorizeOnCourse(ctx, actor, courseID); err != nil {
		return err
	}

	entry := newEntry(actor, domain.AuditActionResourceDeleted, "resource:"+resourceID)
	return s.deps.Resources.Delete(ctx, resourceID, entry)
}

func (s *Service) courseIDForResource(ctx context.Context, resourceID string) (string, error) {
	resource, err := s.deps.Resources.GetByID(ctx, resourceID)
	if err != nil {
		return "", err
	}
	return s.courseIDForUnit(ctx, resource.UnitID)
}

// ---- shared -----------------------------------------------------------

// authorizeOnCourse enforces issue #17's rule (carried over here since it's
// the same course that owns the structure): only the course's author, or an
// administrator, may change it. Defence in depth, like course.Service: the
// route is already restricted to professors and administrators by
// middleware.RequireRole.
func (s *Service) authorizeOnCourse(ctx context.Context, actor Actor, courseID string) error {
	if actor.Role != domain.RoleProfessor && actor.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	course, err := s.deps.Courses.GetByID(ctx, courseID)
	if err != nil {
		return err
	}
	if actor.Role != domain.RoleAdmin && course.AuthorID != actor.ID {
		return domain.ErrForbidden
	}
	return nil
}

func requireTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

func isKnownResourceType(t domain.ResourceType) bool {
	switch t {
	case domain.ResourceTypeRichText, domain.ResourceTypeImage, domain.ResourceTypeVideo, domain.ResourceTypeAudio,
		domain.ResourceTypePDF, domain.ResourceTypePresentation, domain.ResourceTypeDownloadable,
		domain.ResourceTypeIframe, domain.ResourceTypeExternalLink, domain.ResourceTypeQuiz:
		return true
	default:
		return false
	}
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

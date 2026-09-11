package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

func newDraftCourseForStructure(t *testing.T, courses *postgres.CourseRepository, authorID string) *domain.Course {
	t.Helper()
	c := newDraftCourse(authorID)
	if err := courses.Create(context.Background(), c, courseAuditEntry(authorID, c.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("failed to create draft course: %v", err)
	}
	return c
}

func newModule(courseID string) *domain.Module {
	return &domain.Module{ID: uuid.NewString(), StableID: uuid.NewString(), CourseID: courseID, Title: "Módulo 1"}
}

func newUnit(moduleID string) *domain.Unit {
	return &domain.Unit{ID: uuid.NewString(), StableID: uuid.NewString(), ModuleID: moduleID, Title: "Unidad 1"}
}

func newResource(unitID string) *domain.Resource {
	return &domain.Resource{
		ID: uuid.NewString(), StableID: uuid.NewString(), UnitID: unitID,
		Title: "Recurso 1", Type: domain.ResourceTypeRichText,
		IsVisible: true, IsMandatory: true, AllowDownload: false,
	}
}

func structureAuditEntry(actorID string, action domain.AuditAction, target string) *domain.AuditEntry {
	return &domain.AuditEntry{ActorID: &actorID, Action: action, TargetResource: target}
}

func TestModuleRepositoryCreateAssignsSequentialPositions(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)

	first := newModule(course.ID)
	if err := modules.Create(ctx, first, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+first.ID)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	second := newModule(course.ID)
	if err := modules.Create(ctx, second, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+second.ID)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if first.Position != 0 {
		t.Errorf("first module position = %d, want 0", first.Position)
	}
	if second.Position != 1 {
		t.Errorf("second module position = %d, want 1", second.Position)
	}
}

func TestModuleRepositoryCreateRefusesAMissingCourse(t *testing.T) {
	db := newTestDB(t)
	modules := postgres.NewModuleRepository(db)
	ctx := context.Background()

	actorID := uuid.NewString()
	m := newModule(uuid.NewString())
	err := modules.Create(ctx, m, structureAuditEntry(actorID, domain.AuditActionModuleCreated, "module:"+m.ID))

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestModuleRepositoryCreateRefusesAPublishedCourse(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)
	if _, err := db.ExecContext(ctx, `UPDATE courses SET status = $1 WHERE id = $2`, domain.CourseStatusPublished, course.ID); err != nil {
		t.Fatalf("failed to publish the course directly: %v", err)
	}

	m := newModule(course.ID)
	err := modules.Create(ctx, m, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+m.ID))

	if !errors.Is(err, domain.ErrCourseImmutable) {
		t.Fatalf("expected domain.ErrCourseImmutable, got %v", err)
	}
}

// The acceptance test issue #19 asks for directly: a unit can't be created
// without an existing module.
func TestUnitRepositoryCreateRefusesAMissingModule(t *testing.T) {
	db := newTestDB(t)
	units := postgres.NewUnitRepository(db)
	ctx := context.Background()

	actorID := uuid.NewString()
	u := newUnit(uuid.NewString())
	err := units.Create(ctx, u, structureAuditEntry(actorID, domain.AuditActionUnitCreated, "unit:"+u.ID))

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

// Same rule one level down: no resource without an existing unit.
func TestResourceRepositoryCreateRefusesAMissingUnit(t *testing.T) {
	db := newTestDB(t)
	resources := postgres.NewResourceRepository(db)
	ctx := context.Background()

	actorID := uuid.NewString()
	res := newResource(uuid.NewString())
	err := resources.Create(ctx, res, structureAuditEntry(actorID, domain.AuditActionResourceCreated, "resource:"+res.ID))

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestModuleRepositoryDeleteClosesThePositionGap(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)

	var created []*domain.Module
	for i := 0; i < 3; i++ {
		m := newModule(course.ID)
		if err := modules.Create(ctx, m, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+m.ID)); err != nil {
			t.Fatalf("Create returned an error: %v", err)
		}
		created = append(created, m)
	}
	// Positions are now 0, 1, 2. Delete the middle one.
	if err := modules.Delete(ctx, created[1].ID, structureAuditEntry(author.ID, domain.AuditActionModuleDeleted, "module:"+created[1].ID)); err != nil {
		t.Fatalf("Delete returned an error: %v", err)
	}

	remaining, err := modules.ListByCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("ListByCourse returned an error: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("got %d modules, want 2", len(remaining))
	}
	if remaining[0].ID != created[0].ID || remaining[0].Position != 0 {
		t.Errorf("first remaining module = %+v, want %s at position 0", remaining[0], created[0].ID)
	}
	if remaining[1].ID != created[2].ID || remaining[1].Position != 1 {
		t.Errorf("second remaining module = %+v, want %s at position 1 (closed gap)", remaining[1], created[2].ID)
	}
}

func TestResourceRepositoryUpdatePersistsFields(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	units := postgres.NewUnitRepository(db)
	resources := postgres.NewResourceRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)
	module := newModule(course.ID)
	if err := modules.Create(ctx, module, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+module.ID)); err != nil {
		t.Fatalf("Create module returned an error: %v", err)
	}
	unit := newUnit(module.ID)
	if err := units.Create(ctx, unit, structureAuditEntry(author.ID, domain.AuditActionUnitCreated, "unit:"+unit.ID)); err != nil {
		t.Fatalf("Create unit returned an error: %v", err)
	}
	resource := newResource(unit.ID)
	if err := resources.Create(ctx, resource, structureAuditEntry(author.ID, domain.AuditActionResourceCreated, "resource:"+resource.ID)); err != nil {
		t.Fatalf("Create resource returned an error: %v", err)
	}

	updated, err := resources.Update(ctx, resource.ID, domain.ResourceUpdate{
		Title: "Recurso actualizado", IsVisible: false, IsMandatory: false, AllowDownload: true,
	}, structureAuditEntry(author.ID, domain.AuditActionResourceUpdated, "resource:"+resource.ID))
	if err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}

	if updated.Title != "Recurso actualizado" || updated.IsVisible || updated.IsMandatory || !updated.AllowDownload {
		t.Errorf("updated resource = %+v, fields did not persist as expected", updated)
	}
	// Type is immutable through Update.
	if updated.Type != domain.ResourceTypeRichText {
		t.Errorf("resource type changed to %q, want it untouched", updated.Type)
	}
}

func TestResourceRepositoryCreateRefusesAPublishedCourse(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	units := postgres.NewUnitRepository(db)
	resources := postgres.NewResourceRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)
	module := newModule(course.ID)
	if err := modules.Create(ctx, module, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+module.ID)); err != nil {
		t.Fatalf("Create module returned an error: %v", err)
	}
	unit := newUnit(module.ID)
	if err := units.Create(ctx, unit, structureAuditEntry(author.ID, domain.AuditActionUnitCreated, "unit:"+unit.ID)); err != nil {
		t.Fatalf("Create unit returned an error: %v", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE courses SET status = $1 WHERE id = $2`, domain.CourseStatusPublished, course.ID); err != nil {
		t.Fatalf("failed to publish the course directly: %v", err)
	}

	res := newResource(unit.ID)
	err := resources.Create(ctx, res, structureAuditEntry(author.ID, domain.AuditActionResourceCreated, "resource:"+res.ID))

	if !errors.Is(err, domain.ErrCourseImmutable) {
		t.Fatalf("expected domain.ErrCourseImmutable, got %v", err)
	}
}

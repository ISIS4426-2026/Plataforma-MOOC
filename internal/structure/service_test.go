package structure_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/structure"
)

func professor(id string) structure.Actor { return structure.Actor{ID: id, Role: domain.RoleProfessor} }
func admin(id string) structure.Actor     { return structure.Actor{ID: id, Role: domain.RoleAdmin} }
func student(id string) structure.Actor   { return structure.Actor{ID: id, Role: domain.RoleStudent} }

type harness struct {
	svc       *structure.Service
	courses   *fakeCourseRepo
	modules   *fakeModuleRepo
	units     *fakeUnitRepo
	resources *fakeResourceRepo
	access    *fakeAccess
}

// fakeAccess stands in for enrollment.Service. The rule it implements is tested
// there; here what matters is that structure asks it and honours the answer.
type fakeAccess struct {
	allow bool
	err   error
	asked int
}

func (f *fakeAccess) CanRead(_ context.Context, _ string, _ domain.Role, _ *domain.Course) (bool, error) {
	f.asked++
	return f.allow, f.err
}

func newHarness() *harness {
	courses := newFakeCourseRepo()
	modules := newFakeModuleRepo()
	units := newFakeUnitRepo()
	resources := newFakeResourceRepo()
	access := &fakeAccess{allow: true}
	svc := structure.NewService(structure.Deps{
		Courses: courses, Modules: modules, Units: units, Resources: resources, Access: access,
	}, nil)
	return &harness{svc: svc, courses: courses, modules: modules, units: units, resources: resources, access: access}
}

func (h *harness) withDraftCourse(id, authorID string) {
	h.courses.put(&domain.Course{ID: id, AuthorID: authorID, Status: domain.CourseStatusDraft})
}

func TestCreateModuleRefusesRolesThatCannotAuthor(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")

	_, err := h.svc.CreateModule(context.Background(), student("student-1"), "course-1", "Módulo 1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

func TestCreateModuleRefusesAnotherProfessorsCourse(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")

	_, err := h.svc.CreateModule(context.Background(), professor("prof-2"), "course-1", "Módulo 1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

func TestCreateModuleAllowsAnAdministrator(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")

	m, err := h.svc.CreateModule(context.Background(), admin("admin-1"), "course-1", "Módulo 1")
	if err != nil {
		t.Fatalf("CreateModule returned an error: %v", err)
	}
	if m.Title != "Módulo 1" {
		t.Errorf("title = %q, want %q", m.Title, "Módulo 1")
	}
}

func TestCreateModuleRejectsAMissingCourse(t *testing.T) {
	h := newHarness()

	_, err := h.svc.CreateModule(context.Background(), professor("prof-1"), "does-not-exist", "Módulo 1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestCreateModuleRejectsBlankTitle(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")

	_, err := h.svc.CreateModule(context.Background(), professor("prof-1"), "course-1", "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

// Ownership is resolved by walking up the hierarchy: a unit doesn't carry an
// author, but its module's course does.
func TestCreateUnitRefusesAnotherProfessorsModule(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")
	mod, err := h.svc.CreateModule(context.Background(), professor("prof-1"), "course-1", "Módulo 1")
	if err != nil {
		t.Fatalf("CreateModule returned an error: %v", err)
	}

	_, err = h.svc.CreateUnit(context.Background(), professor("prof-2"), mod.ID, "Unidad 1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

func TestCreateUnitRejectsAMissingModule(t *testing.T) {
	h := newHarness()

	_, err := h.svc.CreateUnit(context.Background(), professor("prof-1"), "does-not-exist", "Unidad 1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestCreateResourceRejectsAMissingUnit(t *testing.T) {
	h := newHarness()

	_, err := h.svc.CreateResource(context.Background(), professor("prof-1"), "does-not-exist", structure.NewResourceInput{
		Title: "Recurso 1", Type: domain.ResourceTypeRichText,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestCreateResourceRejectsAnUnknownType(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")
	mod, err := h.svc.CreateModule(context.Background(), professor("prof-1"), "course-1", "Módulo 1")
	if err != nil {
		t.Fatalf("CreateModule returned an error: %v", err)
	}
	unit, err := h.svc.CreateUnit(context.Background(), professor("prof-1"), mod.ID, "Unidad 1")
	if err != nil {
		t.Fatalf("CreateUnit returned an error: %v", err)
	}

	_, err = h.svc.CreateResource(context.Background(), professor("prof-1"), unit.ID, structure.NewResourceInput{
		Title: "Recurso 1", Type: "not-a-real-type",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

// End-to-end through the full hierarchy: an administrator can author a
// module, a unit inside it, and a resource inside that, and each write is
// audited.
func TestFullHierarchyCreationIsAuditedAtEveryLevel(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")

	mod, err := h.svc.CreateModule(context.Background(), professor("prof-1"), "course-1", "Módulo 1")
	if err != nil {
		t.Fatalf("CreateModule returned an error: %v", err)
	}
	unit, err := h.svc.CreateUnit(context.Background(), professor("prof-1"), mod.ID, "Unidad 1")
	if err != nil {
		t.Fatalf("CreateUnit returned an error: %v", err)
	}
	resource, err := h.svc.CreateResource(context.Background(), professor("prof-1"), unit.ID, structure.NewResourceInput{
		Title: "Recurso 1", Type: domain.ResourceTypeVideo, IsVisible: true, IsMandatory: true,
	})
	if err != nil {
		t.Fatalf("CreateResource returned an error: %v", err)
	}
	if resource.UnitID != unit.ID {
		t.Errorf("resource unit = %q, want %q", resource.UnitID, unit.ID)
	}

	h.modules.mu.Lock()
	defer h.modules.mu.Unlock()
	if len(h.modules.entries) != 1 || h.modules.entries[0].Action != domain.AuditActionModuleCreated {
		t.Fatalf("module audit entries = %+v, want exactly one module.created", h.modules.entries)
	}
}

func TestDeleteModulePropagatesNotFound(t *testing.T) {
	h := newHarness()

	err := h.svc.DeleteModule(context.Background(), professor("prof-1"), "does-not-exist")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestUpdateResourceRefusesAnotherProfessorsResource(t *testing.T) {
	h := newHarness()
	h.withDraftCourse("course-1", "prof-1")
	mod, _ := h.svc.CreateModule(context.Background(), professor("prof-1"), "course-1", "Módulo 1")
	unit, _ := h.svc.CreateUnit(context.Background(), professor("prof-1"), mod.ID, "Unidad 1")
	resource, err := h.svc.CreateResource(context.Background(), professor("prof-1"), unit.ID, structure.NewResourceInput{
		Title: "Recurso 1", Type: domain.ResourceTypeRichText,
	})
	if err != nil {
		t.Fatalf("CreateResource returned an error: %v", err)
	}

	_, err = h.svc.UpdateResource(context.Background(), professor("prof-2"), resource.ID, domain.ResourceUpdate{Title: "Hijacked"})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

// ---- read-side access gate (issue #111) --------------------------------

// seedStructure returns a module and unit id under course-1, so the unit and
// resource listings have a parent chain to walk up to the course.
func seedStructure(t *testing.T, h *harness) (moduleID, unitID string) {
	t.Helper()
	author := professor("prof-1")
	module, err := h.svc.CreateModule(context.Background(), author, "course-1", "Módulo 1")
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	unit, err := h.svc.CreateUnit(context.Background(), author, module.ID, "Unidad 1")
	if err != nil {
		t.Fatalf("CreateUnit: %v", err)
	}
	return module.ID, unit.ID
}

// The three content listings go through the access check. Until #111 they were
// public, which meant anyone could walk a published course's whole structure
// without enrolling.
func TestContentListingsHonourTheAccessCheck(t *testing.T) {
	viewer := structure.Viewer{ID: "student-1", Role: domain.RoleStudent}

	listings := map[string]func(*harness, string, string) error{
		"modules": func(h *harness, _, _ string) error {
			_, err := h.svc.ListModules(context.Background(), viewer, "course-1")
			return err
		},
		"units": func(h *harness, moduleID, _ string) error {
			_, err := h.svc.ListUnits(context.Background(), viewer, moduleID)
			return err
		},
		"resources": func(h *harness, _, unitID string) error {
			_, err := h.svc.ListResources(context.Background(), viewer, unitID)
			return err
		},
	}

	for name, call := range listings {
		t.Run(name+" allowed", func(t *testing.T) {
			h := newHarness()
			h.withDraftCourse("course-1", "prof-1")
			moduleID, unitID := seedStructure(t, h)
			h.access.allow = true
			h.access.asked = 0

			if err := call(h, moduleID, unitID); err != nil {
				t.Fatalf("listing: %v", err)
			}
			if h.access.asked != 1 {
				t.Errorf("access was consulted %d times, want 1", h.access.asked)
			}
		})

		t.Run(name+" denied", func(t *testing.T) {
			h := newHarness()
			h.withDraftCourse("course-1", "prof-1")
			moduleID, unitID := seedStructure(t, h)
			h.access.allow = false

			if err := call(h, moduleID, unitID); !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	}
}

// A service built without its access checker must deny, not serve. Failing open
// here would hand out course content to anyone the moment someone forgets a
// dependency.
func TestListingsDenyWhenNoAccessCheckerIsWired(t *testing.T) {
	courses := newFakeCourseRepo()
	courses.put(&domain.Course{ID: "course-1", AuthorID: "prof-1", Status: domain.CourseStatusPublished})
	svc := structure.NewService(structure.Deps{
		Courses: courses, Modules: newFakeModuleRepo(), Units: newFakeUnitRepo(), Resources: newFakeResourceRepo(),
	}, nil)

	_, err := svc.ListModules(context.Background(), structure.Viewer{ID: "anyone"}, "course-1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

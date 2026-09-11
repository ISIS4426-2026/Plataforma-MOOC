package course_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/course"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// publishHarness wires a course.Service with fake structural repositories a
// test can populate directly, to drive Publish's validation without a
// database.
type publishHarness struct {
	svc       *course.Service
	courses   *fakeCourseRepo
	modules   *fakeModuleRepo
	units     *fakeUnitRepo
	resources *fakeResourceRepo
}

func newPublishHarness() *publishHarness {
	courses := newFakeCourseRepo()
	modules := &fakeModuleRepo{byCourse: map[string][]*domain.Module{}}
	units := &fakeUnitRepo{byModule: map[string][]*domain.Unit{}}
	resources := &fakeResourceRepo{byUnit: map[string][]*domain.Resource{}}
	svc := course.NewService(course.Deps{
		Courses: courses, Modules: modules, Units: units, Resources: resources,
	}, nil)
	return &publishHarness{svc: svc, courses: courses, modules: modules, units: units, resources: resources}
}

// withCompleteCourse populates a course with valid metadata and a minimal
// valid structure: one module, one unit, one visible+available+mandatory
// resource -- everything Publish requires.
func (h *publishHarness) withCompleteCourse(id, authorID string) {
	h.courses.courses[id] = &domain.Course{
		ID: id, AuthorID: authorID, Status: domain.CourseStatusDraft,
		Title: "Cloud 101", Description: "Intro to the cloud",
	}
	h.modules.byCourse[id] = []*domain.Module{{ID: "mod-1", CourseID: id}}
	h.units.byModule["mod-1"] = []*domain.Unit{{ID: "unit-1", ModuleID: "mod-1"}}
	h.resources.byUnit["unit-1"] = []*domain.Resource{
		{ID: "res-1", UnitID: "unit-1", IsVisible: true, IsMandatory: true, ProcessingStatus: "completed"},
	}
}

func TestPublishSucceedsWithACompleteCourse(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")

	published, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")
	if err != nil {
		t.Fatalf("Publish returned an error: %v", err)
	}
	if published.Status != domain.CourseStatusPublished {
		t.Errorf("status = %q, want %q", published.Status, domain.CourseStatusPublished)
	}
}

// The acceptance test issue #20 asks for directly, in its two failing
// halves: an incomplete draft is refused with a list of every problem, not
// just the first.
func TestPublishReturnsEveryValidationErrorAtOnce(t *testing.T) {
	h := newPublishHarness()
	h.courses.courses["course-1"] = &domain.Course{
		ID: "course-1", AuthorID: "prof-1", Status: domain.CourseStatusDraft,
		Title: "", Description: "", // both metadata fields blank
	}
	// No modules at all: structure and approval-criteria both fail too.

	_, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")

	var validationErrs domain.ValidationErrors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("expected domain.ValidationErrors, got %v (%T)", err, err)
	}
	if len(validationErrs) != 4 {
		t.Fatalf("got %d validation errors, want 4 (title, description, structure, approval_criteria); errors: %+v",
			len(validationErrs), validationErrs)
	}

	fields := make(map[string]bool, len(validationErrs))
	for _, fe := range validationErrs {
		fields[fe.Field] = true
	}
	for _, want := range []string{"title", "description", "structure", "approval_criteria"} {
		if !fields[want] {
			t.Errorf("missing validation error for field %q", want)
		}
	}
}

func TestPublishRejectsAHiddenOnlyResource(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")
	h.resources.byUnit["unit-1"][0].IsVisible = false

	_, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")

	var validationErrs domain.ValidationErrors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("expected domain.ValidationErrors, got %v", err)
	}
	if len(validationErrs) != 2 {
		t.Fatalf("got %d validation errors, want 2 (structure and approval_criteria, since the only resource is hidden): %+v",
			len(validationErrs), validationErrs)
	}
}

func TestPublishRejectsAnUnprocessedResource(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")
	h.resources.byUnit["unit-1"][0].ProcessingStatus = "processing"

	_, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")
	if err == nil {
		t.Fatal("expected Publish to fail while the only resource is still processing")
	}
}

func TestPublishRejectsWhenNoResourceIsMandatory(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")
	h.resources.byUnit["unit-1"][0].IsMandatory = false

	_, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")

	var validationErrs domain.ValidationErrors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("expected domain.ValidationErrors, got %v", err)
	}
	if len(validationErrs) != 1 || validationErrs[0].Field != "approval_criteria" {
		t.Fatalf("validation errors = %+v, want exactly one for approval_criteria", validationErrs)
	}
}

func TestPublishRefusesAnAlreadyPublishedCourse(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")
	h.courses.courses["course-1"].Status = domain.CourseStatusPublished

	_, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected domain.ErrConflict, got %v", err)
	}
}

func TestPublishRefusesAnotherProfessorsCourse(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")

	_, err := h.svc.Publish(context.Background(), professor("prof-2"), "course-1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

func TestPublishRefusesRolesThatCannotAuthor(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")

	_, err := h.svc.Publish(context.Background(), student("student-1"), "course-1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

// Full round trip from issue #20's acceptance test: incomplete draft fails
// with a list of errors, gets completed, publishes, and the published
// version is then immutable (Update refuses it -- proven in service_test.go
// for issue #17; re-asserted here end to end).
func TestPublishRoundTripLeavesThePublishedVersionImmutable(t *testing.T) {
	h := newPublishHarness()
	h.courses.courses["course-1"] = &domain.Course{
		ID: "course-1", AuthorID: "prof-1", Status: domain.CourseStatusDraft,
	}

	if _, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1"); err == nil {
		t.Fatal("expected Publish to fail on the incomplete draft")
	}

	h.withCompleteCourse("course-1", "prof-1")

	published, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1")
	if err != nil {
		t.Fatalf("Publish returned an error after completing the draft: %v", err)
	}
	if published.Status != domain.CourseStatusPublished {
		t.Fatalf("status = %q, want %q", published.Status, domain.CourseStatusPublished)
	}

	_, err = h.svc.Update(context.Background(), professor("prof-1"), "course-1", "New title", "New description", domain.ChangeOptions{})
	if !errors.Is(err, domain.ErrCourseImmutable) {
		t.Fatalf("expected domain.ErrCourseImmutable editing the published version, got %v", err)
	}
}

func TestUnpublishMakesAPublishedCourseEditableAgain(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1")
	if _, err := h.svc.Publish(context.Background(), professor("prof-1"), "course-1"); err != nil {
		t.Fatalf("Publish returned an error: %v", err)
	}

	unpublished, err := h.svc.Unpublish(context.Background(), professor("prof-1"), "course-1")
	if err != nil {
		t.Fatalf("Unpublish returned an error: %v", err)
	}
	if unpublished.Status != domain.CourseStatusUnpublished {
		t.Fatalf("status = %q, want %q", unpublished.Status, domain.CourseStatusUnpublished)
	}

	if _, err := h.svc.Update(context.Background(), professor("prof-1"), "course-1", "New title", "New description", domain.ChangeOptions{}); err != nil {
		t.Fatalf("Update after unpublish returned an error: %v", err)
	}
}

func TestUnpublishRefusesADraftCourse(t *testing.T) {
	h := newPublishHarness()
	h.withCompleteCourse("course-1", "prof-1") // still a draft, never published

	_, err := h.svc.Unpublish(context.Background(), professor("prof-1"), "course-1")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected domain.ErrConflict, got %v", err)
	}
}

package course_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/course"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

func newTestService() (*course.Service, *fakeCourseRepo) {
	repo := newFakeCourseRepo()
	svc := course.NewService(course.Deps{Courses: repo}, nil)
	return svc, repo
}

func professor(id string) course.Actor { return course.Actor{ID: id, Role: domain.RoleProfessor} }
func admin(id string) course.Actor     { return course.Actor{ID: id, Role: domain.RoleAdmin} }
func student(id string) course.Actor   { return course.Actor{ID: id, Role: domain.RoleStudent} }

// A newly created course is born in draft status (issue #17, criterion 2).
func TestCreateStartsInDraftStatus(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if created.Status != domain.CourseStatusDraft {
		t.Errorf("new course status = %q, want %q", created.Status, domain.CourseStatusDraft)
	}
	if created.Version != 1 {
		t.Errorf("new course version = %d, want 1", created.Version)
	}
}

// Only professors and administrators may create courses (issue #17,
// criterion 3).
func TestCreateRefusesRolesThatCannotAuthor(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.Create(context.Background(), student("student-1"), "Cloud 101", "Intro to the cloud")

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

func TestCreateRejectsBlankMetadata(t *testing.T) {
	svc, _ := newTestService()

	tests := []struct {
		name        string
		title       string
		description string
	}{
		{"blank title", "   ", "a description"},
		{"blank description", "a title", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), professor("prof-1"), tt.title, tt.description)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
			}
		})
	}
}

// Editing a course whose current version is already published must not
// alter that published version (issue #17, criterion 4): the write is
// refused outright rather than silently applied.
func TestUpdateRefusesToTouchAPublishedCourse(t *testing.T) {
	svc, repo := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	// Simulate publication directly on the fake, as issue #20 owns Publish.
	repo.mu.Lock()
	repo.courses[created.ID].Status = domain.CourseStatusPublished
	originalTitle := repo.courses[created.ID].Title
	repo.mu.Unlock()

	_, err = svc.Update(context.Background(), professor("prof-1"), created.ID, "Changed title", "Changed description", domain.ChangeOptions{})
	if !errors.Is(err, domain.ErrCourseImmutable) {
		t.Fatalf("expected domain.ErrCourseImmutable, got %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.courses[created.ID].Title != originalTitle {
		t.Errorf("published course title changed to %q, want it untouched at %q",
			repo.courses[created.ID].Title, originalTitle)
	}
}

// A professor may edit their own draft.
func TestUpdateAllowsTheOwningProfessor(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	updated, err := svc.Update(context.Background(), professor("prof-1"), created.ID, "Cloud 101 v2", "Updated description", domain.ChangeOptions{})
	if err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}
	if updated.Title != "Cloud 101 v2" {
		t.Errorf("title = %q, want %q", updated.Title, "Cloud 101 v2")
	}
}

// A professor may not edit a draft authored by someone else.
func TestUpdateRefusesAnotherProfessorsCourse(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	_, err = svc.Update(context.Background(), professor("prof-2"), created.ID, "Hijacked title", "Hijacked description", domain.ChangeOptions{})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

// An administrator may edit any professor's draft.
func TestUpdateAllowsAnAdministratorRegardlessOfAuthor(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	updated, err := svc.Update(context.Background(), admin("admin-1"), created.ID, "Cloud 101 v2", "Updated description", domain.ChangeOptions{})
	if err != nil {
		t.Fatalf("Update by an administrator returned an error: %v", err)
	}
	if updated.Title != "Cloud 101 v2" {
		t.Errorf("title = %q, want %q", updated.Title, "Cloud 101 v2")
	}
}

func TestUpdateRefusesRolesThatCannotAuthor(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	_, err = svc.Update(context.Background(), student("student-1"), created.ID, "x", "y", domain.ChangeOptions{})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected domain.ErrForbidden, got %v", err)
	}
}

// A stale If-Match is refused rather than silently overwriting a change the
// caller never saw.
func TestUpdateRefusesAStaleETag(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	_, err = svc.Update(context.Background(), professor("prof-1"), created.ID, "x", "y", domain.ChangeOptions{ExpectedETag: `"stale"`})
	if !errors.Is(err, domain.ErrPreconditionFailed) {
		t.Fatalf("expected domain.ErrPreconditionFailed, got %v", err)
	}
}

func TestEachCreateAndUpdateRecordsExactlyOneAuditEntry(t *testing.T) {
	svc, repo := newTestService()

	created, err := svc.Create(context.Background(), professor("prof-1"), "Cloud 101", "Intro to the cloud")
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if _, err := svc.Update(context.Background(), professor("prof-1"), created.ID, "Cloud 101 v2", "desc", domain.ChangeOptions{}); err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.entries) != 2 {
		t.Fatalf("recorded %d audit entries, want 2 (create + update)", len(repo.entries))
	}
	if repo.entries[0].Action != domain.AuditActionCourseCreated {
		t.Errorf("first entry action = %q, want %q", repo.entries[0].Action, domain.AuditActionCourseCreated)
	}
	if repo.entries[0].TargetResource != "course:"+created.ID {
		t.Errorf("first entry target = %q, want %q", repo.entries[0].TargetResource, "course:"+created.ID)
	}
	if repo.entries[1].Action != domain.AuditActionCourseUpdated {
		t.Errorf("second entry action = %q, want %q", repo.entries[1].Action, domain.AuditActionCourseUpdated)
	}
}

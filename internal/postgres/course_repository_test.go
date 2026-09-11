package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// newAuthorUser stores a professor who can own a course, satisfying the
// courses.author_id foreign key.
func newAuthorUser(t *testing.T, repo *postgres.UserRepository) *domain.User {
	t.Helper()

	user := newTestUser()
	user.Role = domain.RoleProfessor
	user.Status = domain.UserStatusActive
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create the author: %v", err)
	}
	return user
}

func newDraftCourse(authorID string) *domain.Course {
	return &domain.Course{
		ID:          uuid.NewString(),
		StableID:    uuid.NewString(),
		Title:       "Cloud 101",
		Description: "Intro to the cloud",
		Version:     1,
		Status:      domain.CourseStatusDraft,
		AuthorID:    authorID,
	}
}

func courseAuditEntry(actorID, courseID string, action domain.AuditAction) *domain.AuditEntry {
	return &domain.AuditEntry{
		ActorID:        &actorID,
		Action:         action,
		TargetResource: "course:" + courseID,
	}
}

func TestCourseRepositoryCreatePersistsTheDraftAndItsAuditEntry(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	draft := newDraftCourse(author.ID)

	entry := courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)
	if err := courses.Create(ctx, draft, entry); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if draft.CreatedAt.IsZero() {
		t.Error("CreatedAt was not filled in by Create")
	}
	if entry.ID == "" {
		t.Error("audit entry was not recorded: it has no id")
	}

	found, err := courses.GetByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}
	if found.Status != domain.CourseStatusDraft {
		t.Errorf("status = %q, want %q", found.Status, domain.CourseStatusDraft)
	}
}

// The acceptance test issue #17 asks for directly: editing a course whose
// current version is already published must not alter that published
// version.
func TestCourseRepositoryUpdateRefusesAPublishedCourse(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	draft := newDraftCourse(author.ID)
	if err := courses.Create(ctx, draft, courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE courses SET status = $1 WHERE id = $2`,
		domain.CourseStatusPublished, draft.ID); err != nil {
		t.Fatalf("failed to mark the course published directly: %v", err)
	}

	_, err := courses.Update(ctx, draft.ID, "Changed title", "Changed description", domain.ChangeOptions{},
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseUpdated))

	if !errors.Is(err, domain.ErrCourseImmutable) {
		t.Fatalf("expected domain.ErrCourseImmutable, got %v", err)
	}

	current, err := courses.GetByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}
	if current.Title != draft.Title {
		t.Errorf("published course title changed to %q, want it untouched at %q", current.Title, draft.Title)
	}
}

func TestCourseRepositoryUpdateRefusesAStaleETag(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	draft := newDraftCourse(author.ID)
	if err := courses.Create(ctx, draft, courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	_, err := courses.Update(ctx, draft.ID, "x", "y", domain.ChangeOptions{ExpectedETag: `"stale"`},
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseUpdated))

	if !errors.Is(err, domain.ErrPreconditionFailed) {
		t.Fatalf("expected domain.ErrPreconditionFailed, got %v", err)
	}
}

func TestCourseRepositoryUpdatePersistsWithAFreshETag(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	draft := newDraftCourse(author.ID)
	if err := courses.Create(ctx, draft, courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	updated, err := courses.Update(ctx, draft.ID, "Cloud 101 v2", "Updated description",
		domain.ChangeOptions{ExpectedETag: draft.ETag()},
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseUpdated))
	if err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}
	if updated.Title != "Cloud 101 v2" {
		t.Errorf("title = %q, want %q", updated.Title, "Cloud 101 v2")
	}
	if updated.ETag() == draft.ETag() {
		t.Error("ETag did not change after a successful update")
	}
}

func TestCourseRepositoryListPublishedOnlyReturnsPublishedCourses(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)

	draft := newDraftCourse(author.ID)
	if err := courses.Create(ctx, draft, courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	published := newDraftCourse(author.ID)
	published.Title = "Published Course " + uuid.NewString()
	if err := courses.Create(ctx, published, courseAuditEntry(author.ID, published.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE courses SET status = $1 WHERE id = $2`,
		domain.CourseStatusPublished, published.ID); err != nil {
		t.Fatalf("failed to mark the course published directly: %v", err)
	}

	page, err := courses.ListPublished(ctx, domain.CourseFilter{Search: published.Title})
	if err != nil {
		t.Fatalf("ListPublished returned an error: %v", err)
	}

	if len(page.Courses) != 1 || page.Courses[0].ID != published.ID {
		t.Fatalf("ListPublished returned %d courses, want exactly the one published course matching the search", len(page.Courses))
	}
}

func TestCourseRepositoryUpdateStatusTransitionsAndAudits(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	audit := postgres.NewAuditRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	draft := newDraftCourse(author.ID)
	if err := courses.Create(ctx, draft, courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	published, err := courses.UpdateStatus(ctx, draft.ID, domain.CourseStatusPublished,
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseNewVersion))
	if err != nil {
		t.Fatalf("UpdateStatus returned an error: %v", err)
	}
	if published.Status != domain.CourseStatusPublished {
		t.Fatalf("status = %q, want %q", published.Status, domain.CourseStatusPublished)
	}

	page, err := audit.List(ctx, domain.AuditFilter{TargetResource: "course:" + draft.ID, ActionPrefix: "course."})
	if err != nil {
		t.Fatalf("audit List returned an error: %v", err)
	}
	found := false
	for _, e := range page.Entries {
		if e.Action == domain.AuditActionCourseNewVersion {
			found = true
		}
	}
	if !found {
		t.Errorf("no %q audit entry found for course %s", domain.AuditActionCourseNewVersion, draft.ID)
	}
}

// UpdateStatus itself applies whatever transition it is given -- it is
// course.Service that decides a transition is allowed before calling it, so
// this only proves the repository does not silently refuse a published ->
// unpublished move, the one course.Service.Unpublish depends on.
func TestCourseRepositoryUpdateStatusAllowsUnpublishing(t *testing.T) {
	db := newTestDB(t)
	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	ctx := context.Background()

	author := newAuthorUser(t, users)
	draft := newDraftCourse(author.ID)
	if err := courses.Create(ctx, draft, courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseCreated)); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if _, err := courses.UpdateStatus(ctx, draft.ID, domain.CourseStatusPublished,
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseNewVersion)); err != nil {
		t.Fatalf("publish UpdateStatus returned an error: %v", err)
	}

	unpublished, err := courses.UpdateStatus(ctx, draft.ID, domain.CourseStatusUnpublished,
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseUnpublished))
	if err != nil {
		t.Fatalf("unpublish UpdateStatus returned an error: %v", err)
	}
	if unpublished.Status != domain.CourseStatusUnpublished {
		t.Fatalf("status = %q, want %q", unpublished.Status, domain.CourseStatusUnpublished)
	}

	// And the course is editable again through the normal Update path.
	if _, err := courses.Update(ctx, draft.ID, "New title", "New description", domain.ChangeOptions{},
		courseAuditEntry(author.ID, draft.ID, domain.AuditActionCourseUpdated)); err != nil {
		t.Fatalf("Update after unpublish returned an error: %v", err)
	}
}

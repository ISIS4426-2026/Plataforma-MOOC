package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// progressFixture is a published course with mandatory resources and a student
// to work through them, which is the minimum any progress test needs.
type progressFixture struct {
	course    *domain.Course
	resources []*domain.Resource
	student   *domain.User
	progress  *postgres.ProgressRepository
	badges    *postgres.BadgeRepository
}

// newProgressFixture builds the course as a draft, fills it, and only then
// publishes it: resource writes are refused on a published course, which is the
// immutability rule the authoring side enforces.
func newProgressFixture(t *testing.T, db *sql.DB, mandatoryResources int) *progressFixture {
	t.Helper()
	ctx := context.Background()

	users := postgres.NewUserRepository(db)
	courses := postgres.NewCourseRepository(db)
	modules := postgres.NewModuleRepository(db)
	units := postgres.NewUnitRepository(db)
	resources := postgres.NewResourceRepository(db)

	author := newAuthorUser(t, users)
	course := newDraftCourseForStructure(t, courses, author.ID)

	module := newModule(course.ID)
	if err := modules.Create(ctx, module, structureAuditEntry(author.ID, domain.AuditActionModuleCreated, "module:"+module.ID)); err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	unit := newUnit(module.ID)
	if err := units.Create(ctx, unit, structureAuditEntry(author.ID, domain.AuditActionUnitCreated, "unit:"+unit.ID)); err != nil {
		t.Fatalf("failed to create unit: %v", err)
	}

	created := make([]*domain.Resource, 0, mandatoryResources)
	for i := 0; i < mandatoryResources; i++ {
		resource := newResource(unit.ID)
		if err := resources.Create(ctx, resource, structureAuditEntry(author.ID, domain.AuditActionResourceCreated, "resource:"+resource.ID)); err != nil {
			t.Fatalf("failed to create resource %d: %v", i, err)
		}
		created = append(created, resource)
	}

	if _, err := courses.UpdateStatus(ctx, course.ID, domain.CourseStatusPublished,
		courseAuditEntry(author.ID, course.ID, domain.AuditActionCourseNewVersion)); err != nil {
		t.Fatalf("failed to publish the course: %v", err)
	}

	student := newTestUser()
	student.Status = domain.UserStatusActive
	if err := users.Create(ctx, student); err != nil {
		t.Fatalf("failed to create the student: %v", err)
	}

	return &progressFixture{
		course:    course,
		resources: created,
		student:   student,
		progress:  postgres.NewProgressRepository(db),
		badges:    postgres.NewBadgeRepository(db),
	}
}

func (f *progressFixture) locate(t *testing.T, resource *domain.Resource) *domain.ResourceLocation {
	t.Helper()
	at, err := f.progress.Locate(context.Background(), resource.ID)
	if err != nil {
		t.Fatalf("Locate returned an error: %v", err)
	}
	return at
}

func (f *progressFixture) heartbeat(t *testing.T, resource *domain.Resource, completed bool) (*domain.StudentProgress, *domain.Badge) {
	t.Helper()
	hb := domain.Heartbeat{StudentID: f.student.ID, ResourceID: resource.ID, DwellTimeSeconds: 30, Completed: completed}
	progress, badge, err := f.progress.RecordHeartbeat(context.Background(), hb, f.locate(t, resource), f.auditEntry())
	if err != nil {
		t.Fatalf("RecordHeartbeat returned an error: %v", err)
	}
	return progress, badge
}

func (f *progressFixture) auditEntry() *domain.AuditEntry {
	actor := f.student.ID
	return &domain.AuditEntry{ActorID: &actor, TargetResource: "course:" + f.course.StableID}
}

func TestProgressRepositoryLocateResolvesTheCourse(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 1)

	at := fixture.locate(t, fixture.resources[0])

	if at.CourseID != fixture.course.ID {
		t.Errorf("CourseID = %q, want %q", at.CourseID, fixture.course.ID)
	}
	if at.CourseStableID != fixture.course.StableID {
		t.Errorf("CourseStableID = %q, want %q", at.CourseStableID, fixture.course.StableID)
	}
	if at.ResourceStableID != fixture.resources[0].StableID {
		t.Errorf("ResourceStableID = %q, want %q", at.ResourceStableID, fixture.resources[0].StableID)
	}
	if at.CourseStatus != domain.CourseStatusPublished {
		t.Errorf("CourseStatus = %q, want published", at.CourseStatus)
	}
}

func TestProgressRepositoryLocateUnknownResource(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewProgressRepository(db)

	_, err := repo.Locate(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Locate error = %v, want ErrNotFound", err)
	}
}

// This is the first acceptance criterion of issue #113: repeated heartbeats on
// the same resource must not inflate the percentage.
func TestProgressRepositoryRepeatedHeartbeatsDoNotInflate(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 4)

	var last *domain.StudentProgress
	for i := 0; i < 5; i++ {
		last, _ = fixture.heartbeat(t, fixture.resources[0], true)
	}

	if last.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d after five heartbeats on one resource, want 1", last.CompletedCount)
	}
	if last.TotalCount != 4 {
		t.Errorf("TotalCount = %d, want 4", last.TotalCount)
	}
	if last.PercentCompleted != 25 {
		t.Errorf("PercentCompleted = %v, want 25", last.PercentCompleted)
	}
	if len(last.CompletedResources) != 1 {
		t.Errorf("CompletedResources = %v, want exactly one entry", last.CompletedResources)
	}
	if last.IsApproved {
		t.Error("IsApproved = true with one of four resources done")
	}
}

// A heartbeat that reports time but not completion advances nothing, and that is
// the point: dwell time is evidence of activity, not of finishing.
func TestProgressRepositoryIncompleteHeartbeatDoesNotCount(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 2)

	progress, _ := fixture.heartbeat(t, fixture.resources[0], false)

	if progress.CompletedCount != 0 {
		t.Errorf("CompletedCount = %d, want 0", progress.CompletedCount)
	}
	if progress.PercentCompleted != 0 {
		t.Errorf("PercentCompleted = %v, want 0", progress.PercentCompleted)
	}
}

// The second acceptance criterion: N concurrent heartbeats leave a consistent
// state. Every resource is reported once, at the same time, and all of them must
// survive -- a lost update would show up as a count below the total.
func TestProgressRepositoryConcurrentHeartbeatsAreConsistent(t *testing.T) {
	db := newTestDB(t)
	const resourceCount = 8

	// The pool is bounded because the test databases do not bound theirs and
	// Postgres allows 100 connections in total. Opening eight at once is enough
	// to make `go test ./...` -- which runs packages in parallel -- fail in an
	// unrelated test that can no longer get a connection.
	//
	// Four is still concurrency: the heartbeats contend for the same
	// student_progress row, which is what this test is about, and the goroutines
	// that cannot get a connection queue rather than disappear.
	db.SetMaxOpenConns(4)

	fixture := newProgressFixture(t, db, resourceCount)

	var wg sync.WaitGroup
	errs := make(chan error, resourceCount)
	for _, resource := range fixture.resources {
		wg.Add(1)
		go func(r *domain.Resource) {
			defer wg.Done()
			at, err := fixture.progress.Locate(context.Background(), r.ID)
			if err != nil {
				errs <- err
				return
			}
			hb := domain.Heartbeat{StudentID: fixture.student.ID, ResourceID: r.ID, DwellTimeSeconds: 10, Completed: true}
			if _, _, err := fixture.progress.RecordHeartbeat(context.Background(), hb, at, fixture.auditEntry()); err != nil {
				errs <- err
			}
		}(resource)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a concurrent heartbeat failed: %v", err)
	}

	final, err := fixture.progress.GetStudentProgress(context.Background(), fixture.student.ID, fixture.course.ID)
	if err != nil {
		t.Fatalf("GetStudentProgress returned an error: %v", err)
	}
	if final.CompletedCount != resourceCount {
		t.Errorf("CompletedCount = %d after %d concurrent heartbeats, want %d",
			final.CompletedCount, resourceCount, resourceCount)
	}
	if final.PercentCompleted != 100 {
		t.Errorf("PercentCompleted = %v, want 100", final.PercentCompleted)
	}
	if len(final.CompletedResources) != resourceCount {
		t.Errorf("CompletedResources has %d entries, want %d", len(final.CompletedResources), resourceCount)
	}
}

// Completing every resource issues the badge, once, and the verification code it
// carries resolves -- the third acceptance criterion, minus the HTTP layer.
func TestProgressRepositoryCompletionIssuesOneBadge(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 2)
	ctx := context.Background()

	if _, badge := fixture.heartbeat(t, fixture.resources[0], true); badge != nil {
		t.Fatal("a badge was issued with the course half done")
	}

	progress, badge := fixture.heartbeat(t, fixture.resources[1], true)
	if !progress.IsApproved {
		t.Fatalf("IsApproved = false with every resource completed (%d of %d)",
			progress.CompletedCount, progress.TotalCount)
	}
	if badge == nil {
		t.Fatal("completing the course issued no badge")
	}
	if badge.VerificationCode == "" {
		t.Error("the badge carries no verification code")
	}
	if badge.ImageKey != domain.BadgeImageKey(fixture.student.ID, fixture.course.StableID) {
		t.Errorf("ImageKey = %q, want the derived key", badge.ImageKey)
	}

	// A later heartbeat must not issue a second badge, and must not report the
	// first one as newly earned.
	if _, again := fixture.heartbeat(t, fixture.resources[1], true); again != nil {
		t.Errorf("a second badge was issued: %s", again.ID)
	}

	credential, err := fixture.badges.Verify(ctx, badge.VerificationCode)
	if err != nil {
		t.Fatalf("Verify returned an error: %v", err)
	}
	if !credential.Valid() {
		t.Error("the freshly issued credential is not valid")
	}
	if credential.CourseTitle != fixture.course.Title {
		t.Errorf("CourseTitle = %q, want %q", credential.CourseTitle, fixture.course.Title)
	}
}

// The verification endpoint is public and the spec calls it privacy-preserving.
// This pins that down at the type level: if a field naming the student is ever
// added to the credential, this stops compiling and whoever added it has to
// decide deliberately rather than by accident.
func TestBadgeCredentialCarriesNothingAboutTheStudent(t *testing.T) {
	fields := reflect.VisibleFields(reflect.TypeOf(domain.BadgeCredential{}))
	allowed := map[string]bool{"CourseTitle": true, "IssuedAt": true, "IsRevoked": true}
	for _, field := range fields {
		if !allowed[field.Name] {
			t.Errorf("BadgeCredential carries %q; public verification must not reveal the student", field.Name)
		}
	}
}

func TestBadgeRepositoryVerifyUnknownCode(t *testing.T) {
	db := newTestDB(t)
	repo := postgres.NewBadgeRepository(db)

	_, err := repo.Verify(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Verify error = %v, want ErrNotFound", err)
	}
}

// A student who has reported nothing is at zero, not missing. A 404 here would
// force every client to treat "not started" as an error.
func TestProgressRepositoryUnreportedCourseIsZero(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 3)

	progress, err := fixture.progress.GetStudentProgress(context.Background(), fixture.student.ID, fixture.course.ID)
	if err != nil {
		t.Fatalf("GetStudentProgress returned an error: %v", err)
	}
	if progress.CompletedCount != 0 || progress.PercentCompleted != 0 {
		t.Errorf("progress = %d/%d (%v%%), want zero", progress.CompletedCount, progress.TotalCount, progress.PercentCompleted)
	}
	if progress.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", progress.TotalCount)
	}
	if progress.CompletedResources == nil {
		t.Error("CompletedResources is nil; want an empty slice so it serialises as []")
	}
}

// The denominator is the mandatory, visible resources: an optional resource must
// not stand between a student and completing the course, and a hidden one cannot
// be reached at all.
func TestProgressRepositoryCountsOnlyMandatoryVisibleResources(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 1)
	ctx := context.Background()

	// The course is published by now, so the extra resources go in underneath
	// the repository: authoring writes are refused on a published course, and
	// what is under test is the counting, not the authoring rule.
	for _, extra := range []struct{ mandatory, visible bool }{{false, true}, {true, false}} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO resources (stable_id, unit_id, title, type, position, is_visible, is_mandatory)
			SELECT gen_random_uuid(), r.unit_id, 'Extra', 'text',
			       (SELECT COUNT(*) FROM resources WHERE unit_id = r.unit_id), $2, $1
			FROM resources r WHERE r.id = $3`,
			extra.mandatory, extra.visible, fixture.resources[0].ID); err != nil {
			t.Fatalf("failed to insert the extra resource: %v", err)
		}
	}

	progress, badge := fixture.heartbeat(t, fixture.resources[0], true)

	if progress.TotalCount != 1 {
		t.Errorf("TotalCount = %d with one mandatory visible resource plus an optional and a hidden one, want 1", progress.TotalCount)
	}
	if progress.PercentCompleted != 100 {
		t.Errorf("PercentCompleted = %v, want 100", progress.PercentCompleted)
	}
	if badge == nil {
		t.Error("completing every mandatory resource issued no badge")
	}
}

// A completed id that no longer names a resource of this course must not count.
// The set is keyed by stable ids that outlive versions, so it can hold leftovers
// -- and a leftover that counted would carry a student towards a badge they did
// not earn.
func TestProgressRepositoryStaleCompletedIDDoesNotCount(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 2)
	ctx := context.Background()

	fixture.heartbeat(t, fixture.resources[0], true)

	// Plant an id that belongs to nothing in this course, the way a deleted
	// resource or a previous version would leave one behind.
	if _, err := db.ExecContext(ctx, `
		UPDATE student_progress
		SET completed_resources = completed_resources || to_jsonb(gen_random_uuid()::text)
		WHERE student_id = $1 AND course_stable_id = $2`,
		fixture.student.ID, fixture.course.StableID); err != nil {
		t.Fatalf("failed to plant the stale id: %v", err)
	}

	progress, err := fixture.progress.GetStudentProgress(ctx, fixture.student.ID, fixture.course.ID)
	if err != nil {
		t.Fatalf("GetStudentProgress returned an error: %v", err)
	}
	if progress.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d with one real completion and one stale id, want 1", progress.CompletedCount)
	}
	if progress.PercentCompleted != 50 {
		t.Errorf("PercentCompleted = %v, want 50", progress.PercentCompleted)
	}
}

func TestBadgeRepositoryGetByID(t *testing.T) {
	db := newTestDB(t)
	fixture := newProgressFixture(t, db, 1)
	ctx := context.Background()

	_, issued := fixture.heartbeat(t, fixture.resources[0], true)
	if issued == nil {
		t.Fatal("completing the course issued no badge")
	}

	got, err := fixture.badges.GetByID(ctx, issued.ID)
	if err != nil {
		t.Fatalf("GetByID returned an error: %v", err)
	}
	if got.ID != issued.ID || got.StudentID != fixture.student.ID {
		t.Errorf("GetByID returned %+v, want the badge of student %s", got, fixture.student.ID)
	}
	if got.VerificationCode != issued.VerificationCode {
		t.Errorf("VerificationCode = %q, want %q", got.VerificationCode, issued.VerificationCode)
	}
	// The ETag is what makes the conditional GET on this resource work, so it has
	// to survive a round trip through the database unchanged.
	if got.ETag() != issued.ETag() {
		t.Errorf("ETag changed across a read: %q then %q", issued.ETag(), got.ETag())
	}

	if _, err := fixture.badges.GetByID(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID error for an unknown id = %v, want ErrNotFound", err)
	}
}

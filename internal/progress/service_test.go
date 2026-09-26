package progress_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/progress"
)

const (
	studentID      = "11111111-1111-1111-1111-111111111111"
	authorID       = "22222222-2222-2222-2222-222222222222"
	resourceID     = "33333333-3333-3333-3333-333333333333"
	courseStableID = "44444444-4444-4444-4444-444444444444"
)

type fakeProgressRepo struct {
	at *domain.ResourceLocation

	locateErr error
	recordErr error

	recorded []domain.Heartbeat
	result   *domain.StudentProgress
	badge    *domain.Badge

	stored *domain.StudentProgress
}

func (f *fakeProgressRepo) Locate(context.Context, string) (*domain.ResourceLocation, error) {
	if f.locateErr != nil {
		return nil, f.locateErr
	}
	return f.at, nil
}

func (f *fakeProgressRepo) RecordHeartbeat(_ context.Context, hb domain.Heartbeat, _ *domain.ResourceLocation, _ *domain.AuditEntry) (*domain.StudentProgress, *domain.Badge, error) {
	if f.recordErr != nil {
		return nil, nil, f.recordErr
	}
	f.recorded = append(f.recorded, hb)
	return f.result, f.badge, nil
}

func (f *fakeProgressRepo) GetStudentProgress(context.Context, string, string) (*domain.StudentProgress, error) {
	return f.stored, nil
}

type fakeBadgeRepo struct {
	held       *domain.Badge
	byID       *domain.Badge
	credential *domain.BadgeCredential
	verifyErr  error
	lookups    int
}

func (f *fakeBadgeRepo) GetByID(context.Context, string) (*domain.Badge, error) {
	if f.byID == nil {
		return nil, domain.ErrNotFound
	}
	return f.byID, nil
}

func (f *fakeBadgeRepo) Verify(context.Context, string) (*domain.BadgeCredential, error) {
	if f.verifyErr != nil {
		return nil, f.verifyErr
	}
	return f.credential, nil
}

func (f *fakeBadgeRepo) GetByStudentAndCourse(context.Context, string, string) (*domain.Badge, error) {
	f.lookups++
	if f.held == nil {
		return nil, domain.ErrNotFound
	}
	return f.held, nil
}

type fakeEnrollmentCheck struct {
	active bool
	err    error
	asked  int
}

func (f *fakeEnrollmentCheck) IsActive(context.Context, string, string) (bool, error) {
	f.asked++
	return f.active, f.err
}

func publishedCourse() *domain.ResourceLocation {
	return &domain.ResourceLocation{
		ResourceStableID: "55555555-5555-5555-5555-555555555555",
		CourseID:         "66666666-6666-6666-6666-666666666666",
		CourseStableID:   courseStableID,
		CourseStatus:     domain.CourseStatusPublished,
		CourseAuthorID:   authorID,
	}
}

func newService(t *testing.T, repo *fakeProgressRepo, badges *fakeBadgeRepo, check progress.EnrollmentCheck) *progress.Service {
	t.Helper()
	return progress.NewService(progress.Deps{Progress: repo, Badges: badges, Enrollments: check}, nil)
}

func student() progress.Actor {
	return progress.Actor{ID: studentID, Role: domain.RoleStudent}
}

func heartbeat() domain.Heartbeat {
	return domain.Heartbeat{ResourceID: resourceID, DwellTimeSeconds: 30, Completed: true}
}

func TestReportRequiresAnActiveEnrollment(t *testing.T) {
	repo := &fakeProgressRepo{at: publishedCourse()}
	check := &fakeEnrollmentCheck{active: false}
	service := newService(t, repo, &fakeBadgeRepo{}, check)

	_, _, err := service.Report(context.Background(), student(), heartbeat())

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Report error = %v, want ErrForbidden", err)
	}
	if len(repo.recorded) != 0 {
		t.Error("a heartbeat was recorded for a student with no active enrollment")
	}
}

// A service wired without its enrollment check must refuse every heartbeat. A
// missing dependency should fail visibly, not credit progress to anyone who asks.
func TestReportFailsClosedWithoutAnEnrollmentCheck(t *testing.T) {
	repo := &fakeProgressRepo{at: publishedCourse()}
	service := progress.NewService(progress.Deps{Progress: repo, Badges: &fakeBadgeRepo{}}, nil)

	_, _, err := service.Report(context.Background(), student(), heartbeat())

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Report error = %v, want ErrForbidden", err)
	}
	if len(repo.recorded) != 0 {
		t.Error("a heartbeat was recorded with no enrollment check wired")
	}
}

func TestReportRefusesAnUnpublishedCourse(t *testing.T) {
	for _, status := range []domain.CourseStatus{domain.CourseStatusDraft, domain.CourseStatusUnpublished} {
		t.Run(string(status), func(t *testing.T) {
			at := publishedCourse()
			at.CourseStatus = status
			repo := &fakeProgressRepo{at: at}
			check := &fakeEnrollmentCheck{active: true}
			service := newService(t, repo, &fakeBadgeRepo{}, check)

			_, _, err := service.Report(context.Background(), student(), heartbeat())

			if !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("Report error = %v, want ErrConflict", err)
			}
			// The status is checked before the enrollment: there is nothing to ask
			// about a course that is not on offer.
			if check.asked != 0 {
				t.Error("the enrollment was checked for a course that is not published")
			}
		})
	}
}

func TestReportRejectsImplausibleDwellTime(t *testing.T) {
	cases := map[string]int{
		"negative":       -1,
		"beyond the cap": domain.MaxHeartbeatDwellSeconds + 1,
		"absurdly large": 86400 * 365,
	}
	for name, dwell := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeProgressRepo{at: publishedCourse()}
			service := newService(t, repo, &fakeBadgeRepo{}, &fakeEnrollmentCheck{active: true})

			hb := heartbeat()
			hb.DwellTimeSeconds = dwell
			_, _, err := service.Report(context.Background(), student(), hb)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("Report error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestReportRejectsAHeartbeatWithoutAResource(t *testing.T) {
	service := newService(t, &fakeProgressRepo{at: publishedCourse()}, &fakeBadgeRepo{}, &fakeEnrollmentCheck{active: true})

	hb := heartbeat()
	hb.ResourceID = ""
	_, _, err := service.Report(context.Background(), student(), hb)

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Report error = %v, want ErrInvalidInput", err)
	}
}

// The heartbeat is credited to the authenticated caller, never to whoever a
// client names.
func TestReportCreditsTheCaller(t *testing.T) {
	repo := &fakeProgressRepo{
		at:     publishedCourse(),
		result: &domain.StudentProgress{CourseStableID: courseStableID, CompletedCount: 1, TotalCount: 3, PercentCompleted: 33.33},
	}
	service := newService(t, repo, &fakeBadgeRepo{}, &fakeEnrollmentCheck{active: true})

	hb := heartbeat()
	hb.StudentID = "impostor"
	updated, badge, err := service.Report(context.Background(), student(), hb)
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}

	if len(repo.recorded) != 1 {
		t.Fatalf("recorded %d heartbeats, want 1", len(repo.recorded))
	}
	if repo.recorded[0].StudentID != studentID {
		t.Errorf("recorded StudentID = %q, want the caller %q", repo.recorded[0].StudentID, studentID)
	}
	if updated.PercentCompleted != 33.33 {
		t.Errorf("PercentCompleted = %v, want 33.33", updated.PercentCompleted)
	}
	if badge != nil {
		t.Error("a badge came back for an unfinished course")
	}
}

// The repository hands back the badge only on the call that created it. Every
// later completed heartbeat must still report the credential, so a client that
// retried does not have to infer it from a missing field.
func TestReportReportsAnAlreadyHeldBadge(t *testing.T) {
	repo := &fakeProgressRepo{
		at:     publishedCourse(),
		result: &domain.StudentProgress{CourseStableID: courseStableID, CompletedCount: 3, TotalCount: 3, PercentCompleted: 100, IsApproved: true},
		badge:  nil,
	}
	held := &domain.Badge{ID: "badge-1", VerificationCode: "code-1", IssuedAt: time.Now()}
	badges := &fakeBadgeRepo{held: held}
	service := newService(t, repo, badges, &fakeEnrollmentCheck{active: true})

	_, badge, err := service.Report(context.Background(), student(), heartbeat())
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	if badge == nil {
		t.Fatal("no badge reported for an approved student who already holds one")
	}
	if badge.VerificationCode != "code-1" {
		t.Errorf("VerificationCode = %q, want code-1", badge.VerificationCode)
	}
}

// A freshly issued badge is returned as-is, without a second lookup.
func TestReportReturnsAFreshlyIssuedBadge(t *testing.T) {
	issued := &domain.Badge{ID: "badge-2", VerificationCode: "code-2", IssuedAt: time.Now()}
	repo := &fakeProgressRepo{
		at:     publishedCourse(),
		result: &domain.StudentProgress{CourseStableID: courseStableID, CompletedCount: 1, TotalCount: 1, PercentCompleted: 100, IsApproved: true},
		badge:  issued,
	}
	badges := &fakeBadgeRepo{}
	service := newService(t, repo, badges, &fakeEnrollmentCheck{active: true})

	_, badge, err := service.Report(context.Background(), student(), heartbeat())
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	if badge != issued {
		t.Errorf("badge = %v, want the freshly issued one", badge)
	}
	if badges.lookups != 0 {
		t.Errorf("the badge was looked up %d times although the repository already returned it", badges.lookups)
	}
}

// Reading one's own progress needs no enrollment: it is the caller's own record,
// and a student who withdrew has every reason to look at what they had done.
func TestCourseNeedsNoEnrollment(t *testing.T) {
	repo := &fakeProgressRepo{
		stored: &domain.StudentProgress{CourseStableID: courseStableID, CompletedCount: 2, TotalCount: 5, PercentCompleted: 40},
	}
	check := &fakeEnrollmentCheck{active: false}
	service := newService(t, repo, &fakeBadgeRepo{}, check)

	current, _, err := service.Course(context.Background(), student(), "course-id")
	if err != nil {
		t.Fatalf("Course returned an error: %v", err)
	}
	if current.PercentCompleted != 40 {
		t.Errorf("PercentCompleted = %v, want 40", current.PercentCompleted)
	}
	if check.asked != 0 {
		t.Error("reading one's own progress asked about the enrollment")
	}
}

func TestVerifyRejectsAnEmptyCode(t *testing.T) {
	service := newService(t, &fakeProgressRepo{}, &fakeBadgeRepo{}, &fakeEnrollmentCheck{active: true})

	_, err := service.Verify(context.Background(), "")

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Verify error = %v, want ErrInvalidInput", err)
	}
}

// A revoked badge resolves and reports itself invalid. Hiding it would be
// indistinguishable from a forgery, which is the opposite of what verification
// is for.
func TestVerifyResolvesARevokedBadge(t *testing.T) {
	badges := &fakeBadgeRepo{credential: &domain.BadgeCredential{
		CourseTitle: "Cloud 101",
		IssuedAt:    time.Now(),
		IsRevoked:   true,
	}}
	service := newService(t, &fakeProgressRepo{}, badges, &fakeEnrollmentCheck{active: true})

	credential, err := service.Verify(context.Background(), "code-3")
	if err != nil {
		t.Fatalf("Verify returned an error: %v", err)
	}
	if credential.Valid() {
		t.Error("a revoked credential reports itself valid")
	}
}

// A badge belongs to the student who earned it.
func TestBadgeReturnsTheCallersOwnBadge(t *testing.T) {
	own := &domain.Badge{ID: "badge-9", StudentID: studentID, VerificationCode: "code-9", IssuedAt: time.Now()}
	service := newService(t, &fakeProgressRepo{}, &fakeBadgeRepo{byID: own}, &fakeEnrollmentCheck{active: true})

	got, err := service.Badge(context.Background(), student(), "badge-9")
	if err != nil {
		t.Fatalf("Badge returned an error: %v", err)
	}
	if got != own {
		t.Errorf("Badge = %v, want the caller's own badge", got)
	}
}

// Someone else's badge is not found, not forbidden. The response carries the
// verification code, so confirming that an id names a real badge would let whoever
// learned the id claim the badge behind it.
func TestBadgeHidesSomeoneElsesBadge(t *testing.T) {
	stranger := &domain.Badge{ID: "badge-9", StudentID: "someone-else", VerificationCode: "code-9", IssuedAt: time.Now()}
	service := newService(t, &fakeProgressRepo{}, &fakeBadgeRepo{byID: stranger}, &fakeEnrollmentCheck{active: true})

	_, err := service.Badge(context.Background(), student(), "badge-9")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Badge error = %v, want ErrNotFound", err)
	}
	if errors.Is(err, domain.ErrForbidden) {
		t.Error("Badge answered forbidden, which confirms the badge exists")
	}
}

// An administrator reads any badge. The audit trail already records every
// issuance, so withholding the badge itself would protect nothing.
func TestBadgeIsVisibleToAnAdministrator(t *testing.T) {
	other := &domain.Badge{ID: "badge-9", StudentID: "someone-else", VerificationCode: "code-9", IssuedAt: time.Now()}
	service := newService(t, &fakeProgressRepo{}, &fakeBadgeRepo{byID: other}, &fakeEnrollmentCheck{active: true})

	admin := progress.Actor{ID: "admin-1", Role: domain.RoleAdmin}
	got, err := service.Badge(context.Background(), admin, "badge-9")
	if err != nil {
		t.Fatalf("Badge returned an error for an administrator: %v", err)
	}
	if got != other {
		t.Errorf("Badge = %v, want the requested badge", got)
	}
}

func TestBadgeRejectsAnEmptyID(t *testing.T) {
	service := newService(t, &fakeProgressRepo{}, &fakeBadgeRepo{}, &fakeEnrollmentCheck{active: true})

	_, err := service.Badge(context.Background(), student(), "")

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Badge error = %v, want ErrInvalidInput", err)
	}
}

// The verification link is built against the platform's public origin, the same
// one activation links use, and a badge whose state changed gets a different tag
// so a revocation cannot be served from a cache.
func TestBadgeVerificationURLAndETag(t *testing.T) {
	const code = "code-10"

	if got := domain.BadgeVerificationURL("http://localhost:8080/", code); got != "http://localhost:8080/api/v1/badges/verify/"+code {
		t.Errorf("BadgeVerificationURL = %q, want the trailing slash trimmed and the api path appended", got)
	}
	// No configured origin means no link. A verification URL pointing at a host
	// nobody configured is worse than none; the code alone still verifies.
	if got := domain.BadgeVerificationURL("", code); got != "" {
		t.Errorf("BadgeVerificationURL with no origin = %q, want empty", got)
	}

	issued := time.Now()
	active := &domain.Badge{ID: "badge-11", IssuedAt: issued}
	revoked := &domain.Badge{ID: "badge-11", IssuedAt: issued, IsRevoked: true}
	if active.ETag() == revoked.ETag() {
		t.Error("revoking a badge left its ETag unchanged; a cached copy would survive the revocation")
	}
}

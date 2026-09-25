package enrollment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

const (
	courseID       = "c0000000-0000-0000-0000-000000000001"
	courseStableID = "e0000000-0000-0000-0000-000000000001"
	authorID       = "a0000000-0000-0000-0000-000000000001"
	studentID      = "b0000000-0000-0000-0000-000000000001"
	otherStudentID = "b0000000-0000-0000-0000-000000000002"
)

// ---- doubles -----------------------------------------------------------

type fakeCourses struct{ course *domain.Course }

func (f *fakeCourses) GetByID(context.Context, string) (*domain.Course, error) {
	if f.course == nil {
		return nil, domain.ErrNotFound
	}
	return f.course, nil
}

// fakeEnrollments models the repository's contract, including the part that
// matters most here: withdrawing keeps the row.
type fakeEnrollments struct {
	mu      sync.Mutex
	rows    map[string]*domain.Enrollment // student|course
	actions []domain.AuditAction
}

func newFakeEnrollments() *fakeEnrollments {
	return &fakeEnrollments{rows: map[string]*domain.Enrollment{}}
}

func key(student, course string) string { return student + "|" + course }

func (f *fakeEnrollments) Enroll(_ context.Context, student, course string, entry *domain.AuditEntry) (*domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	existing, ok := f.rows[key(student, course)]
	if !ok {
		e := &domain.Enrollment{
			ID: "enr-" + student, StudentID: student, CourseStableID: course,
			Status: domain.EnrollmentActive, EnrolledAt: time.Now().UTC(),
		}
		f.rows[key(student, course)] = e
		if entry != nil {
			entry.Action = domain.AuditActionEnrollmentCreated
			f.actions = append(f.actions, entry.Action)
		}
		copied := *e
		return &copied, nil
	}
	if existing.Status == domain.EnrollmentWithdrawn {
		existing.Status = domain.EnrollmentActive
		existing.EnrolledAt = time.Now().UTC()
		if entry != nil {
			entry.Action = domain.AuditActionEnrollmentReactivated
			f.actions = append(f.actions, entry.Action)
		}
	}
	copied := *existing
	return &copied, nil
}

func (f *fakeEnrollments) Withdraw(_ context.Context, student, course string, entry *domain.AuditEntry) (*domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	existing, ok := f.rows[key(student, course)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if existing.Status == domain.EnrollmentActive {
		now := time.Now().UTC()
		existing.Status = domain.EnrollmentWithdrawn
		existing.WithdrawnAt = &now
		if entry != nil {
			entry.Action = domain.AuditActionEnrollmentWithdrawn
			f.actions = append(f.actions, entry.Action)
		}
	}
	copied := *existing
	return &copied, nil
}

func (f *fakeEnrollments) Get(_ context.Context, student, course string) (*domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.rows[key(student, course)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *e
	return &copied, nil
}

func (f *fakeEnrollments) IsActive(_ context.Context, student, course string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.rows[key(student, course)]
	return ok && e.Status == domain.EnrollmentActive, nil
}

func (f *fakeEnrollments) ListByStudent(_ context.Context, student string) ([]*domain.Enrollment, error) {
	return f.filter(func(e *domain.Enrollment) bool { return e.StudentID == student }), nil
}

func (f *fakeEnrollments) ListByCourse(_ context.Context, course string) ([]*domain.Enrollment, error) {
	return f.filter(func(e *domain.Enrollment) bool { return e.CourseStableID == course }), nil
}

func (f *fakeEnrollments) filter(keep func(*domain.Enrollment) bool) []*domain.Enrollment {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Enrollment
	for _, e := range f.rows {
		if keep(e) {
			copied := *e
			out = append(out, &copied)
		}
	}
	return out
}

func (f *fakeEnrollments) audited() []domain.AuditAction {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.AuditAction(nil), f.actions...)
}

// ---- harness -----------------------------------------------------------

func newService(t *testing.T, status domain.CourseStatus) (*Service, *fakeEnrollments) {
	t.Helper()
	repo := newFakeEnrollments()
	svc := NewService(Deps{
		Courses: &fakeCourses{course: &domain.Course{
			ID: courseID, StableID: courseStableID, AuthorID: authorID, Status: status,
		}},
		Enrollments: repo,
	}, nil)
	return svc, repo
}

func student() Actor { return Actor{ID: studentID, Role: domain.RoleStudent} }

// ---- enrolling ---------------------------------------------------------

func TestEnrollOnAPublishedCourse(t *testing.T) {
	svc, repo := newService(t, domain.CourseStatusPublished)

	e, err := svc.Enroll(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if !e.Active() {
		t.Errorf("status = %q, want active", e.Status)
	}
	// The enrollment keys on the stable id, not the row id, so it survives a new
	// published version of the course.
	if e.CourseStableID != courseStableID {
		t.Errorf("course_stable_id = %q, want %q", e.CourseStableID, courseStableID)
	}
	if got := repo.audited(); len(got) != 1 || got[0] != domain.AuditActionEnrollmentCreated {
		t.Errorf("audit trail = %v, want one enrollment.created", got)
	}
}

// Enrolling twice is the acceptance criterion: the caller asked to be enrolled
// and they are, so the existing enrollment comes back rather than a second row
// or an error.
func TestEnrollTwiceReturnsTheSameEnrollment(t *testing.T) {
	svc, repo := newService(t, domain.CourseStatusPublished)

	first, err := svc.Enroll(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("first Enroll: %v", err)
	}
	second, err := svc.Enroll(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("second Enroll: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("got two enrollments, %q and %q", first.ID, second.ID)
	}
	if !second.Active() {
		t.Errorf("status = %q, want active", second.Status)
	}
	// A repeat is not an event, so it must not fill the trail with noise.
	if got := repo.audited(); len(got) != 1 {
		t.Errorf("audit trail = %v, want a single entry", got)
	}
}

func TestEnrollRejectsACourseThatIsNotPublished(t *testing.T) {
	for _, status := range []domain.CourseStatus{domain.CourseStatusDraft, domain.CourseStatusUnpublished} {
		t.Run(string(status), func(t *testing.T) {
			svc, _ := newService(t, status)
			_, err := svc.Enroll(context.Background(), student(), courseID)
			if !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("error = %v, want ErrConflict", err)
			}
		})
	}
}

// An author reading their own course does not need an enrollment, and giving
// them one would put them on their own roster.
func TestEnrollRejectsTheAuthor(t *testing.T) {
	svc, _ := newService(t, domain.CourseStatusPublished)

	_, err := svc.Enroll(context.Background(), Actor{ID: authorID, Role: domain.RoleProfessor}, courseID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

// ---- withdrawing and returning -----------------------------------------

// The spec asks for "retiro y reinscripcion conservando progreso": the row has
// to survive the withdrawal, or the return would look like a first enrollment.
func TestWithdrawKeepsTheEnrollmentAndAllowsReturning(t *testing.T) {
	svc, repo := newService(t, domain.CourseStatusPublished)

	enrolled, err := svc.Enroll(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	withdrawn, err := svc.Withdraw(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if withdrawn.Status != domain.EnrollmentWithdrawn {
		t.Fatalf("status = %q, want withdrawn", withdrawn.Status)
	}
	if withdrawn.ID != enrolled.ID {
		t.Errorf("withdrawal replaced the enrollment: %q became %q", enrolled.ID, withdrawn.ID)
	}
	if withdrawn.WithdrawnAt == nil {
		t.Error("withdrawn_at was not recorded")
	}

	back, err := svc.Enroll(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("re-enrolling: %v", err)
	}
	if !back.Active() {
		t.Errorf("status = %q, want active", back.Status)
	}
	if back.ID != enrolled.ID {
		t.Errorf("returning created a new enrollment %q instead of reviving %q", back.ID, enrolled.ID)
	}

	// Returning is its own event, distinct from a first enrollment.
	want := []domain.AuditAction{
		domain.AuditActionEnrollmentCreated,
		domain.AuditActionEnrollmentWithdrawn,
		domain.AuditActionEnrollmentReactivated,
	}
	got := repo.audited()
	if len(got) != len(want) {
		t.Fatalf("audit trail = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("audit[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWithdrawWithoutAnEnrollment(t *testing.T) {
	svc, _ := newService(t, domain.CourseStatusPublished)

	if _, err := svc.Withdraw(context.Background(), student(), courseID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// A course can be unpublished while students are on it; they must still be able
// to leave.
func TestWithdrawWorksOnAnUnpublishedCourse(t *testing.T) {
	svc, repo := newService(t, domain.CourseStatusPublished)
	if _, err := svc.Enroll(context.Background(), student(), courseID); err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	svc.deps.Courses.(*fakeCourses).course.Status = domain.CourseStatusUnpublished

	withdrawn, err := svc.Withdraw(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if withdrawn.Status != domain.EnrollmentWithdrawn {
		t.Errorf("status = %q, want withdrawn", withdrawn.Status)
	}
	_ = repo
}

// ---- reading -----------------------------------------------------------

func TestStatusReportsNilWhenNeverEnrolled(t *testing.T) {
	svc, _ := newService(t, domain.CourseStatusPublished)

	current, err := svc.Status(context.Background(), student(), courseID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if current != nil {
		t.Errorf("got %+v, want nil for a student who never enrolled", current)
	}
}

func TestRosterIsRestrictedToTheAuthorAndAdmins(t *testing.T) {
	svc, _ := newService(t, domain.CourseStatusPublished)
	if _, err := svc.Enroll(context.Background(), student(), courseID); err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	t.Run("the author may read it", func(t *testing.T) {
		list, err := svc.Roster(context.Background(), Actor{ID: authorID, Role: domain.RoleProfessor}, courseID)
		if err != nil {
			t.Fatalf("Roster: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("got %d enrollments, want 1", len(list))
		}
	})

	t.Run("an administrator may read it", func(t *testing.T) {
		if _, err := svc.Roster(context.Background(), Actor{ID: "someone", Role: domain.RoleAdmin}, courseID); err != nil {
			t.Fatalf("Roster: %v", err)
		}
	})

	t.Run("another professor may not", func(t *testing.T) {
		_, err := svc.Roster(context.Background(), Actor{ID: "other", Role: domain.RoleProfessor}, courseID)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("error = %v, want ErrForbidden", err)
		}
	})

	t.Run("an enrolled student may not", func(t *testing.T) {
		_, err := svc.Roster(context.Background(), student(), courseID)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("error = %v, want ErrForbidden", err)
		}
	})
}

// ---- the access rule ---------------------------------------------------

func TestCanRead(t *testing.T) {
	published := &domain.Course{ID: courseID, StableID: courseStableID, AuthorID: authorID, Status: domain.CourseStatusPublished}
	draft := &domain.Course{ID: courseID, StableID: courseStableID, AuthorID: authorID, Status: domain.CourseStatusDraft}

	svc, _ := newService(t, domain.CourseStatusPublished)
	if _, err := svc.Enroll(context.Background(), student(), courseID); err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	cases := []struct {
		name     string
		viewerID string
		role     domain.Role
		course   *domain.Course
		want     bool
	}{
		{"the author reads their own draft", authorID, domain.RoleProfessor, draft, true},
		{"an administrator reads a draft", "any", domain.RoleAdmin, draft, true},
		{"an enrolled student reads the published course", studentID, domain.RoleStudent, published, true},
		{"a student who never enrolled does not", otherStudentID, domain.RoleStudent, published, false},
		{"an enrolled student does not read a draft", studentID, domain.RoleStudent, draft, false},
		{"an anonymous viewer never does", "", "", published, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.CanRead(context.Background(), tc.viewerID, tc.role, tc.course)
			if err != nil {
				t.Fatalf("CanRead: %v", err)
			}
			if got != tc.want {
				t.Errorf("CanRead = %v, want %v", got, tc.want)
			}
		})
	}
}

// Withdrawing has to close the door behind it, or leaving a course would be
// cosmetic.
func TestCanReadAfterWithdrawing(t *testing.T) {
	svc, _ := newService(t, domain.CourseStatusPublished)
	published := &domain.Course{ID: courseID, StableID: courseStableID, AuthorID: authorID, Status: domain.CourseStatusPublished}

	if _, err := svc.Enroll(context.Background(), student(), courseID); err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if allowed, _ := svc.CanRead(context.Background(), studentID, domain.RoleStudent, published); !allowed {
		t.Fatal("an enrolled student should be able to read")
	}

	if _, err := svc.Withdraw(context.Background(), student(), courseID); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if allowed, _ := svc.CanRead(context.Background(), studentID, domain.RoleStudent, published); allowed {
		t.Error("a withdrawn student can still read the content")
	}
}

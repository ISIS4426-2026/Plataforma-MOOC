package course_test

import (
	"context"
	"sync"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// fakeCourseRepo is an in-memory stand-in for domain.CourseRepository, the
// same style internal/auth uses for its service tests: fast, no Postgres
// required, and just enough behavior to exercise the service's own logic
// rather than the database's.
type fakeCourseRepo struct {
	mu      sync.Mutex
	courses map[string]*domain.Course
	entries []*domain.AuditEntry
}

func newFakeCourseRepo() *fakeCourseRepo {
	return &fakeCourseRepo{courses: make(map[string]*domain.Course)}
}

func (f *fakeCourseRepo) Create(_ context.Context, c *domain.Course, entry *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt

	stored := *c
	f.courses[c.ID] = &stored
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeCourseRepo) GetByID(_ context.Context, id string) (*domain.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.courses[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *c
	return &copied, nil
}

func (f *fakeCourseRepo) GetByStableIDAndVersion(_ context.Context, stableID string, version int) (*domain.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range f.courses {
		if c.StableID == stableID && c.Version == version {
			copied := *c
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeCourseRepo) ListPublished(_ context.Context, _ domain.CourseFilter) (*domain.CoursePage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var page domain.CoursePage
	for _, c := range f.courses {
		if c.Status == domain.CourseStatusPublished {
			copied := *c
			page.Courses = append(page.Courses, &copied)
		}
	}
	return &page, nil
}

func (f *fakeCourseRepo) Update(_ context.Context, courseID string, title, description string, opts domain.ChangeOptions, entry *domain.AuditEntry) (*domain.Course, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	current, ok := f.courses[courseID]
	if !ok {
		return nil, domain.ErrNotFound
	}

	if opts.ExpectedETag != "" && opts.ExpectedETag != current.ETag() {
		return nil, domain.ErrPreconditionFailed
	}
	if current.Status == domain.CourseStatusPublished {
		return nil, domain.ErrCourseImmutable
	}

	current.Title = title
	current.Description = description
	current.UpdatedAt = current.UpdatedAt.Add(time.Second)

	f.entries = append(f.entries, entry)

	copied := *current
	return &copied, nil
}

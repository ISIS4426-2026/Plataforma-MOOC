package structure_test

import (
	"context"
	"sync"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// The fakes below mirror internal/course's style: in-memory, just enough
// behavior to exercise the service's own role/ownership/validation logic
// without a database.

type fakeCourseRepo struct {
	mu      sync.Mutex
	courses map[string]*domain.Course
}

func newFakeCourseRepo() *fakeCourseRepo {
	return &fakeCourseRepo{courses: make(map[string]*domain.Course)}
}

func (f *fakeCourseRepo) put(c *domain.Course) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.courses[c.ID] = c
}

func (f *fakeCourseRepo) Create(context.Context, *domain.Course, *domain.AuditEntry) error {
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

func (f *fakeCourseRepo) GetByStableIDAndVersion(context.Context, string, int) (*domain.Course, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeCourseRepo) ListPublished(context.Context, domain.CourseFilter) (*domain.CoursePage, error) {
	return &domain.CoursePage{}, nil
}
func (f *fakeCourseRepo) Update(context.Context, string, string, string, domain.ChangeOptions, *domain.AuditEntry) (*domain.Course, error) {
	return nil, nil
}

type fakeModuleRepo struct {
	mu      sync.Mutex
	modules map[string]*domain.Module
	entries []*domain.AuditEntry
}

func newFakeModuleRepo() *fakeModuleRepo {
	return &fakeModuleRepo{modules: make(map[string]*domain.Module)}
}

func (f *fakeModuleRepo) Create(_ context.Context, m *domain.Module, entry *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := *m
	f.modules[m.ID] = &stored
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeModuleRepo) GetByID(_ context.Context, id string) (*domain.Module, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.modules[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *m
	return &copied, nil
}

func (f *fakeModuleRepo) ListByCourse(_ context.Context, courseID string) ([]*domain.Module, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Module
	for _, m := range f.modules {
		if m.CourseID == courseID {
			copied := *m
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (f *fakeModuleRepo) Update(_ context.Context, moduleID, title string, entry *domain.AuditEntry) (*domain.Module, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.modules[moduleID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	m.Title = title
	f.entries = append(f.entries, entry)
	copied := *m
	return &copied, nil
}

func (f *fakeModuleRepo) Delete(_ context.Context, moduleID string, entry *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.modules[moduleID]; !ok {
		return domain.ErrNotFound
	}
	delete(f.modules, moduleID)
	f.entries = append(f.entries, entry)
	return nil
}

type fakeUnitRepo struct {
	mu    sync.Mutex
	units map[string]*domain.Unit
}

func newFakeUnitRepo() *fakeUnitRepo { return &fakeUnitRepo{units: make(map[string]*domain.Unit)} }

func (f *fakeUnitRepo) Create(_ context.Context, u *domain.Unit, _ *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := *u
	f.units[u.ID] = &stored
	return nil
}

func (f *fakeUnitRepo) GetByID(_ context.Context, id string) (*domain.Unit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.units[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *u
	return &copied, nil
}

func (f *fakeUnitRepo) ListByModule(_ context.Context, moduleID string) ([]*domain.Unit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Unit
	for _, u := range f.units {
		if u.ModuleID == moduleID {
			copied := *u
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (f *fakeUnitRepo) Update(_ context.Context, unitID, title string, _ *domain.AuditEntry) (*domain.Unit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.units[unitID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	u.Title = title
	copied := *u
	return &copied, nil
}

func (f *fakeUnitRepo) Delete(_ context.Context, unitID string, _ *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.units[unitID]; !ok {
		return domain.ErrNotFound
	}
	delete(f.units, unitID)
	return nil
}

type fakeResourceRepo struct {
	mu        sync.Mutex
	resources map[string]*domain.Resource
}

func newFakeResourceRepo() *fakeResourceRepo {
	return &fakeResourceRepo{resources: make(map[string]*domain.Resource)}
}

func (f *fakeResourceRepo) Create(_ context.Context, r *domain.Resource, _ *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := *r
	f.resources[r.ID] = &stored
	return nil
}

func (f *fakeResourceRepo) GetByID(_ context.Context, id string) (*domain.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.resources[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *r
	return &copied, nil
}

func (f *fakeResourceRepo) ListByUnit(_ context.Context, unitID string) ([]*domain.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Resource
	for _, r := range f.resources {
		if r.UnitID == unitID {
			copied := *r
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (f *fakeResourceRepo) Update(_ context.Context, resourceID string, fields domain.ResourceUpdate, _ *domain.AuditEntry) (*domain.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.resources[resourceID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	r.Title = fields.Title
	r.IsVisible = fields.IsVisible
	r.IsMandatory = fields.IsMandatory
	r.AllowDownload = fields.AllowDownload
	copied := *r
	return &copied, nil
}

func (f *fakeResourceRepo) Delete(_ context.Context, resourceID string, _ *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.resources[resourceID]; !ok {
		return domain.ErrNotFound
	}
	delete(f.resources, resourceID)
	return nil
}

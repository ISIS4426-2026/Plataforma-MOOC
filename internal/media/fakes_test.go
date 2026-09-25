package media

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// The doubles below are only as wide as the ports the service declares, which
// is the point of declaring them narrowly in the first place.

type fakeCourses struct{ course *domain.Course }

func (f *fakeCourses) GetByID(context.Context, string) (*domain.Course, error) {
	if f.course == nil {
		return nil, domain.ErrNotFound
	}
	return f.course, nil
}

type fakeModules struct{ module *domain.Module }

func (f *fakeModules) GetByID(context.Context, string) (*domain.Module, error) {
	if f.module == nil {
		return nil, domain.ErrNotFound
	}
	return f.module, nil
}

type fakeUnits struct{ unit *domain.Unit }

func (f *fakeUnits) GetByID(context.Context, string) (*domain.Unit, error) {
	if f.unit == nil {
		return nil, domain.ErrNotFound
	}
	return f.unit, nil
}

type fakeResources struct {
	mu        sync.Mutex
	resource  *domain.Resource
	attached  []string // object keys AttachMedia was called with
	attachErr error
}

func (f *fakeResources) GetByID(context.Context, string) (*domain.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resource == nil {
		return nil, domain.ErrNotFound
	}
	copied := *f.resource
	return &copied, nil
}

func (f *fakeResources) AttachMedia(_ context.Context, _, objectKey string, _ *domain.AuditEntry) (*domain.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attachErr != nil {
		return nil, f.attachErr
	}
	f.attached = append(f.attached, objectKey)
	f.resource.ObjectKey = objectKey
	f.resource.ProcessingStatus = string(domain.ResourceProcessingPending)
	copied := *f.resource
	return &copied, nil
}

func (f *fakeResources) attachedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.attached...)
}

type fakeStorage struct {
	mu sync.Mutex

	// objects is what the bucket "holds"; a key absent from it makes StatObject
	// report domain.ErrObjectNotFound, which is how a never-completed transfer
	// is simulated.
	objects map[string]*domain.ObjectInfo

	signedUploads   []string
	signedDownloads []string
	signErr         error
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: map[string]*domain.ObjectInfo{}}
}

func (f *fakeStorage) put(key, contentType string, size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = &domain.ObjectInfo{Key: key, SizeBytes: size, ContentType: contentType}
}

func (f *fakeStorage) GeneratePresignedUploadURL(_ context.Context, objectKey, _ string, _ time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.signErr != nil {
		return "", f.signErr
	}
	f.signedUploads = append(f.signedUploads, objectKey)
	return "https://bucket.example/" + objectKey + "?signature=test", nil
}

func (f *fakeStorage) GeneratePresignedDownloadURL(_ context.Context, objectKey string, _ time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.signErr != nil {
		return "", f.signErr
	}
	f.signedDownloads = append(f.signedDownloads, objectKey)
	return "https://bucket.example/" + objectKey + "?signature=read", nil
}

func (f *fakeStorage) StatObject(_ context.Context, objectKey string) (*domain.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info, ok := f.objects[objectKey]
	if !ok {
		return nil, domain.ErrObjectNotFound
	}
	return info, nil
}

// GetObject and PutObject exist so the double still satisfies the storage port;
// the media service never calls them -- pulling bytes through the API is what
// the direct-upload design avoids, and only the worker does it.
func (f *fakeStorage) GetObject(_ context.Context, objectKey string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objects[objectKey]; !ok {
		return nil, domain.ErrObjectNotFound
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeStorage) PutObject(_ context.Context, objectKey, contentType string, r io.Reader, _ int64) error {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[objectKey] = &domain.ObjectInfo{Key: objectKey, SizeBytes: n, ContentType: contentType}
	return nil
}

func (f *fakeStorage) DeleteObject(_ context.Context, objectKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, objectKey)
	return nil
}

type enqueuedJob struct {
	resourceID     string
	taskType       string
	objectKey      string
	idempotencyKey string
}

type fakeQueue struct {
	mu   sync.Mutex
	jobs []enqueuedJob
	err  error
}

func (f *fakeQueue) EnqueueMediaProcessJob(_ context.Context, resourceID, taskType, objectKey, idempotencyKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.jobs = append(f.jobs, enqueuedJob{resourceID, taskType, objectKey, idempotencyKey})
	return nil
}

func (f *fakeQueue) enqueued() []enqueuedJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]enqueuedJob(nil), f.jobs...)
}

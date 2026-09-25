// Package media owns the direct-upload flow: the API authorizes a transfer and
// signs a URL, the client moves the bytes straight to the bucket, and a second
// call confirms the object landed and hands it to the processing queue.
//
// The bytes never pass through the API. That is what keeps a 500 MB lecture off
// the web server's memory, and it is the split the delivery's capacity scenario
// measures: control traffic here, file transfer against the bucket.
package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
)

// Task types the worker dispatches on, mirroring task.MediaProcessPayload.
const (
	TaskHLSTranscode = "hls_transcode"
	TaskMalwareScan  = "malware_scan"
)

// Enqueuer is the slice of the task queue this service needs. It is declared
// here, in terms of plain strings, so the service never imports asynq.
type Enqueuer interface {
	EnqueueMediaProcessJob(ctx context.Context, resourceID, taskType, objectKey, idempotencyKey string) error
}

// The service walks resource -> unit -> module -> course to find the owner, and
// writes one field on the resource. These are the four ports that covers.
//
// They are declared here rather than taking the full domain repositories
// because a consumer should ask for what it calls: the real repositories
// satisfy them as they are, and a test double is four methods instead of
// twenty.
type (
	// CourseLookup reads the course that owns a resource, for the ownership check.
	CourseLookup interface {
		GetByID(ctx context.Context, id string) (*domain.Course, error)
	}
	// ModuleLookup reads a module on the way up the hierarchy.
	ModuleLookup interface {
		GetByID(ctx context.Context, id string) (*domain.Module, error)
	}
	// UnitLookup reads a unit on the way up the hierarchy.
	UnitLookup interface {
		GetByID(ctx context.Context, id string) (*domain.Unit, error)
	}
	// ResourceStore reads the resource and records a confirmed upload on it.
	// Notably absent is Update: this service never touches the client-editable
	// fields.
	ResourceStore interface {
		GetByID(ctx context.Context, id string) (*domain.Resource, error)
		AttachMedia(ctx context.Context, resourceID, objectKey string, entry *domain.AuditEntry) (*domain.Resource, error)
	}
)

// Deps are the ports the service depends on.
type Deps struct {
	Courses   CourseLookup
	Modules   ModuleLookup
	Units     UnitLookup
	Resources ResourceStore
	Storage   domain.StorageProvider
	Queue     Enqueuer
}

// Options tune the issued URLs and the size ceiling.
type Options struct {
	// UploadURLTTL is how long a signed upload URL stays valid. The
	// specification asks for a resumable upload valid for 24 hours, so that is
	// the default.
	UploadURLTTL time.Duration
	// DownloadURLTTL is deliberately much shorter: a read URL that leaks is a
	// copy of the content.
	DownloadURLTTL time.Duration
	// MaxUploadBytes bounds what we will authorize. The bucket cannot enforce a
	// size limit on a signed PUT, so this is a declared-size check, not a
	// guarantee -- the confirmation re-checks against what actually landed.
	MaxUploadBytes int64
}

// Actor is who is performing the action, matching the shape the course and
// structure services use.
type Actor struct {
	ID        string
	Role      domain.Role
	IPAddress string
	UserAgent string
}

type Service struct {
	deps   Deps
	opts   Options
	logger *slog.Logger
}

func NewService(deps Deps, opts Options, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.UploadURLTTL <= 0 {
		opts.UploadURLTTL = 24 * time.Hour
	}
	if opts.DownloadURLTTL <= 0 {
		opts.DownloadURLTTL = time.Hour
	}
	if opts.MaxUploadBytes <= 0 {
		opts.MaxUploadBytes = 1 << 30 // 1 GiB
	}
	return &Service{deps: deps, opts: opts, logger: logger}
}

// UploadRequest is what a caller supplies to start an upload.
type UploadRequest struct {
	ResourceID string
	Filename   string
	MimeType   string
	SizeBytes  int64
}

// UploadTicket is everything the client needs to perform the transfer itself.
type UploadTicket struct {
	UploadURL   string
	ObjectKey   string
	Method      string
	ContentType string
	ExpiresAt   time.Time
}

// AuthorizeUpload validates the request and signs a URL the client PUTs to.
//
// Nothing is written to the resource here. The object does not exist yet, so
// recording a key would leave a row pointing at nothing every time a client
// asks for a URL and never uses it. The resource is touched at confirmation.
func (s *Service) AuthorizeUpload(ctx context.Context, actor Actor, req UploadRequest) (*UploadTicket, error) {
	resource, err := s.deps.Resources.GetByID(ctx, req.ResourceID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnResource(ctx, actor, resource); err != nil {
		return nil, err
	}

	expectedArea, ok := areaForResourceType(resource.Type)
	if !ok {
		return nil, fmt.Errorf("resource type %q carries no file: %w", resource.Type, domain.ErrInvalidInput)
	}

	ext, area, ok := storage.NormalizeExtension(req.Filename)
	if !ok {
		return nil, fmt.Errorf("unsupported file extension %q: %w", ext, domain.ErrInvalidInput)
	}
	if area != expectedArea {
		return nil, fmt.Errorf("a %s resource does not accept a %q file: %w", resource.Type, ext, domain.ErrInvalidInput)
	}

	canonicalMIME, _ := storage.CanonicalMIME(ext)
	if !strings.EqualFold(strings.TrimSpace(req.MimeType), canonicalMIME) {
		return nil, fmt.Errorf("declared mime type %q does not match %q for %s: %w",
			req.MimeType, canonicalMIME, ext, domain.ErrInvalidInput)
	}

	if req.SizeBytes <= 0 {
		return nil, fmt.Errorf("file size must be positive: %w", domain.ErrInvalidInput)
	}
	if req.SizeBytes > s.opts.MaxUploadBytes {
		return nil, fmt.Errorf("file of %d bytes exceeds the %d byte limit: %w",
			req.SizeBytes, s.opts.MaxUploadBytes, domain.ErrInvalidInput)
	}

	objectKey, err := storage.OriginalKey(resource.StableID, req.Filename)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", err, domain.ErrInvalidInput)
	}

	url, err := s.deps.Storage.GeneratePresignedUploadURL(ctx, objectKey, canonicalMIME, s.opts.UploadURLTTL)
	if err != nil {
		return nil, err
	}

	return &UploadTicket{
		UploadURL:   url,
		ObjectKey:   objectKey,
		Method:      "PUT",
		ContentType: canonicalMIME,
		ExpiresAt:   time.Now().UTC().Add(s.opts.UploadURLTTL),
	}, nil
}

// ConfirmUpload checks the bytes actually landed, records the key and hands the
// resource to the worker.
//
// The client's word that the transfer finished is not evidence. StatObject is:
// it is the difference between a resource that points at a real object and one
// that points at a key nobody ever wrote.
func (s *Service) ConfirmUpload(ctx context.Context, actor Actor, resourceID, objectKey string) (*domain.Resource, error) {
	resource, err := s.deps.Resources.GetByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOnResource(ctx, actor, resource); err != nil {
		return nil, err
	}

	// The key comes back from the client, so it has to be one we could have
	// issued for this resource -- otherwise a caller could point their resource
	// at an object belonging to someone else's.
	if !storage.KeyBelongsToResource(objectKey, resource.StableID) {
		return nil, fmt.Errorf("object key %q was not issued for this resource: %w", objectKey, domain.ErrInvalidInput)
	}

	// This upload has already been through the pipeline. Re-attaching would send
	// processing_status back to pending, and the queue would then drop the job as
	// a duplicate -- leaving the resource stuck at pending forever with its
	// derivatives sitting in the bucket, produced and unreachable.
	//
	// So a repeated confirmation of a finished upload answers with the resource
	// as it stands. A re-upload is a different object key and does not take this
	// path.
	if resource.ProcessingStatus == string(domain.ResourceProcessingCompleted) &&
		resource.ObjectKey == objectKey {
		s.logger.InfoContext(ctx, "upload already confirmed and processed; nothing to do",
			slog.String("resource_id", resourceID),
			slog.String("object_key", objectKey),
		)
		return resource, nil
	}

	info, err := s.deps.Storage.StatObject(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	if info.SizeBytes <= 0 {
		return nil, fmt.Errorf("object %q is empty: %w", objectKey, domain.ErrInvalidInput)
	}
	if info.SizeBytes > s.opts.MaxUploadBytes {
		return nil, fmt.Errorf("object %q is %d bytes, over the %d byte limit: %w",
			objectKey, info.SizeBytes, s.opts.MaxUploadBytes, domain.ErrInvalidInput)
	}

	// The content type the bucket reports comes from the signed PUT, so a
	// mismatch means the transfer did not use the URL we issued.
	if ext, _, ok := storage.NormalizeExtension(objectKey); ok {
		if want, ok := storage.CanonicalMIME(ext); ok && info.ContentType != "" &&
			!strings.EqualFold(info.ContentType, want) {
			return nil, fmt.Errorf("object %q was stored as %q, expected %q: %w",
				objectKey, info.ContentType, want, domain.ErrInvalidInput)
		}
	}

	entry := newEntry(actor, domain.AuditActionResourceMediaAttached, "resource:"+resourceID)
	updated, err := s.deps.Resources.AttachMedia(ctx, resourceID, objectKey, entry)
	if err != nil {
		return nil, err
	}

	// The idempotency key is the object key, not the resource id: re-confirming
	// the same upload must not produce a second job, but a genuine re-upload
	// lands on a fresh key and does deserve one.
	taskType := taskTypeForResource(resource.Type)
	switch err := s.deps.Queue.EnqueueMediaProcessJob(ctx, resourceID, taskType, objectKey, "media-"+objectKey); {
	case err == nil:
	case errors.Is(err, domain.ErrTaskAlreadyQueued):
		// The retry found the job already there, which is the whole point of
		// deriving the key from the object. Treating the queue's refusal to
		// write a duplicate as a failure would turn a correct idempotent retry
		// into a 500.
		s.logger.InfoContext(ctx, "media upload re-confirmed; job was already queued",
			slog.String("resource_id", resourceID),
			slog.String("object_key", objectKey),
		)
	default:
		// The row already points at a real object; losing the enqueue is
		// recoverable by re-confirming, so say what happened instead of
		// pretending the upload failed.
		s.logger.ErrorContext(ctx, "media upload confirmed but enqueue failed",
			slog.String("resource_id", resourceID),
			slog.String("object_key", objectKey),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("upload recorded but could not be queued for processing: %w", err)
	}

	s.logger.InfoContext(ctx, "media upload confirmed and queued",
		slog.String("resource_id", resourceID),
		slog.String("object_key", objectKey),
		slog.String("task_type", taskType),
		slog.Int64("size_bytes", info.SizeBytes),
	)
	return updated, nil
}

// DownloadURL issues a short-lived read URL for a resource's stored object.
func (s *Service) DownloadURL(ctx context.Context, actor Actor, resourceID string) (string, time.Time, error) {
	resource, err := s.deps.Resources.GetByID(ctx, resourceID)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := s.authorizeOnResource(ctx, actor, resource); err != nil {
		return "", time.Time{}, err
	}
	if resource.ObjectKey == "" {
		return "", time.Time{}, fmt.Errorf("resource %s carries no stored object: %w", resourceID, domain.ErrNotFound)
	}

	url, err := s.deps.Storage.GeneratePresignedDownloadURL(ctx, resource.ObjectKey, s.opts.DownloadURLTTL)
	if err != nil {
		return "", time.Time{}, err
	}
	return url, time.Now().UTC().Add(s.opts.DownloadURLTTL), nil
}

// ---- shared -----------------------------------------------------------

// authorizeOnResource applies the same rule the authoring services do: only the
// course's author, or an administrator, may act on its content. Defence in
// depth -- the routes are already restricted by middleware.RequireRole.
func (s *Service) authorizeOnResource(ctx context.Context, actor Actor, resource *domain.Resource) error {
	if actor.Role != domain.RoleProfessor && actor.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	unit, err := s.deps.Units.GetByID(ctx, resource.UnitID)
	if err != nil {
		return err
	}
	module, err := s.deps.Modules.GetByID(ctx, unit.ModuleID)
	if err != nil {
		return err
	}
	course, err := s.deps.Courses.GetByID(ctx, module.CourseID)
	if err != nil {
		return err
	}
	if actor.Role != domain.RoleAdmin && course.AuthorID != actor.ID {
		return domain.ErrForbidden
	}
	return nil
}

// areaForResourceType maps a resource type to the bucket area its file belongs
// in. Types that carry no file at all -- text, iframe, external link, quiz --
// are absent, which is what makes "this resource does not take an upload" a
// validation result rather than a silent success.
func areaForResourceType(t domain.ResourceType) (string, bool) {
	switch t {
	case domain.ResourceTypeVideo, domain.ResourceTypeAudio:
		return storage.PrefixOriginals, true
	case domain.ResourceTypePDF, domain.ResourceTypePresentation, domain.ResourceTypeDownloadable:
		return storage.PrefixDocuments, true
	case domain.ResourceTypeImage:
		return storage.PrefixThumbnails, true
	default:
		return "", false
	}
}

// taskTypeForResource picks the job the worker will run. Video and audio go to
// transcoding; everything else only needs the scan.
func taskTypeForResource(t domain.ResourceType) string {
	switch t {
	case domain.ResourceTypeVideo, domain.ResourceTypeAudio:
		return TaskHLSTranscode
	default:
		return TaskMalwareScan
	}
}

func newEntry(actor Actor, action domain.AuditAction, targetResource string) *domain.AuditEntry {
	actorID := actor.ID
	return &domain.AuditEntry{
		ActorID:        &actorID,
		Action:         action,
		TargetResource: targetResource,
		IPAddress:      actor.IPAddress,
		UserAgent:      actor.UserAgent,
	}
}

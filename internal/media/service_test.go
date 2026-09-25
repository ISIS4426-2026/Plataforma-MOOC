package media

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
)

const (
	authorID         = "11111111-1111-1111-1111-111111111111"
	otherProfessorID = "22222222-2222-2222-2222-222222222222"
	resourceID       = "33333333-3333-3333-3333-333333333333"
	resourceStableID = "44444444-4444-4444-4444-444444444444"
)

type harness struct {
	svc       *Service
	resources *fakeResources
	store     *fakeStorage
	queue     *fakeQueue
}

func newHarness(t *testing.T, resourceType domain.ResourceType) *harness {
	t.Helper()

	resources := &fakeResources{resource: &domain.Resource{
		ID: resourceID, StableID: resourceStableID, UnitID: "unit-1",
		Title: "Clase 1", Type: resourceType,
		ProcessingStatus: string(domain.ResourceProcessingCompleted),
	}}
	store := newFakeStorage()
	queue := &fakeQueue{}

	svc := NewService(Deps{
		Courses:   &fakeCourses{course: &domain.Course{ID: "course-1", AuthorID: authorID}},
		Modules:   &fakeModules{module: &domain.Module{ID: "module-1", CourseID: "course-1"}},
		Units:     &fakeUnits{unit: &domain.Unit{ID: "unit-1", ModuleID: "module-1"}},
		Resources: resources,
		Storage:   store,
		Queue:     queue,
	}, Options{MaxUploadBytes: 10 << 20}, nil)

	return &harness{svc: svc, resources: resources, store: store, queue: queue}
}

func author() Actor {
	return Actor{ID: authorID, Role: domain.RoleProfessor}
}

// ---- AuthorizeUpload ---------------------------------------------------

func TestAuthorizeUploadIssuesATicket(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	ticket, err := h.svc.AuthorizeUpload(context.Background(), author(), UploadRequest{
		ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}

	if want := storage.PrefixOriginals + "/" + resourceStableID + "/"; !strings.HasPrefix(ticket.ObjectKey, want) {
		t.Errorf("object key %q, want prefix %q", ticket.ObjectKey, want)
	}
	if ticket.ContentType != "video/mp4" {
		t.Errorf("content type = %q, want video/mp4", ticket.ContentType)
	}
	if ticket.Method != "PUT" {
		t.Errorf("method = %q, want PUT", ticket.Method)
	}
	if ticket.UploadURL == "" {
		t.Error("expected a signed upload URL")
	}
	if ticket.ExpiresAt.IsZero() {
		t.Error("expected an expiry")
	}
}

// Authorizing is not a write. A client that asks for a URL and walks away must
// not leave the resource pointing at an object nobody ever uploaded.
func TestAuthorizeUploadDoesNotTouchTheResource(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	if _, err := h.svc.AuthorizeUpload(context.Background(), author(), UploadRequest{
		ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1 << 20,
	}); err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}

	if got := h.resources.attachedKeys(); len(got) != 0 {
		t.Errorf("AttachMedia was called %d times, want 0", len(got))
	}
	if got := h.queue.enqueued(); len(got) != 0 {
		t.Errorf("%d jobs enqueued, want 0", len(got))
	}
}

func TestAuthorizeUploadRejections(t *testing.T) {
	cases := []struct {
		name         string
		resourceType domain.ResourceType
		req          UploadRequest
	}{
		{
			"a text resource carries no file",
			domain.ResourceTypeRichText,
			UploadRequest{ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1024},
		},
		{
			"a pdf does not belong on a video resource",
			domain.ResourceTypeVideo,
			UploadRequest{ResourceID: resourceID, Filename: "apunte.pdf", MimeType: "application/pdf", SizeBytes: 1024},
		},
		{
			"unsupported extension",
			domain.ResourceTypeVideo,
			UploadRequest{ResourceID: resourceID, Filename: "payload.exe", MimeType: "video/mp4", SizeBytes: 1024},
		},
		{
			"declared mime does not match the extension",
			domain.ResourceTypeVideo,
			UploadRequest{ResourceID: resourceID, Filename: "clase.mp4", MimeType: "text/html", SizeBytes: 1024},
		},
		{
			"zero size",
			domain.ResourceTypeVideo,
			UploadRequest{ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 0},
		},
		{
			"over the size ceiling",
			domain.ResourceTypeVideo,
			UploadRequest{ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 11 << 20},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.resourceType)
			_, err := h.svc.AuthorizeUpload(context.Background(), author(), tc.req)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
			if got := len(h.store.signedUploads); got != 0 {
				t.Errorf("%d URLs signed, want 0", got)
			}
		})
	}
}

func TestAuthorizeUploadEnforcesOwnership(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	cases := []struct {
		name  string
		actor Actor
	}{
		{"another professor", Actor{ID: otherProfessorID, Role: domain.RoleProfessor}},
		{"a student", Actor{ID: authorID, Role: domain.RoleStudent}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.svc.AuthorizeUpload(context.Background(), tc.actor, UploadRequest{
				ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1024,
			})
			if !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	}
}

func TestAuthorizeUploadAllowsAnAdministrator(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	if _, err := h.svc.AuthorizeUpload(context.Background(),
		Actor{ID: otherProfessorID, Role: domain.RoleAdmin},
		UploadRequest{ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1024},
	); err != nil {
		t.Fatalf("an administrator should be allowed: %v", err)
	}
}

// ---- ConfirmUpload -----------------------------------------------------

// uploadAndLand runs the authorize step and then simulates the client's
// transfer actually completing.
func uploadAndLand(t *testing.T, h *harness, filename, mime string, size int64) string {
	t.Helper()
	ticket, err := h.svc.AuthorizeUpload(context.Background(), author(), UploadRequest{
		ResourceID: resourceID, Filename: filename, MimeType: mime, SizeBytes: size,
	})
	if err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}
	h.store.put(ticket.ObjectKey, mime, size)
	return ticket.ObjectKey
}

func TestConfirmUploadAttachesAndQueues(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	key := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)

	updated, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key)
	if err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	if updated.ObjectKey != key {
		t.Errorf("object key = %q, want %q", updated.ObjectKey, key)
	}
	if got, want := updated.ProcessingStatus, string(domain.ResourceProcessingPending); got != want {
		t.Errorf("processing status = %q, want %q", got, want)
	}

	jobs := h.queue.enqueued()
	if len(jobs) != 1 {
		t.Fatalf("%d jobs enqueued, want 1", len(jobs))
	}
	if jobs[0].taskType != TaskHLSTranscode {
		t.Errorf("task type = %q, want %q", jobs[0].taskType, TaskHLSTranscode)
	}
	if jobs[0].objectKey != key {
		t.Errorf("job object key = %q, want %q", jobs[0].objectKey, key)
	}
}

// The idempotency key is derived from the object key so a retried confirmation
// deduplicates, while a genuine re-upload -- which lands on a fresh key -- still
// gets its own job. This is the "entrega duplicada no genera salidas repetidas"
// condition, enforced at the point where the job is named.
func TestConfirmUploadDerivesIdempotencyKeyFromTheObject(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	key := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)

	for i := 0; i < 2; i++ {
		if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key); err != nil {
			t.Fatalf("ConfirmUpload #%d: %v", i+1, err)
		}
	}

	jobs := h.queue.enqueued()
	if len(jobs) != 2 {
		t.Fatalf("%d jobs seen by the queue, want 2", len(jobs))
	}
	if jobs[0].idempotencyKey != jobs[1].idempotencyKey {
		t.Errorf("two confirmations of the same object produced different idempotency keys: %q and %q",
			jobs[0].idempotencyKey, jobs[1].idempotencyKey)
	}
	if !strings.Contains(jobs[0].idempotencyKey, key) {
		t.Errorf("idempotency key %q does not carry the object key %q", jobs[0].idempotencyKey, key)
	}

	// A second, genuine upload lands on a different key and must be a new job.
	other := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)
	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, other); err != nil {
		t.Fatalf("ConfirmUpload after re-upload: %v", err)
	}
	jobs = h.queue.enqueued()
	if jobs[2].idempotencyKey == jobs[0].idempotencyKey {
		t.Error("a genuine re-upload reused the previous idempotency key and would be deduplicated away")
	}
}

// The client hands the key back, so it has to be one we could have issued for
// this resource. Otherwise a professor could point their own resource at an
// object belonging to somebody else's course.
func TestConfirmUploadRejectsAForeignKey(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	foreign := storage.PrefixOriginals + "/99999999-9999-9999-9999-999999999999/file.mp4"
	h.store.put(foreign, "video/mp4", 1024)

	_, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, foreign)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if got := h.resources.attachedKeys(); len(got) != 0 {
		t.Errorf("AttachMedia was called for a foreign key: %v", got)
	}
}

// "I finished uploading" is a claim, not evidence.
func TestConfirmUploadRejectsAnUploadThatNeverLanded(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	ticket, err := h.svc.AuthorizeUpload(context.Background(), author(), UploadRequest{
		ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}

	// No h.store.put: the transfer never happened.
	_, err = h.svc.ConfirmUpload(context.Background(), author(), resourceID, ticket.ObjectKey)
	if !errors.Is(err, domain.ErrObjectNotFound) {
		t.Fatalf("error = %v, want ErrObjectNotFound", err)
	}
	if got := h.queue.enqueued(); len(got) != 0 {
		t.Errorf("%d jobs enqueued for an absent object, want 0", len(got))
	}
}

// The stored content type comes from the signed PUT, so a mismatch means the
// transfer did not use the URL we issued.
func TestConfirmUploadRejectsAMismatchedContentType(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	ticket, err := h.svc.AuthorizeUpload(context.Background(), author(), UploadRequest{
		ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}
	h.store.put(ticket.ObjectKey, "text/html", 1<<20)

	_, err = h.svc.ConfirmUpload(context.Background(), author(), resourceID, ticket.ObjectKey)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestConfirmUploadRejectsAnEmptyObject(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	ticket, err := h.svc.AuthorizeUpload(context.Background(), author(), UploadRequest{
		ResourceID: resourceID, Filename: "clase.mp4", MimeType: "video/mp4", SizeBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}
	h.store.put(ticket.ObjectKey, "video/mp4", 0)

	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, ticket.ObjectKey); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

// A document only needs scanning; transcoding is for video and audio.
func TestConfirmUploadPicksTheTaskTypeByResourceType(t *testing.T) {
	h := newHarness(t, domain.ResourceTypePDF)
	key := uploadAndLand(t, h, "apunte.pdf", "application/pdf", 4096)

	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	jobs := h.queue.enqueued()
	if len(jobs) != 1 {
		t.Fatalf("%d jobs enqueued, want 1", len(jobs))
	}
	if jobs[0].taskType != TaskMalwareScan {
		t.Errorf("task type = %q, want %q", jobs[0].taskType, TaskMalwareScan)
	}
}

// ---- DownloadURL -------------------------------------------------------

func TestDownloadURLRequiresAStoredObject(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)

	if _, _, err := h.svc.DownloadURL(context.Background(), author(), resourceID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestDownloadURLSignsTheStoredObject(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	key := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)
	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	url, expiresAt, err := h.svc.DownloadURL(context.Background(), author(), resourceID)
	if err != nil {
		t.Fatalf("DownloadURL: %v", err)
	}
	if !strings.Contains(url, key) {
		t.Errorf("download URL %q does not reference %q", url, key)
	}
	if expiresAt.IsZero() {
		t.Error("expected an expiry")
	}
}

// The queue refuses to write a second task with the same id. That refusal is
// what idempotency looks like from the inside, so it must not surface as a
// failure -- the caller asked for the job to exist, and it does.
func TestConfirmUploadTreatsAnAlreadyQueuedJobAsSuccess(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	key := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)

	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key); err != nil {
		t.Fatalf("first ConfirmUpload: %v", err)
	}

	h.queue.err = domain.ErrTaskAlreadyQueued

	updated, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key)
	if err != nil {
		t.Fatalf("re-confirming an already queued upload should succeed, got: %v", err)
	}
	if updated.ObjectKey != key {
		t.Errorf("object key = %q, want %q", updated.ObjectKey, key)
	}
}

// Any other enqueue failure is still an error: the object landed but nothing
// will ever process it, and silently reporting success would hide that.
func TestConfirmUploadReportsARealEnqueueFailure(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	key := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)

	h.queue.err = errors.New("redis is unreachable")

	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key); err == nil {
		t.Fatal("expected a real enqueue failure to be reported")
	}
}

// Regression: confirming again after the worker has finished must not send the
// resource back to pending. The queue would drop the re-enqueued job as a
// duplicate, leaving the resource stuck at pending forever while its
// derivatives sit in the bucket, produced and unreachable.
func TestConfirmUploadDoesNotUndoAFinishedUpload(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	key := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)

	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key); err != nil {
		t.Fatalf("first ConfirmUpload: %v", err)
	}

	// The worker finishes and reports the outcome.
	h.resources.resource.ProcessingStatus = string(domain.ResourceProcessingCompleted)

	updated, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, key)
	if err != nil {
		t.Fatalf("re-confirming a finished upload: %v", err)
	}
	if got, want := updated.ProcessingStatus, string(domain.ResourceProcessingCompleted); got != want {
		t.Errorf("processing status = %q, want %q -- the resource was sent back to pending", got, want)
	}
	if got := len(h.resources.attachedKeys()); got != 1 {
		t.Errorf("AttachMedia ran %d times, want 1; the second confirmation rewrote the resource", got)
	}
	if got := len(h.queue.enqueued()); got != 1 {
		t.Errorf("%d jobs enqueued, want 1", got)
	}
}

// A genuine re-upload lands on a new key and must still be processed, even
// though the resource is currently completed.
func TestConfirmUploadAcceptsANewObjectOnACompletedResource(t *testing.T) {
	h := newHarness(t, domain.ResourceTypeVideo)
	first := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)
	if _, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, first); err != nil {
		t.Fatalf("first ConfirmUpload: %v", err)
	}
	h.resources.resource.ProcessingStatus = string(domain.ResourceProcessingCompleted)

	second := uploadAndLand(t, h, "clase.mp4", "video/mp4", 2<<20)
	updated, err := h.svc.ConfirmUpload(context.Background(), author(), resourceID, second)
	if err != nil {
		t.Fatalf("confirming a re-upload: %v", err)
	}
	if got, want := updated.ProcessingStatus, string(domain.ResourceProcessingPending); got != want {
		t.Errorf("processing status = %q, want %q", got, want)
	}
	if got := len(h.queue.enqueued()); got != 2 {
		t.Errorf("%d jobs enqueued, want 2", got)
	}
}

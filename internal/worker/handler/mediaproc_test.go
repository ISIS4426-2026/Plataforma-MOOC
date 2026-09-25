package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/transcode"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

const (
	resourceID  = "11111111-1111-1111-1111-111111111111"
	stableID    = "22222222-2222-2222-2222-222222222222"
	originalKey = "originals/22222222-2222-2222-2222-222222222222/abc.mp4"
)

// ---- doubles -----------------------------------------------------------

type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	put     map[string]string // key -> content type
	statErr error
	getErr  error
	putErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, put: map[string]string{}}
}

func (f *fakeStore) StatObject(_ context.Context, key string) (*domain.ObjectInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	if !ok {
		return nil, domain.ErrObjectNotFound
	}
	return &domain.ObjectInfo{Key: key, SizeBytes: int64(len(b)), ContentType: "video/mp4"}, nil
}

func (f *fakeStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	if !ok {
		return nil, domain.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (f *fakeStore) PutObject(_ context.Context, key, contentType string, r io.Reader, _ int64) error {
	if f.putErr != nil {
		return f.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = b
	f.put[key] = contentType
	return nil
}

func (f *fakeStore) uploadedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.put {
		out = append(out, k)
	}
	return out
}

type fakeResources struct {
	mu          sync.Mutex
	resource    *domain.Resource
	transitions []domain.ResourceProcessingStatus
	getErr      error
	setErr      error
}

func (f *fakeResources) GetByID(context.Context, string) (*domain.Resource, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := *f.resource
	return &copied, nil
}

func (f *fakeResources) SetProcessingStatus(_ context.Context, _ string, status domain.ResourceProcessingStatus, entry *domain.AuditEntry) (*domain.Resource, error) {
	if f.setErr != nil {
		return nil, f.setErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry == nil {
		return nil, errors.New("a status transition must carry an audit entry")
	}
	f.transitions = append(f.transitions, status)
	f.resource.ProcessingStatus = string(status)
	copied := *f.resource
	return &copied, nil
}

func (f *fakeResources) seen() []domain.ResourceProcessingStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.ResourceProcessingStatus(nil), f.transitions...)
}

// fakeTranscoder writes plausible HLS output without needing ffmpeg.
type fakeTranscoder struct {
	info     *transcode.MediaInfo
	probeErr error
	hlsErr   error
	encoded  []string
}

func (f *fakeTranscoder) Probe(context.Context, string) (*transcode.MediaInfo, error) {
	if f.probeErr != nil {
		return nil, f.probeErr
	}
	return f.info, nil
}

func (f *fakeTranscoder) ToHLS(_ context.Context, _, outDir string, r transcode.Rendition) error {
	if f.hlsErr != nil {
		return f.hlsErr
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	f.encoded = append(f.encoded, r.Name)
	if err := os.WriteFile(filepath.Join(outDir, r.PlaylistName()), []byte("#EXTM3U\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, strings.Replace(r.SegmentPattern(), "%04d", "0000", 1)), []byte("segment"), 0o644)
}

// ---- harness -----------------------------------------------------------

type harness struct {
	proc      *MediaProcessor
	store     *fakeStore
	resources *fakeResources
	tc        *fakeTranscoder
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	store := newFakeStore()
	store.objects[originalKey] = bytes.Repeat([]byte("v"), 4096)

	resources := &fakeResources{resource: &domain.Resource{
		ID: resourceID, StableID: stableID, Type: domain.ResourceTypeVideo,
		ObjectKey: originalKey, ProcessingStatus: string(domain.ResourceProcessingPending),
	}}
	tc := &fakeTranscoder{info: &transcode.MediaInfo{HasVideo: true, HasAudio: true, Width: 1920, Height: 1080, DurationSeconds: 12}}

	return &harness{
		proc:      NewMediaProcessor(store, resources, tc, MediaProcessorOptions{WorkDir: t.TempDir()}, discardLogger()),
		store:     store,
		resources: resources,
		tc:        tc,
	}
}

func mediaTask(t *testing.T) *asynq.Task {
	t.Helper()
	tk, err := task.NewMediaProcessTask(resourceID, "hls_transcode", originalKey)
	if err != nil {
		t.Fatalf("NewMediaProcessTask: %v", err)
	}
	return tk
}

// ---- tests -------------------------------------------------------------

func TestMediaProcessorHappyPath(t *testing.T) {
	h := newHarness(t)

	if err := h.proc.Handle(context.Background(), mediaTask(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got, want := h.resources.seen(), []domain.ResourceProcessingStatus{
		domain.ResourceProcessingProcessing, domain.ResourceProcessingCompleted,
	}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("transitions = %v, want processing then completed", got)
	}

	keys := h.store.uploadedKeys()
	var master string
	for _, k := range keys {
		if strings.HasSuffix(k, "master.m3u8") {
			master = k
		}
		if !strings.HasPrefix(k, "hls/"+stableID+"/") {
			t.Errorf("derivative %q is not under the resource's hls prefix", k)
		}
	}
	if master == "" {
		t.Errorf("no master playlist among %v", keys)
	}
	// 1080p source takes both rungs: two playlists, two segments, one master.
	if len(keys) != 5 {
		t.Errorf("uploaded %d files %v, want 5", len(keys), keys)
	}
}

// The original is the only copy of what the author uploaded, and the statement
// requires it be preserved.
func TestMediaProcessorPreservesTheOriginal(t *testing.T) {
	h := newHarness(t)
	before := append([]byte(nil), h.store.objects[originalKey]...)

	if err := h.proc.Handle(context.Background(), mediaTask(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !bytes.Equal(h.store.objects[originalKey], before) {
		t.Error("the original was modified")
	}
	for _, k := range h.store.uploadedKeys() {
		if k == originalKey {
			t.Error("the pipeline wrote over the original")
		}
	}
}

// Manifests and segments need real content types or a player will not read them.
func TestMediaProcessorTypesTheDerivatives(t *testing.T) {
	h := newHarness(t)
	if err := h.proc.Handle(context.Background(), mediaTask(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	for key, ct := range h.store.put {
		switch {
		case strings.HasSuffix(key, ".m3u8"):
			if ct != "application/vnd.apple.mpegurl" {
				t.Errorf("%s served as %q", key, ct)
			}
		case strings.HasSuffix(key, ".ts"):
			if ct != "video/mp2t" {
				t.Errorf("%s served as %q", key, ct)
			}
		}
	}
}

// A duplicate delivery must not produce a second set of derivatives. This is
// the worker's own guard, independent of the queue's idempotency middleware.
func TestMediaProcessorSkipsAnAlreadyCompletedObject(t *testing.T) {
	h := newHarness(t)
	h.resources.resource.ProcessingStatus = string(domain.ResourceProcessingCompleted)

	if err := h.proc.Handle(context.Background(), mediaTask(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got := h.resources.seen(); len(got) != 0 {
		t.Errorf("status was moved %v on an already completed object", got)
	}
	if got := h.store.uploadedKeys(); len(got) != 0 {
		t.Errorf("uploaded %v for an already completed object", got)
	}
	if len(h.tc.encoded) != 0 {
		t.Errorf("re-encoded %v for an already completed object", h.tc.encoded)
	}
}

// A re-upload lands on a new key, so the same resource does deserve processing
// again even though it is already completed.
func TestMediaProcessorReprocessesANewObject(t *testing.T) {
	h := newHarness(t)
	h.resources.resource.ProcessingStatus = string(domain.ResourceProcessingCompleted)
	h.resources.resource.ObjectKey = "originals/" + stableID + "/older.mp4"

	if err := h.proc.Handle(context.Background(), mediaTask(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := h.resources.seen(); len(got) != 2 {
		t.Errorf("transitions = %v, want processing then completed", got)
	}
}

// A corrupt upload is permanent: it must be recorded as failed and told not to
// retry, rather than failing three times and only then saying why.
func TestMediaProcessorMarksUnprocessableInputFailed(t *testing.T) {
	h := newHarness(t)
	h.tc.probeErr = errors.New("moov atom not found: " + transcode.ErrUnprocessable.Error())
	h.tc.probeErr = transcode.ErrUnprocessable

	err := h.proc.Handle(context.Background(), mediaTask(t))
	if err == nil {
		t.Fatal("expected an error for unprocessable input")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("error %v does not carry SkipRetry; the job would be retried", err)
	}

	got := h.resources.seen()
	if len(got) != 2 || got[1] != domain.ResourceProcessingFailed {
		t.Errorf("transitions = %v, want processing then failed", got)
	}
}

// A storage blip is transient: the job should retry, and the resource must not
// be branded failed on the author's behalf.
func TestMediaProcessorRetriesTransientFailures(t *testing.T) {
	h := newHarness(t)
	h.store.getErr = errors.New("connection reset by peer")

	err := h.proc.Handle(context.Background(), mediaTask(t))
	if err == nil {
		t.Fatal("expected an error for a storage failure")
	}
	if errors.Is(err, asynq.SkipRetry) {
		t.Error("a transient storage failure should not skip retries")
	}

	for _, s := range h.resources.seen() {
		if s == domain.ResourceProcessingFailed {
			t.Error("a transient failure marked the resource failed")
		}
	}
}

// An object the confirmation recorded but that is no longer in the bucket is
// not going to reappear on a retry.
func TestMediaProcessorTreatsAMissingOriginalAsPermanent(t *testing.T) {
	h := newHarness(t)
	delete(h.store.objects, originalKey)

	err := h.proc.Handle(context.Background(), mediaTask(t))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v, want SkipRetry", err)
	}
	if got := h.resources.seen(); got[len(got)-1] != domain.ResourceProcessingFailed {
		t.Errorf("transitions = %v, want it to end in failed", got)
	}
}

func TestMediaProcessorRejectsAnOversizedOriginal(t *testing.T) {
	h := newHarness(t)
	h.proc.opts.MaxOriginalBytes = 10 // the fixture is 4096 bytes

	err := h.proc.Handle(context.Background(), mediaTask(t))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v, want SkipRetry", err)
	}
	if len(h.tc.encoded) != 0 {
		t.Error("an oversized original was handed to the encoder anyway")
	}
}

func TestMediaProcessorRejectsAMalformedPayload(t *testing.T) {
	h := newHarness(t)

	err := h.proc.Handle(context.Background(), asynq.NewTask("media:process", []byte("{not json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v, want SkipRetry", err)
	}
	if got := h.resources.seen(); len(got) != 0 {
		t.Errorf("a malformed payload moved the resource status: %v", got)
	}
}

// A resource deleted while its job sat in the queue has nothing to process.
func TestMediaProcessorHandlesADeletedResource(t *testing.T) {
	h := newHarness(t)
	h.resources.getErr = domain.ErrNotFound

	err := h.proc.Handle(context.Background(), mediaTask(t))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v, want SkipRetry", err)
	}
}

func TestMediaProcessorEncodesOnlyWhatFitsTheSource(t *testing.T) {
	h := newHarness(t)
	h.tc.info = &transcode.MediaInfo{HasVideo: true, HasAudio: true, Width: 640, Height: 360}

	if err := h.proc.Handle(context.Background(), mediaTask(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(h.tc.encoded) != 1 || h.tc.encoded[0] != "360p" {
		t.Errorf("encoded %v, want only 360p for a 360p source", h.tc.encoded)
	}
}

func TestHLSContentType(t *testing.T) {
	cases := map[string]string{
		"master.m3u8":  "application/vnd.apple.mpegurl",
		"720p_0000.ts": "video/mp2t",
		"chunk.m4s":    "video/mp4",
	}
	for name, want := range cases {
		if got := hlsContentType(name); got != want {
			t.Errorf("hlsContentType(%q) = %q, want %q", name, got, want)
		}
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

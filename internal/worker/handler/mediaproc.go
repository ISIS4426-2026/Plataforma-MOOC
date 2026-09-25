package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/transcode"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// The ports the processor needs, declared here rather than taken whole so the
// doubles in the tests stay small.
type (
	// ObjectStore is the slice of the storage provider the pipeline uses: pull
	// the original down, push the derivatives up.
	ObjectStore interface {
		GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
		PutObject(ctx context.Context, objectKey, contentType string, r io.Reader, size int64) error
		StatObject(ctx context.Context, objectKey string) (*domain.ObjectInfo, error)
	}

	// ResourceStore reads the resource being processed and reports the outcome
	// back onto it.
	ResourceStore interface {
		GetByID(ctx context.Context, id string) (*domain.Resource, error)
		SetProcessingStatus(ctx context.Context, resourceID string, status domain.ResourceProcessingStatus, entry *domain.AuditEntry) (*domain.Resource, error)
	}

	// Transcoder is what turns the downloaded original into HLS.
	Transcoder interface {
		Probe(ctx context.Context, path string) (*transcode.MediaInfo, error)
		ToHLS(ctx context.Context, inputPath, outDir string, rendition transcode.Rendition) error
	}
)

// MediaProcessorOptions tune the pipeline.
type MediaProcessorOptions struct {
	// WorkDir is where originals are downloaded and renditions written. Empty
	// uses the system temp directory.
	WorkDir string
	// MaxOriginalBytes caps what will be pulled down. A signed upload cannot be
	// size-limited at the bucket, so this is the worker's own guard against a
	// file that would fill the VM's 30 GiB disk.
	MaxOriginalBytes int64
	// Ladder is the declared set of renditions. Empty uses transcode.DefaultLadder.
	Ladder []transcode.Rendition
}

// MediaProcessor runs the media pipeline for one job.
//
// The shape of the work is: mark the resource processing, pull the original to
// local disk, ask ffprobe what it is, encode the renditions that do not upscale
// it, upload the results, and mark it completed. Any step can fail, and the
// resource is left in a state that says which.
type MediaProcessor struct {
	store      ObjectStore
	resources  ResourceStore
	transcoder Transcoder
	opts       MediaProcessorOptions
	logger     *slog.Logger
}

func NewMediaProcessor(store ObjectStore, resources ResourceStore, transcoder Transcoder, opts MediaProcessorOptions, logger *slog.Logger) *MediaProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.MaxOriginalBytes <= 0 {
		opts.MaxOriginalBytes = 2 << 30 // 2 GiB
	}
	if len(opts.Ladder) == 0 {
		opts.Ladder = transcode.DefaultLadder
	}
	return &MediaProcessor{store: store, resources: resources, transcoder: transcoder, opts: opts, logger: logger}
}

// Handle is the asynq entry point for task.TypeMediaProcess.
func (p *MediaProcessor) Handle(ctx context.Context, t *asynq.Task) error {
	var payload task.MediaProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		// A payload we cannot read will not become readable on a retry.
		return fmt.Errorf("media job carries an unreadable payload: %v: %w", err, asynq.SkipRetry)
	}
	if payload.ResourceID == "" || payload.StoragePath == "" {
		return fmt.Errorf("media job is missing resource_id or storage_path: %w", asynq.SkipRetry)
	}

	started := time.Now()
	err := p.process(ctx, payload)
	if err == nil {
		p.logger.InfoContext(ctx, "media job completed",
			slog.String("resource_id", payload.ResourceID),
			slog.String("object_key", payload.StoragePath),
			slog.Duration("took", time.Since(started)),
		)
		return nil
	}

	// A permanent failure gets recorded on the resource and told not to retry.
	// A transient one is left alone: the status stays `processing` and asynq
	// tries again, so a Redis blip does not present itself to the author as a
	// broken upload.
	permanent := errors.Is(err, transcode.ErrUnprocessable)
	p.logger.ErrorContext(ctx, "media job failed",
		slog.String("resource_id", payload.ResourceID),
		slog.String("object_key", payload.StoragePath),
		slog.Bool("permanent", permanent),
		slog.Duration("took", time.Since(started)),
		slog.String("error", err.Error()),
	)

	if permanent {
		p.markFailed(ctx, payload.ResourceID)
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	return err
}

func (p *MediaProcessor) process(ctx context.Context, payload task.MediaProcessPayload) error {
	resource, err := p.resources.GetByID(ctx, payload.ResourceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// The resource was deleted while the job waited in the queue. There
			// is nothing to process and nothing to report; retrying would just
			// repeat the same lookup.
			return fmt.Errorf("resource %s no longer exists: %w", payload.ResourceID, transcode.ErrUnprocessable)
		}
		return fmt.Errorf("look up resource %s: %w", payload.ResourceID, err)
	}

	// A resource that already reached `completed` on this same object has been
	// processed. Re-running would overwrite identical derivatives and burn a
	// worker slot, so a duplicate delivery stops here.
	if resource.ProcessingStatus == string(domain.ResourceProcessingCompleted) &&
		resource.ObjectKey == payload.StoragePath {
		p.logger.InfoContext(ctx, "media job skipped; this object is already processed",
			slog.String("resource_id", resource.ID),
			slog.String("object_key", payload.StoragePath),
		)
		return nil
	}

	if _, err := p.resources.SetProcessingStatus(ctx, resource.ID, domain.ResourceProcessingProcessing, p.entry(resource.ID)); err != nil {
		return fmt.Errorf("mark resource %s processing: %w", resource.ID, err)
	}

	workDir, err := os.MkdirTemp(p.opts.WorkDir, "mooc-media-*")
	if err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	// The original and every rendition live here. Removing the tree is what
	// keeps a worker that processes hundreds of jobs from filling its disk.
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			p.logger.WarnContext(ctx, "could not clean the work directory",
				slog.String("dir", workDir), slog.String("error", err.Error()))
		}
	}()

	originalPath, downloaded, err := p.download(ctx, payload.StoragePath, workDir)
	if err != nil {
		return err
	}

	info, err := p.transcoder.Probe(ctx, originalPath)
	if err != nil {
		return err
	}

	renditions, err := transcode.SelectRenditions(info, p.opts.Ladder)
	if err != nil {
		return err
	}

	outDir := filepath.Join(workDir, "hls")
	for _, r := range renditions {
		if err := p.transcoder.ToHLS(ctx, originalPath, outDir, r); err != nil {
			return err
		}
	}

	master, err := transcode.MasterPlaylist(renditions, info.Width, info.Height)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "master.m3u8"), []byte(master), 0o644); err != nil {
		return fmt.Errorf("write master playlist: %w", err)
	}

	uploaded, err := p.uploadDerivatives(ctx, outDir, storage.HLSPrefix(resource.StableID))
	if err != nil {
		return err
	}

	if _, err := p.resources.SetProcessingStatus(ctx, resource.ID, domain.ResourceProcessingCompleted, p.entry(resource.ID)); err != nil {
		return fmt.Errorf("mark resource %s completed: %w", resource.ID, err)
	}

	p.logger.InfoContext(ctx, "media rendered to HLS",
		slog.String("resource_id", resource.ID),
		slog.String("hls_prefix", storage.HLSPrefix(resource.StableID)),
		slog.Int("renditions", len(renditions)),
		slog.String("ladder", renditionNames(renditions)),
		slog.Int("files_uploaded", uploaded),
		slog.Int64("original_bytes", downloaded),
		slog.Int("source_width", info.Width),
		slog.Int("source_height", info.Height),
		slog.Float64("duration_seconds", info.DurationSeconds),
	)
	return nil
}

// download pulls the original to local disk, refusing anything over the cap.
// The original itself is never touched or replaced: the statement requires it
// be preserved, and it is the only copy of what the author actually uploaded.
func (p *MediaProcessor) download(ctx context.Context, objectKey, workDir string) (path string, size int64, err error) {
	info, err := p.store.StatObject(ctx, objectKey)
	if err != nil {
		if errors.Is(err, domain.ErrObjectNotFound) {
			return "", 0, fmt.Errorf("original %q is not in the bucket: %w", objectKey, transcode.ErrUnprocessable)
		}
		return "", 0, fmt.Errorf("stat original %q: %w", objectKey, err)
	}
	if info.SizeBytes > p.opts.MaxOriginalBytes {
		return "", 0, fmt.Errorf("original is %d bytes, over the %d byte processing limit: %w",
			info.SizeBytes, p.opts.MaxOriginalBytes, transcode.ErrUnprocessable)
	}

	reader, err := p.store.GetObject(ctx, objectKey)
	if err != nil {
		return "", 0, fmt.Errorf("open original %q: %w", objectKey, err)
	}
	defer func() { _ = reader.Close() }()

	path = filepath.Join(workDir, "original"+filepath.Ext(objectKey))
	f, err := os.Create(path)
	if err != nil {
		return "", 0, fmt.Errorf("create local copy: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Bounded even though the stat above already checked: the object could have
	// been replaced between the two calls, and an unbounded copy onto a 30 GiB
	// disk is not a risk worth taking on a stat from a moment ago.
	size, err = io.Copy(f, io.LimitReader(reader, p.opts.MaxOriginalBytes+1))
	if err != nil {
		return "", 0, fmt.Errorf("download original %q: %w", objectKey, err)
	}
	if size > p.opts.MaxOriginalBytes {
		return "", 0, fmt.Errorf("original exceeds the %d byte processing limit: %w",
			p.opts.MaxOriginalBytes, transcode.ErrUnprocessable)
	}
	return path, size, nil
}

// uploadDerivatives pushes everything under outDir to the bucket, flat, under
// prefix.
func (p *MediaProcessor) uploadDerivatives(ctx context.Context, outDir, prefix string) (int, error) {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return 0, fmt.Errorf("read rendition output: %w", err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		local := filepath.Join(outDir, e.Name())
		f, err := os.Open(local)
		if err != nil {
			return count, fmt.Errorf("open %s: %w", e.Name(), err)
		}
		stat, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return count, fmt.Errorf("stat %s: %w", e.Name(), err)
		}

		key := prefix + "/" + e.Name()
		err = p.store.PutObject(ctx, key, hlsContentType(e.Name()), f, stat.Size())
		_ = f.Close()
		if err != nil {
			return count, fmt.Errorf("upload %s: %w", key, err)
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("the encoder produced no output: %w", transcode.ErrUnprocessable)
	}
	return count, nil
}

// markFailed records a permanent failure. It is best-effort on purpose: the job
// has already failed, and losing the status write should not turn a diagnosable
// failure into a retry loop.
func (p *MediaProcessor) markFailed(ctx context.Context, resourceID string) {
	if _, err := p.resources.SetProcessingStatus(ctx, resourceID, domain.ResourceProcessingFailed, p.entry(resourceID)); err != nil {
		p.logger.ErrorContext(ctx, "could not mark the resource failed",
			slog.String("resource_id", resourceID),
			slog.String("error", err.Error()),
		)
	}
}

// entry builds the audit record for a pipeline transition. ActorID stays nil:
// the platform acted on its own, and inventing an actor would make the trail
// say something untrue.
func (p *MediaProcessor) entry(resourceID string) *domain.AuditEntry {
	return &domain.AuditEntry{
		Action:         domain.AuditActionResourceMediaProcessed,
		TargetResource: "resource:" + resourceID,
	}
}

// hlsContentType types a derivative so the bucket serves it correctly. A player
// handed a manifest as application/octet-stream will not play it.
func hlsContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".m4s", ".mp4":
		return "video/mp4"
	default:
		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}

func renditionNames(rs []transcode.Rendition) string {
	names := make([]string, len(rs))
	for i, r := range rs {
		names[i] = r.Name
	}
	return strings.Join(names, ",")
}

// Command seed-media builds the multimedia half of the G1 dataset (issue
// #127): three real videos, transcoded through the exact production HLS
// pipeline (internal/transcode + internal/worker/handler.MediaProcessor),
// seeded into the bucket as originals and derivatives, reconciled against
// what the database and the bucket actually hold, and written out as
// docs/entrega2/evidencias/G1/manifest.json.
//
// It deliberately reuses MediaProcessor.Handle instead of reimplementing the
// download-probe-transcode-upload sequence: the point of the evidence is that
// the seeded derivatives are what the real worker would have produced from
// the same original, not a lookalike built by a second code path.
//
// Entrega 1 never built object storage, so there are no pre-existing objects
// to migrate -- this command is the first thing that has ever written into
// the bucket's originals/ and hls/ prefixes for this dataset.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/transcode"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/handler"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

// Deterministic IDs for the dedicated course this command owns, following the
// same letter-prefixed-block convention as scripts/seeds/synthetic_data.sql
// (a=admin, b=profesor, c=estudiante, d/e=row id/stable id). "d9"/"e9" is a
// block nothing else in the seed uses.
const (
	mediaCourseID       = "d9000000-0000-0000-0000-000000000001"
	mediaCourseStableID = "e9000000-0000-0000-0000-000000000001"
	mediaModuleID       = "d9100000-0000-0000-0000-000000000001"
	mediaModuleStableID = "e9100000-0000-0000-0000-000000000001"
	mediaUnitID         = "d9200000-0000-0000-0000-000000000001"
	mediaUnitStableID   = "e9200000-0000-0000-0000-000000000001"

	// mediaAuthorID is profesor1 from the functional seed. This command
	// assumes `make seed` (or ./scripts/seed.sh --load) already ran.
	mediaAuthorID = "b0000000-0000-0000-0000-000000000001"
)

// mediaProfile is one of the three declared durations. Resolution is fixed at
// 1280x720 for all three: source at 720p is what makes the 360p+720p ladder
// (internal/transcode.DefaultLadder) apply without ever upscaling, and 720p
// is exactly the ceiling the ladder declares.
//
// These are the same three profiles CONFIGURACION_Y_COSTOS.md §3 costs out
// under "Corto/Medio/Largo" -- that table is only comparable to what actually
// runs if the two agree, which is why the durations are not free choices.
type mediaProfile struct {
	Name             string // used in filenames and the manifest
	Label            string // human title for the resource
	DurationSeconds  int
	ResourceID       string
	ResourceStableID string
	Position         int
}

var profiles = []mediaProfile{
	{
		Name: "corto", Label: "Video de prueba: perfil corto (2 min, 1280x720)",
		DurationSeconds:  2 * 60,
		ResourceID:       "d9300000-0000-0000-0000-000000000001",
		ResourceStableID: "e9300000-0000-0000-0000-000000000001",
		Position:         0,
	},
	{
		Name: "medio", Label: "Video de prueba: perfil medio (10 min, 1280x720)",
		DurationSeconds:  10 * 60,
		ResourceID:       "d9300000-0000-0000-0000-000000000002",
		ResourceStableID: "e9300000-0000-0000-0000-000000000002",
		Position:         1,
	},
	{
		Name: "largo", Label: "Video de prueba: perfil largo (30 min, 1280x720)",
		DurationSeconds:  30 * 60,
		ResourceID:       "d9300000-0000-0000-0000-000000000003",
		ResourceStableID: "e9300000-0000-0000-0000-000000000003",
		Position:         2,
	},
}

// objectRecord is one uploaded object: enough to reconcile it against the
// bucket afterwards and to describe it in the manifest.
type objectRecord struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"` // "original" | "derivative"
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
}

// profileManifest is one profile's entry in manifest.json.
type profileManifest struct {
	Profile          string         `json:"profile"`
	Label            string         `json:"label"`
	DurationSeconds  int            `json:"duration_seconds"`
	ResourceID       string         `json:"resource_id"`
	ResourceStableID string         `json:"resource_stable_id"`
	SourceResolution string         `json:"source_resolution"`
	Ladder           []string       `json:"ladder"`
	Original         objectRecord   `json:"original"`
	Derivatives      []objectRecord `json:"derivatives"`
}

type reconciliationFinding struct {
	Key      string `json:"key"`
	Problem  string `json:"problem"`
	Expected int64  `json:"expected_size_bytes,omitempty"`
	Actual   int64  `json:"actual_size_bytes,omitempty"`
}

type manifest struct {
	GeneratedAt        time.Time               `json:"generated_at"`
	CourseID           string                  `json:"course_id"`
	CourseStableID     string                  `json:"course_stable_id"`
	Bucket             string                  `json:"bucket"`
	StorageBackend     string                  `json:"storage_backend"`
	Profiles           []profileManifest       `json:"profiles"`
	TotalObjects       int                     `json:"total_objects"`
	TotalBytes         int64                   `json:"total_bytes"`
	Reconciliation     []reconciliationFinding `json:"reconciliation_discrepancies"`
	ReconciliationOK   bool                    `json:"reconciliation_ok"`
	NoPriorObjectsNote string                  `json:"no_prior_objects_note"`
}

func main() {
	profileFlag := flag.String("profiles", "corto,medio,largo", "Comma-separated subset of profiles to (re)generate")
	evidenceDir := flag.String("evidence-dir", "docs/entrega2/evidencias/G1", "Where to write manifest.json")
	flag.Parse()

	selected, err := selectProfiles(*profileFlag)
	if err != nil {
		log.Fatalf("Invalid -profiles: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 10*time.Second)
	db, err := postgres.Connect(connectCtx, cfg)
	cancelConnect()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := ensureAuthorExists(ctx, db); err != nil {
		log.Fatalf("%v\nRun `make seed` (or ./scripts/seed.sh --load) before seed-media.", err)
	}

	storageProvider, err := storage.New(ctx, storage.Config{
		Backend:   storage.Backend(cfg.StorageBackend),
		Bucket:    cfg.S3Bucket,
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
	})
	if err != nil {
		log.Fatalf("Failed to initialise object storage: %v", err)
	}

	runner := transcode.NewRunner()
	if err := runner.Available(); err != nil {
		log.Fatalf("FFmpeg is required to generate and transcode the profiles: %v", err)
	}
	ladder := transcode.DefaultLadder

	courseRepo := postgres.NewCourseRepository(db)
	moduleRepo := postgres.NewModuleRepository(db)
	unitRepo := postgres.NewUnitRepository(db)
	resourceRepo := postgres.NewResourceRepository(db)

	// Rerunning this command has to be able to redo the media pipeline, and
	// AttachMedia/Create both refuse a published course. If a previous run
	// already published it, drop it back to draft first -- direct SQL, same
	// as how scripts/seeds/synthetic_data.sql assigns course status directly
	// rather than through the authoring service.
	if err := forceCourseDraft(ctx, db, mediaCourseID); err != nil {
		log.Fatalf("Failed to prepare the media course for (re)generation: %v", err)
	}

	if err := ensureCourseModuleUnit(ctx, courseRepo, moduleRepo, unitRepo); err != nil {
		log.Fatalf("Failed to ensure the media course structure: %v", err)
	}

	workDir := cfg.MediaWorkDir
	if workDir == "" {
		workDir = os.TempDir()
	}
	runDir, err := os.MkdirTemp(workDir, "seed-media-*")
	if err != nil {
		log.Fatalf("Failed to create a work directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(runDir) }()

	mediaProcessor := handler.NewMediaProcessor(
		storageProvider,
		// Un solo bucket: la siembra corre contra el entorno local.
		nil,
		resourceRepo,
		runner,
		handler.MediaProcessorOptions{WorkDir: runDir, Ladder: ladder},
		nil,
	)

	var profileManifests []profileManifest
	var totalObjects int
	var totalBytes int64

	for _, p := range selected {
		log.Printf("=== Perfil %s: %s (%ds) ===", p.Name, p.Label, p.DurationSeconds)

		pm, err := processProfile(ctx, p, runDir, runner.FFmpegPath, storageProvider, resourceRepo, mediaProcessor, ladder)
		if err != nil {
			log.Fatalf("Perfil %s falló: %v", p.Name, err)
		}
		profileManifests = append(profileManifests, pm)
		totalObjects += 1 + len(pm.Derivatives)
		totalBytes += pm.Original.SizeBytes
		for _, d := range pm.Derivatives {
			totalBytes += d.SizeBytes
		}
		log.Printf("Perfil %s: original %.1f MB, %d derivados subidos", p.Name,
			float64(pm.Original.SizeBytes)/1e6, len(pm.Derivatives))
	}

	// Only publish once every selected profile finished; a partial run should
	// not leave a "published" course with unprocessed resources inside it.
	// A subset run (-profiles=corto) does not publish, since the course would
	// misrepresent what it actually contains.
	if len(selected) == len(profiles) {
		if err := publishCourse(ctx, db, mediaCourseID); err != nil {
			log.Fatalf("Failed to publish the media course: %v", err)
		}
		log.Printf("Curso %s publicado.", mediaCourseID)
	} else {
		log.Printf("Ejecución parcial (-profiles=%s): el curso se deja en draft.", *profileFlag)
	}

	findings, err := reconcile(ctx, storageProvider, profileManifests)
	if err != nil {
		log.Fatalf("Reconciliation failed to run: %v", err)
	}

	m := manifest{
		GeneratedAt:      time.Now().UTC(),
		CourseID:         mediaCourseID,
		CourseStableID:   mediaCourseStableID,
		Bucket:           cfg.S3Bucket,
		StorageBackend:   cfg.StorageBackend,
		Profiles:         profileManifests,
		TotalObjects:     totalObjects,
		TotalBytes:       totalBytes,
		Reconciliation:   findings,
		ReconciliationOK: len(findings) == 0,
		NoPriorObjectsNote: "La Entrega 1 nunca implementó almacenamiento de objetos: " +
			"no existían objetos previos que migrar. Todo lo que hay bajo originals/ y " +
			"hls/ para este dataset lo escribió este comando.",
	}

	if err := writeManifest(*evidenceDir, m); err != nil {
		log.Fatalf("Failed to write manifest.json: %v", err)
	}

	log.Printf("=== Listo: %d objetos, %.1f MB, reconciliación %s ===",
		m.TotalObjects, float64(m.TotalBytes)/1e6, okLabel(m.ReconciliationOK))
	if !m.ReconciliationOK {
		log.Printf("Discrepancias encontradas:")
		for _, f := range findings {
			log.Printf("  - %s: %s (esperado=%d, real=%d)", f.Key, f.Problem, f.Expected, f.Actual)
		}
		os.Exit(1)
	}
}

func okLabel(ok bool) string {
	if ok {
		return "OK"
	}
	return "CON DISCREPANCIAS"
}

func selectProfiles(spec string) ([]mediaProfile, error) {
	wanted := map[string]bool{}
	for _, name := range strings.Split(spec, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			wanted[name] = true
		}
	}
	if len(wanted) == 0 {
		return nil, fmt.Errorf("no profiles named")
	}
	var out []mediaProfile
	for _, p := range profiles {
		if wanted[p.Name] {
			out = append(out, p)
			delete(wanted, p.Name)
		}
	}
	if len(wanted) > 0 {
		var unknown []string
		for name := range wanted {
			unknown = append(unknown, name)
		}
		return nil, fmt.Errorf("unknown profile(s): %s", strings.Join(unknown, ", "))
	}
	return out, nil
}

func ensureAuthorExists(ctx context.Context, db *sql.DB) error {
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1`, mediaAuthorID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("author %s (profesor1) does not exist yet", mediaAuthorID)
	}
	return err
}

// forceCourseDraft drops the media course back to draft if it already exists
// from a previous run, bypassing the authoring service the same way the
// functional seed bypasses it to set a course's initial status directly.
func forceCourseDraft(ctx context.Context, db *sql.DB, courseID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE courses SET status = 'draft', updated_at = NOW() WHERE id = $1 AND status <> 'draft'`,
		courseID)
	return err
}

func publishCourse(ctx context.Context, db *sql.DB, courseID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE courses SET status = 'published', updated_at = NOW() WHERE id = $1`,
		courseID)
	return err
}

func auditEntry(action domain.AuditAction, target string) *domain.AuditEntry {
	return &domain.AuditEntry{Action: action, TargetResource: target}
}

func ensureCourseModuleUnit(ctx context.Context, courseRepo *postgres.CourseRepository, moduleRepo *postgres.ModuleRepository, unitRepo *postgres.UnitRepository) error {
	if _, err := courseRepo.GetByID(ctx, mediaCourseID); errors.Is(err, domain.ErrNotFound) {
		course := &domain.Course{
			ID:          mediaCourseID,
			StableID:    mediaCourseStableID,
			Title:       "Perfiles de Video para Pruebas de Capacidad",
			Description: "Curso técnico, sin contenido académico, que existe únicamente para sembrar tres perfiles reales de video (corto/medio/largo) procesados por el pipeline de HLS. Ver docs/entrega2/evidencias/G1.",
			Version:     1,
			Status:      domain.CourseStatusDraft,
			AuthorID:    mediaAuthorID,
		}
		if err := courseRepo.Create(ctx, course, auditEntry(domain.AuditActionCourseCreated, "course:"+mediaCourseID)); err != nil {
			return fmt.Errorf("create media course: %w", err)
		}
	} else if err != nil {
		return err
	}

	if _, err := moduleRepo.GetByID(ctx, mediaModuleID); errors.Is(err, domain.ErrNotFound) {
		module := &domain.Module{
			ID: mediaModuleID, StableID: mediaModuleStableID, CourseID: mediaCourseID,
			Title: "Perfiles de video",
		}
		if err := moduleRepo.Create(ctx, module, auditEntry(domain.AuditActionModuleCreated, "module:"+mediaModuleID)); err != nil {
			return fmt.Errorf("create media module: %w", err)
		}
	} else if err != nil {
		return err
	}

	if _, err := unitRepo.GetByID(ctx, mediaUnitID); errors.Is(err, domain.ErrNotFound) {
		unit := &domain.Unit{
			ID: mediaUnitID, StableID: mediaUnitStableID, ModuleID: mediaModuleID,
			Title: "Videos de prueba",
		}
		if err := unitRepo.Create(ctx, unit, auditEntry(domain.AuditActionUnitCreated, "unit:"+mediaUnitID)); err != nil {
			return fmt.Errorf("create media unit: %w", err)
		}
	} else if err != nil {
		return err
	}

	return nil
}

func ensureVideoResource(ctx context.Context, resourceRepo *postgres.ResourceRepository, p mediaProfile) (*domain.Resource, error) {
	res, err := resourceRepo.GetByID(ctx, p.ResourceID)
	if errors.Is(err, domain.ErrNotFound) {
		res = &domain.Resource{
			ID: p.ResourceID, StableID: p.ResourceStableID, UnitID: mediaUnitID,
			Title: p.Label, Type: domain.ResourceTypeVideo, Position: p.Position,
			// Not mandatory: these resources exist to hold seeded media, not to
			// be completed by a student, and marking them mandatory would
			// change the denominator every progress-percentage test in
			// scripts/seeds/synthetic_data.sql depends on -- see NOTAS_TECNICAS
			// note 7. They live in their own course precisely to keep that
			// denominator untouched.
			IsVisible: true, IsMandatory: false, AllowDownload: false,
		}
		if err := resourceRepo.Create(ctx, res, auditEntry(domain.AuditActionResourceCreated, "resource:"+p.ResourceID)); err != nil {
			return nil, fmt.Errorf("create resource: %w", err)
		}
		return res, nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// processProfile generates the synthetic original, uploads it, runs it
// through the real MediaProcessor pipeline, and returns everything the
// manifest and the reconciliation pass need to know about it.
func processProfile(
	ctx context.Context,
	p mediaProfile,
	runDir string,
	ffmpegPath string,
	store domain.StorageProvider,
	resourceRepo *postgres.ResourceRepository,
	mediaProcessor *handler.MediaProcessor,
	ladder []transcode.Rendition,
) (profileManifest, error) {
	res, err := ensureVideoResource(ctx, resourceRepo, p)
	if err != nil {
		return profileManifest{}, err
	}

	originalPath := filepath.Join(runDir, p.Name+"-original.mp4")
	if err := generateSyntheticVideo(ctx, ffmpegPath, originalPath, p.DurationSeconds); err != nil {
		return profileManifest{}, fmt.Errorf("generate synthetic video: %w", err)
	}

	objectKey, err := storage.OriginalKey(res.StableID, "video.mp4")
	if err != nil {
		return profileManifest{}, fmt.Errorf("build original key: %w", err)
	}

	sha, size, err := hashFile(originalPath)
	if err != nil {
		return profileManifest{}, fmt.Errorf("hash original: %w", err)
	}
	if err := uploadPath(ctx, store, objectKey, "video/mp4", originalPath, size); err != nil {
		return profileManifest{}, fmt.Errorf("upload original: %w", err)
	}
	original := objectRecord{Key: objectKey, Kind: "original", ContentType: "video/mp4", SizeBytes: size, SHA256: sha}

	if _, err := resourceRepo.AttachMedia(ctx, res.ID, objectKey, auditEntry(domain.AuditActionResourceMediaAttached, "resource:"+res.ID)); err != nil {
		return profileManifest{}, fmt.Errorf("attach media: %w", err)
	}

	// This is the real production path: the same asynq task a signed upload
	// confirmation enqueues, handled synchronously by the same MediaProcessor
	// the worker runs, so the derivatives this command seeds are exactly what
	// the worker would have produced.
	t, err := task.NewMediaProcessTask(res.ID, "hls_transcode", objectKey)
	if err != nil {
		return profileManifest{}, fmt.Errorf("build media task: %w", err)
	}
	if err := mediaProcessor.Handle(ctx, t); err != nil {
		return profileManifest{}, fmt.Errorf("transcode: %w", err)
	}

	// The derivatives now live under hls/<stable_id>/. Their segment count
	// depends on ffmpeg's own keyframe placement, not a clean division of the
	// duration by the target segment length, so their names are discovered by
	// reading the manifests themselves -- master.m3u8 names the rendition
	// playlists, and each rendition playlist names its own segments. That is
	// also exactly how a real player resolves them, which is what makes this
	// a meaningful reconciliation and not just a re-statement of what we
	// assume ToHLS wrote.
	prefix := storage.HLSPrefix(res.StableID)
	derivatives, err := discoverDerivatives(ctx, store, prefix)
	if err != nil {
		return profileManifest{}, fmt.Errorf("discover derivatives: %w", err)
	}

	ladderNames := make([]string, len(ladder))
	for i, r := range ladder {
		ladderNames[i] = fmt.Sprintf("%s(%dkbps video + %dkbps audio)", r.Name, r.VideoBitrateKbps, r.AudioBitrateKbps)
	}

	return profileManifest{
		Profile:          p.Name,
		Label:            p.Label,
		DurationSeconds:  p.DurationSeconds,
		ResourceID:       res.ID,
		ResourceStableID: res.StableID,
		SourceResolution: "1280x720",
		Ladder:           ladderNames,
		Original:         original,
		Derivatives:      derivatives,
	}, nil
}

func generateSyntheticVideo(ctx context.Context, ffmpegPath string, outPath string, durationSeconds int) error {
	genCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	args := []string{
		"-hide_banner", "-loglevel", "error", "-y", "-nostdin",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=1280x720:rate=25:duration=%d", durationSeconds),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", durationSeconds),
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-shortest", outPath,
	}
	cmd := exec.CommandContext(genCtx, ffmpegPath, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %v: %s", err, lastLine(stderr.String()))
	}
	return nil
}

func hashFile(path string) (sha256hex string, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func uploadPath(ctx context.Context, store domain.StorageProvider, key, contentType, path string, size int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return store.PutObject(ctx, key, contentType, f, size)
}

// discoverDerivatives walks the HLS tree the way a player would: read
// master.m3u8 to find the rendition playlists, read each rendition playlist
// to find its segments, and stat everything along the way. There is no bucket
// List operation in domain.StorageProvider (the API never lists the bucket
// either -- see docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md), so following
// the manifests is the only way to enumerate what a profile actually
// produced.
func discoverDerivatives(ctx context.Context, store domain.StorageProvider, prefix string) ([]objectRecord, error) {
	masterKey := prefix + "/master.m3u8"
	masterBody, masterInfo, err := getAndStat(ctx, store, masterKey)
	if err != nil {
		return nil, fmt.Errorf("read master playlist: %w", err)
	}
	out := []objectRecord{{Key: masterKey, Kind: "derivative", ContentType: "application/vnd.apple.mpegurl", SizeBytes: masterInfo.SizeBytes}}

	for _, renditionName := range playlistReferences(masterBody) {
		renditionKey := prefix + "/" + renditionName
		renditionBody, renditionInfo, err := getAndStat(ctx, store, renditionKey)
		if err != nil {
			return nil, fmt.Errorf("read rendition playlist %s: %w", renditionName, err)
		}
		out = append(out, objectRecord{Key: renditionKey, Kind: "derivative", ContentType: "application/vnd.apple.mpegurl", SizeBytes: renditionInfo.SizeBytes})

		for _, segmentName := range playlistReferences(renditionBody) {
			segKey := prefix + "/" + segmentName
			info, err := store.StatObject(ctx, segKey)
			if err != nil {
				return nil, fmt.Errorf("stat segment %s (referenced by %s): %w", segKey, renditionName, err)
			}
			out = append(out, objectRecord{Key: segKey, Kind: "derivative", ContentType: "video/mp2t", SizeBytes: info.SizeBytes})
		}
	}
	return out, nil
}

func getAndStat(ctx context.Context, store domain.StorageProvider, key string) (body string, info *domain.ObjectInfo, err error) {
	info, err = store.StatObject(ctx, key)
	if err != nil {
		return "", nil, err
	}
	r, err := store.GetObject(ctx, key)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = r.Close() }()

	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	return string(data), info, nil
}

// playlistReferences returns the non-comment, non-blank lines of an HLS
// playlist. Every such line is a filename the playlist references, relative
// to its own directory -- true of both the master (naming rendition
// playlists) and a rendition playlist (naming its segments).
func playlistReferences(playlist string) []string {
	var out []string
	for _, line := range strings.Split(playlist, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// reconcile re-stats every object this run claims exist and compares the
// size the bucket reports right now against the size recorded at upload
// time. Discrepancies here mean the bucket drifted from what the manifest
// (and by extension the database, since object_key points at the original)
// says it holds.
func reconcile(ctx context.Context, store domain.StorageProvider, pms []profileManifest) ([]reconciliationFinding, error) {
	var findings []reconciliationFinding
	for _, pm := range pms {
		info, err := store.StatObject(ctx, pm.Original.Key)
		if errors.Is(err, domain.ErrObjectNotFound) {
			findings = append(findings, reconciliationFinding{Key: pm.Original.Key, Problem: "original missing from bucket", Expected: pm.Original.SizeBytes})
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.SizeBytes != pm.Original.SizeBytes {
			findings = append(findings, reconciliationFinding{Key: pm.Original.Key, Problem: "size mismatch", Expected: pm.Original.SizeBytes, Actual: info.SizeBytes})
		}
		for _, d := range pm.Derivatives {
			dInfo, err := store.StatObject(ctx, d.Key)
			if errors.Is(err, domain.ErrObjectNotFound) {
				findings = append(findings, reconciliationFinding{Key: d.Key, Problem: "derivative missing from bucket", Expected: d.SizeBytes})
				continue
			}
			if err != nil {
				return nil, err
			}
			if dInfo.SizeBytes != d.SizeBytes {
				findings = append(findings, reconciliationFinding{Key: d.Key, Problem: "size mismatch", Expected: d.SizeBytes, Actual: dInfo.SizeBytes})
			}
		}
	}
	return findings, nil
}

func writeManifest(dir string, m manifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return "no output"
}

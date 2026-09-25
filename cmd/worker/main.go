package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/observability"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/transcode"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/handler"
	"github.com/hibiken/asynq"
)

// serviceVersion mirrors cmd/api/main.go's constant of the same purpose.
const serviceVersion = "0.1.0"

func main() {
	enqueueFlag := flag.Bool("enqueue-test", false, "Enqueue a test ping job to Redis and exit")
	msgFlag := flag.String("message", "Test ping job from CLI", "Custom message for test ping job")
	flag.BoolVar(enqueueFlag, "enqueue", false, "Alias for -enqueue-test")
	flag.Parse()

	cfg := config.Load()

	if *enqueueFlag {
		if err := enqueueTestJob(cfg, *msgFlag); err != nil {
			log.Fatalf("Failed to enqueue test job: %v\n", err)
		}
		return
	}

	log.Printf("Starting Plataforma MOOC Background Worker [Env: %s]\n", cfg.Environment)

	// Traces and metrics (issue #21): the worker gets its own service name
	// so a span or a "jobs processed" sample is attributable to the process
	// that produced it, distinct from the API's.
	obs, err := observability.Setup(observability.Config{
		ServiceName:    "plataforma-mooc-worker",
		ServiceVersion: serviceVersion,
		Environment:    cfg.Environment,
	})
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v\n", err)
	}

	// The media pipeline needs three things the ping handler does not: the
	// bucket to read originals from and write renditions to, the database to
	// report each transition on, and ffmpeg on PATH.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := postgres.Connect(startupCtx, cfg)
	cancelStartup()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v\n", err)
	}
	defer func() { _ = db.Close() }()

	storageProvider, err := storage.New(context.Background(), storage.Config{
		Backend:   storage.Backend(cfg.StorageBackend),
		Bucket:    cfg.S3Bucket,
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
	})
	if err != nil {
		log.Fatalf("Failed to initialise object storage: %v\n", err)
	}

	runner := transcode.NewRunner()
	if err := runner.Available(); err != nil {
		// Better to refuse to start than to accept media jobs and fail every one
		// of them three times over before the DLQ finally says why.
		log.Fatalf("FFmpeg is required by the media pipeline: %v\n", err)
	}

	ladder, err := transcode.ParseLadder(cfg.MediaHLSLadder)
	if err != nil {
		// A malformed ladder has to stop startup. Discovering it mid-run would
		// leave a capacity measurement describing an encoding profile nobody
		// declared.
		log.Fatalf("Invalid MEDIA_HLS_LADDER: %v\n", err)
	}
	log.Printf("Media ladder: %s\n", transcode.FormatLadder(ladder))

	mediaProcessor := handler.NewMediaProcessor(
		storageProvider,
		postgres.NewResourceRepository(db),
		runner,
		handler.MediaProcessorOptions{
			WorkDir:          cfg.MediaWorkDir,
			MaxOriginalBytes: cfg.MediaMaxOriginalBytes,
			Ladder:           ladder,
		},
		nil,
	)

	engine, err := worker.NewWorkerEngine(cfg,
		worker.WithMeter(obs.Meter),
		worker.WithMediaProcessor(asynq.HandlerFunc(mediaProcessor.Handle)),
	)
	if err != nil {
		log.Fatalf("Failed to initialize worker engine: %v\n", err)
	}

	// The worker has no other HTTP server, so metrics get a small one of
	// their own rather than sharing the API's -- the worker is meant to
	// scale and fail independently of it.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", obs.MetricsHandler)
	metricsServer := &http.Server{Addr: ":" + cfg.MetricsPort, Handler: metricsMux}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server stopped: %v\n", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	engineErr := engine.Start(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("failed to shut down metrics server: %v\n", err)
	}
	if err := obs.Shutdown(shutdownCtx); err != nil {
		log.Printf("failed to shut down observability providers: %v\n", err)
	}

	if engineErr != nil {
		log.Fatalf("Worker engine encountered fatal error: %v\n", engineErr)
	}
}

func enqueueTestJob(cfg *config.Config, msg string) error {
	client, err := worker.NewClient(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to create worker client: %w", err)
	}
	defer client.Close()

	info, err := client.EnqueueTestPing(context.Background(), msg)
	if err != nil {
		return fmt.Errorf("failed to enqueue test ping job: %w", err)
	}

	log.Printf("[CLI] Successfully enqueued test job [ID: %s, Type: %s, Queue: %s]\n", info.ID, info.Type, info.Queue)
	return nil
}

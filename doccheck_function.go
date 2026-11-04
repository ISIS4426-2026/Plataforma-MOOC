// Package doccheck is the entry point of the Cloud Run function that validates documents
// (Entrega 5). It lives at the root of the module on purpose: Cloud Functions builds the
// package found at the root of the uploaded source, and a function inside a subdirectory
// could not import this module's internal/ packages. Everything that matters is in
// internal/worker/eventfn and internal/worker/handler, where it is tested; this file only
// wires them to the CloudEvent the platform delivers.
//
// It is deployed by Terraform (infra/terraform/functions.tf) from a zip of this module.
package doccheck

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/storage"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/eventfn"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/handler"
)

func init() {
	functions.CloudEvent("DocCheck", docCheck)
}

var (
	mu        sync.Mutex
	processor *handler.MediaProcessor
	logger    = newLogger()
)

// newLogger writes JSON that Cloud Logging understands: `severity` and `message`.
func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severity"
			case slog.MessageKey:
				a.Key = "message"
			}
			return a
		},
	}))
}

// get builds the processor on the first event instead of in init(): a database that is
// momentarily unreachable then fails one invocation (which is retried) and not the whole
// instance start. A failure is not remembered, so the next event tries again.
func get(ctx context.Context) (*handler.MediaProcessor, error) {
	mu.Lock()
	defer mu.Unlock()
	if processor != nil {
		return processor, nil
	}

	cfg := config.Load()

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	db, err := postgres.Connect(connectCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to the database: %w", err)
	}

	store, err := storage.New(ctx, storage.Config{
		Backend:   storage.Backend(cfg.StorageBackend),
		Bucket:    cfg.S3Bucket,
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialise object storage: %w", err)
	}

	// Same processor the worker uses. No transcoder and no derivatives bucket: this
	// function only ever runs the document branch.
	processor = handler.NewMediaProcessor(store, nil, postgres.NewResourceRepository(db), nil,
		handler.MediaProcessorOptions{MaxOriginalBytes: cfg.MediaMaxOriginalBytes}, logger)
	return processor, nil
}

func docCheck(ctx context.Context, e event.Event) error {
	p, err := get(ctx)
	if err != nil {
		// Not consumed: the trigger redelivers, and the next attempt builds it again.
		return err
	}
	return eventfn.Handle(ctx, p, logger, e.Data())
}

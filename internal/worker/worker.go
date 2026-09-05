package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/handler"
	"github.com/hibiken/asynq"
)

// WorkerEngine manages the background task processing worker lifecycle.
type WorkerEngine struct {
	cfg         *config.Config
	server      *asynq.Server
	mux         *asynq.ServeMux
	logger      *slog.Logger
	concurrency int
}

// WorkerOption defines functional configuration options for WorkerEngine.
type WorkerOption func(*WorkerEngine)

// WithLogger sets a custom structured logger on WorkerEngine.
func WithLogger(logger *slog.Logger) WorkerOption {
	return func(we *WorkerEngine) {
		we.logger = logger
	}
}

// WithConcurrency sets the number of concurrent task processors for this worker instance.
func WithConcurrency(n int) WorkerOption {
	return func(we *WorkerEngine) {
		we.concurrency = n
	}
}

// NewWorkerEngine creates and configures a stateless background worker engine instance.
func NewWorkerEngine(cfg *config.Config, opts ...WorkerOption) (*WorkerEngine, error) {
	redisOpt, err := ParseRedisOpt(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid worker redis configuration: %w", err)
	}

	defaultLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	we := &WorkerEngine{
		cfg:         cfg,
		logger:      defaultLogger,
		concurrency: 10,
	}

	for _, opt := range opts {
		opt(we)
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: we.concurrency,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			Logger: NewSlogAsynqAdapter(we.logger),
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				taskID, _ := asynq.GetTaskID(ctx)
				we.logger.ErrorContext(ctx, "[Worker ErrorHandler] Task execution failed",
					slog.String("id", taskID),
					slog.String("tipo", task.Type()),
					slog.String("error", err.Error()),
				)
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.Use(LoggingMiddleware(we.logger))
	handler.RegisterRoutes(mux)

	we.server = srv
	we.mux = mux

	return we, nil
}

// Start launches the worker processing loop and blocks until the context is cancelled.
func (w *WorkerEngine) Start(ctx context.Context) error {
	w.logger.Info("[Worker Engine] Starting background worker processor...",
		slog.String("env", w.cfg.Environment),
		slog.String("redis_url", w.cfg.RedisURL),
		slog.Int("concurrency", w.concurrency),
	)

	if err := w.server.Start(w.mux); err != nil {
		return fmt.Errorf("failed to start asynq worker server: %w", err)
	}

	<-ctx.Done()
	w.logger.Info("[Worker Engine] Shutting down background worker processor...")
	w.server.Shutdown()
	return nil
}

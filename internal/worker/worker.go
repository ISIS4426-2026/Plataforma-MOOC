package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/handler"
	"github.com/hibiken/asynq"
)

// AlertHandlerFunc is a function signature for listening to alerts when a task fails and moves to DLQ.
type AlertHandlerFunc func(ctx context.Context, task *asynq.Task, err error)

// WorkerEngine manages the background task processing worker lifecycle.
type WorkerEngine struct {
	cfg                      *config.Config
	server                   *asynq.Server
	mux                      *asynq.ServeMux
	logger                   *slog.Logger
	concurrency              int
	retryDelayFunc           asynq.RetryDelayFunc
	alertHandler             AlertHandlerFunc
	idempotencyStore         IdempotencyStore
	queues                   map[string]int
	delayedTaskCheckInterval time.Duration
	taskCheckInterval        time.Duration
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

// WithRetryDelayFunc configures a custom retry backoff function.
func WithRetryDelayFunc(fn asynq.RetryDelayFunc) WorkerOption {
	return func(we *WorkerEngine) {
		we.retryDelayFunc = fn
	}
}

// WithAlertHandler registers a callback handler triggered when a task reaches max retries and lands in the DLQ.
func WithAlertHandler(fn AlertHandlerFunc) WorkerOption {
	return func(we *WorkerEngine) {
		we.alertHandler = fn
	}
}

// WithIdempotencyStore sets a custom idempotency store on WorkerEngine.
func WithIdempotencyStore(store IdempotencyStore) WorkerOption {
	return func(we *WorkerEngine) {
		we.idempotencyStore = store
	}
}

// WithQueues configures custom queue priority mappings for WorkerEngine.
func WithQueues(queues map[string]int) WorkerOption {
	return func(we *WorkerEngine) {
		we.queues = queues
	}
}

// WithDelayedTaskCheckInterval configures the check interval for delayed/retry tasks.
func WithDelayedTaskCheckInterval(d time.Duration) WorkerOption {
	return func(we *WorkerEngine) {
		we.delayedTaskCheckInterval = d
	}
}

// WithTaskCheckInterval configures the check interval for empty queue polling.
func WithTaskCheckInterval(d time.Duration) WorkerOption {
	return func(we *WorkerEngine) {
		we.taskCheckInterval = d
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
		cfg:              cfg,
		logger:           defaultLogger,
		concurrency:      10,
		retryDelayFunc:   DefaultExponentialBackoff,
		idempotencyStore: NewMemoryIdempotencyStore(),
		queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	}

	for _, opt := range opts {
		opt(we)
	}

	srvConfig := asynq.Config{
		Concurrency:    we.concurrency,
		Queues:         we.queues,
		Logger:         NewSlogAsynqAdapter(we.logger),
		RetryDelayFunc: we.retryDelayFunc,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			taskID, _ := asynq.GetTaskID(ctx)
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)

			we.logger.ErrorContext(ctx, "[Worker ErrorHandler] Task execution failed",
				slog.String("id", taskID),
				slog.String("tipo", task.Type()),
				slog.Int("retried", retried),
				slog.Int("max_retry", maxRetry),
				slog.String("error", err.Error()),
			)

			// Condition of Section 6 & Criterion 2:
			// After exactly 3 retries (or retried >= maxRetry), task falls into DLQ (archived queue) and alert is raised.
			if retried >= maxRetry {
				we.logger.ErrorContext(ctx, "[ALERT] Job moved to Dead-Letter Queue (DLQ)",
					slog.String("alert", "DLQ_JOB_FAILED"),
					slog.String("id", taskID),
					slog.String("tipo", task.Type()),
					slog.Int("retried", retried),
					slog.Int("max_retry", maxRetry),
					slog.String("error", err.Error()),
				)
				if we.alertHandler != nil {
					we.alertHandler(ctx, task, err)
				}
			}
		}),
	}

	if we.delayedTaskCheckInterval > 0 {
		srvConfig.DelayedTaskCheckInterval = we.delayedTaskCheckInterval
	}
	if we.taskCheckInterval > 0 {
		srvConfig.TaskCheckInterval = we.taskCheckInterval
	}

	srv := asynq.NewServer(redisOpt, srvConfig)

	mux := asynq.NewServeMux()
	mux.Use(IdempotencyMiddleware(we.idempotencyStore, we.logger))
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

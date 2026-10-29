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
	"go.opentelemetry.io/otel/metric"
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
	meter                    metric.Meter
	mediaProcessor           asynq.Handler
	shutdownTimeout          time.Duration
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

// WithShutdownTimeout sets how long a drain waits for the jobs already running
// before handing them back to the queue (C7, issue #210).
//
// Que hace exactamente, porque el nombre se presta:
//
// El plazo **no** retrasa nada del apagado salvo la espera por las tareas en
// vuelo. En cuanto llega la senal, asynq deja de tomar tareas nuevas de la cola
// --`processor.stop()`-- y eso ocurre siempre, valga lo que valga este numero.
// Lo unico que decide es cuanto se espera a las que ya empezaron.
//
// Y lo que pasa al vencer tampoco es una perdida: asynq cierra su canal `abort`
// y la tarea vuelve a la cola con `requeue`, que **no consume un reintento**
// --no es `handleFailedMessage`--. Un drenaje que se queda corto cuesta trabajo
// repetido, nunca un trabajo perdido ni un reintento gastado.
//
// Sin esta opcion manda el valor por omision de asynq, que son **8 segundos**:
// menos que cualquier trabajo real de esta plataforma --A7 midio 144 s en el
// perfil corto-- y mucho menos que los 120 s que `mooc-worker.service` ya le
// concede al contenedor. Es decir, sin esto el plazo de la unidad no sirve de
// nada: el proceso se rinde mucho antes de que nadie lo mate.
//
// Un valor <= 0 se ignora y deja el de asynq, porque un cero accidental no debe
// convertirse en «aborta en el acto».
func WithShutdownTimeout(d time.Duration) WorkerOption {
	return func(we *WorkerEngine) {
		if d > 0 {
			we.shutdownTimeout = d
		}
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

// WithMeter enables MetricsMiddleware (issue #21: jobs processed/failed).
// Omitting it -- as every existing test does -- leaves the worker exactly as
// it was before this option existed, metrics included.
// WithMediaProcessor supplies the handler for media jobs. Without it the engine
// still starts -- a worker that only runs ping jobs is useful in tests -- but
// media jobs fail loudly instead of silently succeeding.
func WithMediaProcessor(h asynq.Handler) WorkerOption {
	return func(w *WorkerEngine) {
		w.mediaProcessor = h
	}
}

func WithMeter(meter metric.Meter) WorkerOption {
	return func(we *WorkerEngine) {
		we.meter = meter
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

	if we.shutdownTimeout > 0 {
		srvConfig.ShutdownTimeout = we.shutdownTimeout
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
	if we.meter != nil {
		mux.Use(MetricsMiddleware(we.meter))

		queueNames := make([]string, 0, len(we.queues))
		for name := range we.queues {
			queueNames = append(queueNames, name)
		}
		if err := RegisterQueueMetrics(we.meter, redisOpt, queueNames); err != nil {
			return nil, fmt.Errorf("register queue metrics: %w", err)
		}
	}
	handler.RegisterRoutes(mux, we.mediaProcessor)

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
		slog.String("drenaje", w.drainTimeout().String()),
	)

	if err := w.server.Start(w.mux); err != nil {
		return fmt.Errorf("failed to start asynq worker server: %w", err)
	}

	<-ctx.Done()

	// Las dos lineas que rodean a `Shutdown` son la evidencia de C7: entre
	// ellas esta el drenaje entero, y la diferencia de marcas de tiempo dice
	// cuanto duro de verdad. Sin ellas habria que deducirlo de los mensajes
	// internos de asynq, que no dicen con que plazo se configuro.
	inicio := time.Now()
	w.logger.Info("[Worker Engine] drenaje iniciado: no se aceptan trabajos nuevos",
		slog.String("plazo", w.drainTimeout().String()),
		slog.String("al_vencer", "lo que siga en curso vuelve a la cola sin gastar un reintento"),
	)

	w.server.Shutdown()

	w.logger.Info("[Worker Engine] drenaje terminado",
		slog.String("duración", time.Since(inicio).String()),
		slog.Int64("duración_ms", time.Since(inicio).Milliseconds()),
	)
	return nil
}

// drainTimeout devuelve el plazo que el servidor va a usar de verdad, que es el
// configurado o, si no se configuro, el de asynq. El 8 esta escrito aqui porque
// asynq no exporta su constante, y una traza que mienta sobre el plazo es peor
// que no tenerla: el sintoma de C7 es exactamente creer que el plazo es uno
// cuando es otro.
func (w *WorkerEngine) drainTimeout() time.Duration {
	if w.shutdownTimeout > 0 {
		return w.shutdownTimeout
	}
	return 8 * time.Second
}

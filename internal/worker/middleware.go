package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// LoggingMiddleware returns an Asynq MiddlewareFunc that performs structured logging
// for every job with fields: id, tipo, estado, duración.
func LoggingMiddleware(logger *slog.Logger) asynq.MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			taskID, _ := asynq.GetTaskID(ctx)
			if taskID == "" && t.ResultWriter() != nil {
				taskID = t.ResultWriter().TaskID()
			}
			taskType := t.Type()
			start := time.Now()

			logger.InfoContext(ctx, fmt.Sprintf("job %s started", taskType),
				slog.String("id", taskID),
				slog.String("tipo", taskType),
				slog.String("estado", "started"),
			)

			err := next.ProcessTask(ctx, t)
			duration := time.Since(start)

			// UNA TAREA INTERRUMPIDA NO ES UNA TAREA QUE FALLO.
			//
			// Cuando un worker se retira (C7, #210) asynq cancela el contexto
			// de lo que este en curso, y el manejador devuelve
			// `context.Canceled`. Con la linea normal, el registro del drenaje
			// --que es la evidencia que C7 tiene que capturar-- dice
			// `estado=failed` sobre un trabajo que esta sano y que va a volver
			// a la cola. Quien lea esa captura dentro de seis meses no tiene
			// como saber que no fue un error.
			//
			// Lo que cambia es solo la traza: el error se devuelve igual y
			// asynq decide lo mismo que decidia antes.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.WarnContext(ctx, fmt.Sprintf("job %s interrumpido", taskType),
					slog.String("id", taskID),
					slog.String("tipo", taskType),
					slog.String("estado", "interrumpido"),
					slog.String("duración", duration.String()),
					slog.Int64("duración_ms", duration.Milliseconds()),
					slog.String("motivo", "el worker se esta retirando; la tarea vuelve a la cola, no es un fallo del trabajo"),
					slog.String("error", err.Error()),
				)
				return err
			}

			if err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("job %s failed", taskType),
					slog.String("id", taskID),
					slog.String("tipo", taskType),
					slog.String("estado", "failed"),
					slog.String("duración", duration.String()),
					slog.Int64("duración_ms", duration.Milliseconds()),
					slog.String("error", err.Error()),
				)
				return err
			}

			logger.InfoContext(ctx, fmt.Sprintf("job %s completed", taskType),
				slog.String("id", taskID),
				slog.String("tipo", taskType),
				slog.String("estado", "completed"),
				slog.String("duración", duration.String()),
				slog.Int64("duración_ms", duration.Milliseconds()),
			)

			return nil
		})
	}
}

// IdempotencyMiddleware returns an Asynq MiddlewareFunc that enforces task idempotency.
// If the task has already been processed successfully for its IdempotencyKey (or task ID),
// execution of task side effects is skipped and success is returned immediately.
func IdempotencyMiddleware(store IdempotencyStore, logger *slog.Logger) asynq.MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			if store == nil {
				return next.ProcessTask(ctx, t)
			}

			key := extractIdempotencyKey(t, ctx)
			if key == "" {
				return next.ProcessTask(ctx, t)
			}

			processed, err := store.IsProcessed(ctx, key)
			if err != nil {
				logger.ErrorContext(ctx, "[IdempotencyMiddleware] Failed to check idempotency status",
					slog.String("key", key),
					slog.String("error", err.Error()),
				)
			} else if processed {
				logger.InfoContext(ctx, "[IdempotencyMiddleware] Task already processed successfully, skipping execution to prevent duplicate effects",
					slog.String("idempotency_key", key),
					slog.String("tipo", t.Type()),
				)
				return nil
			}

			err = next.ProcessTask(ctx, t)
			if err == nil {
				if markErr := store.MarkProcessed(ctx, key, 24*time.Hour); markErr != nil {
					logger.ErrorContext(ctx, "[IdempotencyMiddleware] Failed to mark task as processed",
						slog.String("key", key),
						slog.String("error", markErr.Error()),
					)
				}
			}

			return err
		})
	}
}

func extractIdempotencyKey(t *asynq.Task, ctx context.Context) string {
	var payload struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err == nil && payload.IdempotencyKey != "" {
		return payload.IdempotencyKey
	}

	taskID, _ := asynq.GetTaskID(ctx)
	if taskID != "" {
		return taskID
	}
	if t.ResultWriter() != nil {
		return t.ResultWriter().TaskID()
	}

	return ""
}

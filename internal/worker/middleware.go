package worker

import (
	"context"
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

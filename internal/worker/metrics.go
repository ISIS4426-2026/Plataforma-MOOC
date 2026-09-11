package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsMiddleware returns an Asynq MiddlewareFunc that counts jobs
// processed and failed, by task type (issue #21: "jobs procesados/fallidos"
// among the minimum metrics OpenTelemetry must expose).
//
// It complements LoggingMiddleware rather than replacing it: the log line
// answers "what happened to this one job", the counter answers "how many
// jobs of this type have failed" -- the question an alert or a dashboard
// asks, which a log line alone cannot answer without being parsed first.
func MetricsMiddleware(meter metric.Meter) asynq.MiddlewareFunc {
	processed, _ := meter.Int64Counter("worker.jobs.processed",
		metric.WithDescription("Trabajos completados exitosamente, por tipo."),
	)
	failed, _ := meter.Int64Counter("worker.jobs.failed",
		metric.WithDescription("Trabajos que devolvieron error, por tipo."),
	)

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			attrs := metric.WithAttributes(attribute.String("type", t.Type()))

			err := next.ProcessTask(ctx, t)
			if err != nil {
				failed.Add(ctx, 1, attrs)
				return err
			}

			processed.Add(ctx, 1, attrs)
			return nil
		})
	}
}

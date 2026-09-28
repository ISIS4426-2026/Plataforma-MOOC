package worker

import (
	"context"
	"time"

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
	// Cuánto tarda un trabajo, que los contadores no pueden responder. El
	// enunciado pide separar la duración del procesamiento del tiempo de espera
	// en cola, y sin esto la única fuente era el generador de carga, que mide
	// desde fuera y no distingue una cosa de la otra.
	//
	// Los cortes van en segundos y cubren de medio segundo a diez minutos: una
	// transcodificación no se parece a una petición HTTP, así que los de por
	// defecto -- pensados para milisegundos -- dejarían todo en un solo cubo.
	duration, _ := meter.Float64Histogram("worker.jobs.duration",
		metric.WithDescription("Duración del procesamiento de un trabajo, por tipo."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600),
	)

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			attrs := metric.WithAttributes(attribute.String("type", t.Type()))
			inicio := time.Now()

			err := next.ProcessTask(ctx, t)

			// Se registra la duración de las dos ramas: un trabajo que falla
			// tarde -- una transcodificación que muere a los cinco minutos --
			// cuesta tanta CPU como uno que termina, y omitirlo haría parecer
			// más barata la saturación de lo que es.
			duration.Record(ctx, time.Since(inicio).Seconds(), attrs)

			if err != nil {
				failed.Add(ctx, 1, attrs)
				return err
			}

			processed.Add(ctx, 1, attrs)
			return nil
		})
	}
}

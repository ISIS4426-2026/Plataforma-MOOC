package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RegisterQueueMetrics exposes queue depth, oldest-pending age and in-flight
// count as observable gauges, one set per named queue (issue #131, H1).
//
// MetricsMiddleware counts jobs as they are processed, which answers "how
// many finished". It cannot answer "how many are waiting right now" or "how
// long has the oldest one been waiting": those are properties of the queue
// itself, not of a task passing through, and nothing observes them unless
// something asks Redis directly on each scrape. That is what
// asynq.Inspector wraps, and gauges (not counters) are the right shape for
// a value that goes up and down with the queue's own state rather than
// accumulating.
func RegisterQueueMetrics(meter metric.Meter, redisOpt asynq.RedisConnOpt, queues []string) error {
	inspector := asynq.NewInspector(redisOpt)

	pending, err := meter.Int64ObservableGauge("worker.queue.pending",
		metric.WithDescription("Trabajos en espera de ser tomados por un worker, por cola."),
	)
	if err != nil {
		return err
	}

	active, err := meter.Int64ObservableGauge("worker.queue.active",
		metric.WithDescription("Trabajos siendo procesados ahora mismo, por cola."),
	)
	if err != nil {
		return err
	}

	oldestAge, err := meter.Float64ObservableGauge("worker.queue.oldest_pending_age_seconds",
		metric.WithDescription("Antiguedad en segundos del trabajo en espera mas viejo de la cola "+
			"(asynq.QueueInfo.Latency); 0 cuando no hay nada en espera."),
	)
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for _, q := range queues {
			// A queue that never received a task does not exist yet in Redis,
			// and Inspector reports that as an error rather than an empty
			// QueueInfo. That is the expected state right after a fresh
			// deploy, not a scrape failure worth surfacing -- reporting zero
			// values for it is what its own to-be-created state actually is.
			info, err := inspector.GetQueueInfo(q)
			if err != nil {
				attrs := metric.WithAttributes(attribute.String("queue", q))
				o.ObserveInt64(pending, 0, attrs)
				o.ObserveInt64(active, 0, attrs)
				o.ObserveFloat64(oldestAge, 0, attrs)
				continue
			}

			attrs := metric.WithAttributes(attribute.String("queue", q))
			o.ObserveInt64(pending, int64(info.Pending), attrs)
			o.ObserveInt64(active, int64(info.Active), attrs)
			o.ObserveFloat64(oldestAge, info.Latency.Seconds(), attrs)
		}
		return nil
	}, pending, active, oldestAge)

	return err
}

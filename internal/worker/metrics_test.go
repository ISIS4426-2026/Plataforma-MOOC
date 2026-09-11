package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
)

// counterValue collects rm and returns the int64 sum recorded for the named
// counter, or 0 if the counter has recorded nothing yet.
func counterValue(t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect returned an error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %q is not an int64 sum: %T", name, m.Data)
			}
			var total int64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total
		}
	}
	return 0
}

// The acceptance criterion this backs: "jobs procesados/fallidos" are among
// the minimum metrics issue #21 requires OpenTelemetry to expose.
func TestMetricsMiddlewareCountsProcessedAndFailedJobs(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := meterProvider.Meter("test")

	var callCount int
	handler := asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		callCount++
		if callCount%2 == 0 {
			return errors.New("simulated failure")
		}
		return nil
	})

	wrapped := worker.MetricsMiddleware(meter)(handler)

	task := asynq.NewTask("test:ping", nil)
	ctx := context.Background()

	if err := wrapped.ProcessTask(ctx, task); err != nil {
		t.Fatalf("first task returned an unexpected error: %v", err)
	}
	if err := wrapped.ProcessTask(ctx, task); err == nil {
		t.Fatal("second task should have failed")
	}
	if err := wrapped.ProcessTask(ctx, task); err != nil {
		t.Fatalf("third task returned an unexpected error: %v", err)
	}

	if got := counterValue(t, reader, "worker.jobs.processed"); got != 2 {
		t.Errorf("worker.jobs.processed = %d, want 2", got)
	}
	if got := counterValue(t, reader, "worker.jobs.failed"); got != 1 {
		t.Errorf("worker.jobs.failed = %d, want 1", got)
	}
}

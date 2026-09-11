// Package observability wires the OpenTelemetry half of issue #21. Structured
// JSON logging with level, timestamp and request_id already existed (slog in
// cmd/api/main.go and cmd/worker/main.go, internal/http/middleware.RequestLogger);
// what was missing was traces and metrics, and a way to find one from the other.
//
// # Correlation
//
// internal/http/middleware.Tracing starts one span per HTTP request and
// carries it in the request context; RequestLogger, running just after it in
// the chain, reads the span's trace ID out of that same context and adds it
// to every access-log line alongside request_id. Both identifiers end up on
// the span too (as attributes), so a request_id from an API response can
// locate its log line and its span by the same search, and the span carries
// the trace_id that ties it to the log line that names it.
//
// # Where the data goes
//
// Spans are written as JSON to stdout -- the same stream `docker compose
// logs` already captures for the application's own structured logs, so nothing
// new has to run or be configured to correlate them; issue #21's acceptance
// test ("dado un request-id, se puede encontrar su log y su traza") is one
// grep. Metrics are exposed Prometheus-style on GET /api/v1/metrics rather
// than pushed, for the same reason: inspectable from the running process,
// no collector to stand up for an MVP.
package observability

import (
	"context"
	"fmt"
	"os"

	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"net/http"
)

// Config names the process being instrumented, so a span or a metric sample
// can be traced back to which service and version produced it.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
}

// Providers is what Setup wires up: a tracer and a meter to instrument code
// with, the HTTP handler that exposes metrics for scraping, and a Shutdown
// that flushes and releases both providers.
type Providers struct {
	Tracer         trace.Tracer
	Meter          metric.Meter
	MetricsHandler http.Handler
	Shutdown       func(context.Context) error
}

// Setup builds the tracer and meter providers described in the package doc.
// It never returns a nil *Providers on a nil error: a caller can always
// instrument code and mount MetricsHandler without a further nil check.
func Setup(cfg Config) (*Providers, error) {
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironmentName(cfg.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	traceExporter, err := stdouttrace.New(stdouttrace.WithWriter(os.Stdout))
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	// A dedicated registry, rather than promclient.DefaultRegisterer:
	// Setup can then be called more than once (each test that needs its own
	// Providers, for instance) without a "duplicate metrics collector
	// registration" panic from a shared global.
	registry := promclient.NewRegistry()
	metricExporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricExporter),
		sdkmetric.WithResource(res),
	)

	shutdown := func(ctx context.Context) error {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("shut down tracer provider: %w", err)
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("shut down meter provider: %w", err)
		}
		return nil
	}

	return &Providers{
		Tracer:         tracerProvider.Tracer(cfg.ServiceName),
		Meter:          meterProvider.Meter(cfg.ServiceName),
		MetricsHandler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		Shutdown:       shutdown,
	}, nil
}

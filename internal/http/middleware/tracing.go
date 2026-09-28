package middleware

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Tracing starts one span per request and records the latency and error-rate
// metrics issue #21 asks for.
//
// It must run after RequestID (so the request_id it attaches to the span
// already exists) and before RequestLogger (so TraceIDFrom finds a span in
// the context RequestLogger reads from) -- server.go's middleware order
// encodes both.
func Tracing(tracer trace.Tracer, meter metric.Meter) Middleware {
	// Errors here are deliberately swallowed rather than propagated: a
	// misconfigured meter must not stop the API from serving requests, only
	// leave these two instruments recording nothing.
	duration, _ := meter.Float64Histogram("http.server.request.duration",
		metric.WithDescription("Duración de las solicitudes HTTP, por endpoint."),
		metric.WithUnit("s"),
		// Los cortes van en segundos, como la unidad. Sin esta línea se usan los
		// de por defecto -- 0, 5, 10, 25 … 10000 --, que están pensados para
		// milisegundos: contra una medida en segundos **todas** las peticiones
		// caen en el primer cubo distinto de cero y el histograma queda con un
		// solo valor útil. Así salió en la campaña de H3: 3 126 peticiones, las
		// 3 126 en `le="5"`, y un p95 imposible de calcular desde aquí.
		metric.WithExplicitBucketBoundaries(
			.005, .01, .025, .05, .075, .1, .25, .5, .75, 1, 2.5, 5, 7.5, 10,
		),
	)
	errorCounter, _ := meter.Int64Counter("http.server.request.errors",
		metric.WithDescription("Solicitudes HTTP respondidas con error de servidor (5xx), por endpoint."),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
				attribute.String("request_id", RequestIDFrom(ctx)),
			)

			start := time.Now()
			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r.WithContext(ctx))
			elapsed := time.Since(start)

			attrs := metric.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.Path),
				attribute.Int("status", recorder.status),
			)
			duration.Record(ctx, elapsed.Seconds(), attrs)

			span.SetAttributes(attribute.Int("http.status_code", recorder.status))
			if recorder.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(recorder.status))
				errorCounter.Add(ctx, 1, attrs)
			}
		})
	}
}

// TraceIDFrom returns the hex-encoded trace ID of the span carried in ctx, or
// an empty string when the request context carries none (the tracing
// middleware is not installed, or the span is invalid). RequestLogger uses
// this to put logs and traces under the same searchable identifier.
func TraceIDFrom(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.HasTraceID() {
		return ""
	}
	return spanContext.TraceID().String()
}

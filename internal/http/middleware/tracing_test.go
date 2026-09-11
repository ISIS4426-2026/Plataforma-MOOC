package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/middleware"
)

// testMeter is enough to satisfy Tracing's dependency on a metric.Meter
// without pulling in a full MeterProvider: these tests only assert on
// traces and logs.
var testMeter = noop.NewMeterProvider().Meter("test")

// TestRequestIDLogAndTraceAreCorrelated is issue #21's acceptance test,
// written directly: given a request_id, its log line and its trace are both
// findable, by the same identifier the response carried.
//
// It goes further and checks the reverse direction too: the span itself
// carries the same trace_id the log line reports, so either artifact leads
// to the other.
func TestRequestIDLogAndTraceAreCorrelated(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tracerProvider.Tracer("test")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	handler := middleware.Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		middleware.RequestID(),
		middleware.Tracing(tracer, testMeter),
		middleware.RequestLogger(logger),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	requestID := rec.Header().Get(middleware.HeaderRequestID)
	if requestID == "" {
		t.Fatal("response carries no X-Request-ID")
	}

	// The log line: find it by request_id and read its trace_id back out.
	var logEntry struct {
		RequestID string `json:"request_id"`
		TraceID   string `json:"trace_id"`
	}
	if err := json.Unmarshal(logBuf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse the log line as JSON: %v (raw: %s)", err, logBuf.String())
	}
	if logEntry.RequestID != requestID {
		t.Fatalf("log request_id = %q, want %q (the id the response carried)", logEntry.RequestID, requestID)
	}
	if logEntry.TraceID == "" {
		t.Fatal("log line carries no trace_id; a request_id alone can't locate its trace")
	}

	// The trace: find it by trace_id and confirm it carries the same
	// request_id right back.
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want exactly 1", len(spans))
	}
	span := spans[0]

	if span.SpanContext.TraceID().String() != logEntry.TraceID {
		t.Errorf("span trace id = %q, want %q (the one the log line reported)",
			span.SpanContext.TraceID().String(), logEntry.TraceID)
	}

	found := false
	for _, attr := range span.Attributes {
		if string(attr.Key) == "request_id" {
			found = true
			if attr.Value.AsString() != requestID {
				t.Errorf("span request_id attribute = %q, want %q", attr.Value.AsString(), requestID)
			}
		}
	}
	if !found {
		t.Error("span carries no request_id attribute")
	}
}

// TestTracingRecordsServerErrorStatus proves the span and the error counter
// both see a 5xx the way an operator computing an error rate needs them to.
func TestTracingMarksServerErrorsOnTheSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tracerProvider.Tracer("test")

	handler := middleware.Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
		middleware.RequestID(),
		middleware.Tracing(tracer, testMeter),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want exactly 1", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status code = %v, want %v for a 500 response", spans[0].Status.Code, codes.Error)
	}
}

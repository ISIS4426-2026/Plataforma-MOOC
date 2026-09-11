package observability_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/observability"
)

func TestSetupReturnsUsableProviders(t *testing.T) {
	obs, err := observability.Setup(observability.Config{
		ServiceName:    "test-service",
		ServiceVersion: "0.0.1",
		Environment:    "test",
	})
	if err != nil {
		t.Fatalf("Setup returned an error: %v", err)
	}
	t.Cleanup(func() {
		if err := obs.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown returned an error: %v", err)
		}
	})

	if obs.Tracer == nil {
		t.Error("Providers.Tracer is nil")
	}
	if obs.Meter == nil {
		t.Error("Providers.Meter is nil")
	}
	if obs.MetricsHandler == nil {
		t.Fatal("Providers.MetricsHandler is nil")
	}

	counter, err := obs.Meter.Int64Counter("test.counter")
	if err != nil {
		t.Fatalf("failed to create a counter from the returned meter: %v", err)
	}
	counter.Add(context.Background(), 3)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	obs.MetricsHandler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("metrics endpoint returned status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "test_counter") {
		t.Errorf("metrics output does not mention test_counter; got:\n%s", body)
	}
}

// Setup must be callable more than once without panicking on a duplicate
// Prometheus registration -- each call to observability.Setup (the API and
// the worker each call it once, from their own process) uses its own
// registry rather than the global default one.
func TestSetupIsCallableMoreThanOnce(t *testing.T) {
	for i := 0; i < 2; i++ {
		obs, err := observability.Setup(observability.Config{ServiceName: "test-service"})
		if err != nil {
			t.Fatalf("Setup call %d returned an error: %v", i, err)
		}
		if err := obs.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown call %d returned an error: %v", i, err)
		}
	}
}

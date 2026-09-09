package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

// stubPinger stands in for *sql.DB so the endpoint can be exercised with both a
// reachable and an unreachable database.
type stubPinger struct {
	err error
}

func (s stubPinger) PingContext(context.Context) error { return s.err }

func TestHealthHandlerReportsPassWhenDatabaseIsReachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	handler.NewHealthHandler(stubPinger{}).ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", res.StatusCode)
	}

	var body handler.HealthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if body.Status != "pass" {
		t.Errorf("expected status 'pass', got '%s'", body.Status)
	}
	if body.Services["database"] != "up" {
		t.Errorf("expected database 'up', got '%s'", body.Services["database"])
	}
}

// A failing dependency must surface as 503 so the Compose healthcheck and any
// load balancer stop routing traffic to this instance.
func TestHealthHandlerReportsFailWhenDatabaseIsUnreachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	handler.NewHealthHandler(stubPinger{err: errors.New("connection refused")}).ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 Service Unavailable, got %d", res.StatusCode)
	}

	var body handler.HealthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if body.Status != "fail" {
		t.Errorf("expected status 'fail', got '%s'", body.Status)
	}
	if body.Services["database"] != "down" {
		t.Errorf("expected database 'down', got '%s'", body.Services["database"])
	}
}

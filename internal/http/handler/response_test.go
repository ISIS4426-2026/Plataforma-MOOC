package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http/handler"
)

func TestRespondWithError(t *testing.T) {
	w := httptest.NewRecorder()

	handler.RespondWithError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid payload", map[string]string{"field": "email"})

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", res.StatusCode)
	}

	var errResp handler.ErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode ErrorResponse JSON: %v", err)
	}

	if errResp.Code != "INVALID_INPUT" {
		t.Errorf("expected code 'INVALID_INPUT', got '%s'", errResp.Code)
	}
	if errResp.Message != "Invalid payload" {
		t.Errorf("expected message 'Invalid payload', got '%s'", errResp.Message)
	}
}

func TestRespondWithETag_NotModified(t *testing.T) {
	etag := `"v1-hash-123"`
	req := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	req.Header.Set(handler.HeaderIfNoneMatch, etag)

	w := httptest.NewRecorder()

	handler.RespondWithETag(w, req, etag, map[string]string{"title": "Test Course"})

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotModified {
		t.Errorf("expected status 304 Not Modified, got %d", res.StatusCode)
	}
	if res.Header.Get(handler.HeaderETag) != etag {
		t.Errorf("expected ETag header '%s', got '%s'", etag, res.Header.Get(handler.HeaderETag))
	}
}

func TestRespondWithETag_OK(t *testing.T) {
	etag := `"v1-hash-123"`
	req := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	req.Header.Set(handler.HeaderIfNoneMatch, `"v1-old-hash"`)

	w := httptest.NewRecorder()

	handler.RespondWithETag(w, req, etag, map[string]string{"title": "Test Course"})

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", res.StatusCode)
	}
	if res.Header.Get(handler.HeaderETag) != etag {
		t.Errorf("expected ETag header '%s', got '%s'", etag, res.Header.Get(handler.HeaderETag))
	}
}

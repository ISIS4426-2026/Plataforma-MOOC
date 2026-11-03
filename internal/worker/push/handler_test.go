package push

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

type fakeProcessor struct {
	calls     int
	permanent bool
	err       error
}

func (f *fakeProcessor) ProcessPayload(_ context.Context, _ task.MediaProcessPayload) (bool, error) {
	f.calls++
	return f.permanent, f.err
}

type fakeDeduper struct {
	mu   sync.Mutex
	done map[string]bool
	down bool
}

func (d *fakeDeduper) IsProcessed(_ context.Context, key string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.down {
		return false, errors.New("store down")
	}
	return d.done[key], nil
}

func (d *fakeDeduper) MarkProcessed(_ context.Context, key string, _ time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done == nil {
		d.done = map[string]bool{}
	}
	d.done[key] = true
	return nil
}

func push(t *testing.T, h http.Handler, payload task.MediaProcessPayload) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(payload)
	body, _ := json.Marshal(map[string]any{
		"message":      map[string]any{"data": base64.StdEncoding.EncodeToString(data), "messageId": "m-1"},
		"subscription": "projects/demo/subscriptions/workers",
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pubsub/push", strings.NewReader(string(body))))
	return rec
}

var job = task.MediaProcessPayload{ResourceID: "res-1", StoragePath: "originals/a.mp4", IdempotencyKey: "media-originals/a.mp4"}

func TestPushAcknowledgesOnlyAfterTheWorkIsDone(t *testing.T) {
	p := &fakeProcessor{}
	rec := push(t, NewHandler(p, nil, 0, nil), job)
	if rec.Code != http.StatusNoContent || p.calls != 1 {
		t.Fatalf("code = %d, calls = %d; want 204 after one run", rec.Code, p.calls)
	}
}

func TestPushAnswers5xxOnATransientFailureSoPubSubRedelivers(t *testing.T) {
	p := &fakeProcessor{err: errors.New("database blip")}
	rec := push(t, NewHandler(p, nil, 0, nil), job)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500 (not acknowledged)", rec.Code)
	}
}

func TestPushAcknowledgesAPermanentFailureToStopRedelivery(t *testing.T) {
	p := &fakeProcessor{permanent: true, err: errors.New("unprocessable media")}
	if rec := push(t, NewHandler(p, nil, 0, nil), job); rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204: redelivering cannot fix a permanent failure", rec.Code)
	}
}

func TestPushAcknowledgesAMessageThatCannotBeRead(t *testing.T) {
	p := &fakeProcessor{}
	h := NewHandler(p, nil, 0, nil)
	for name, body := range map[string]string{
		"not an envelope": "garbage",
		"bad base64":      `{"message":{"data":"@@@","messageId":"m"}}`,
		"not a job":       `{"message":{"data":"` + base64.StdEncoding.EncodeToString([]byte("plain text")) + `","messageId":"m"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pubsub/push", strings.NewReader(body)))
			if rec.Code != http.StatusNoContent || p.calls != 0 {
				t.Fatalf("code = %d, calls = %d; want 204 and no work", rec.Code, p.calls)
			}
		})
	}
}

func TestPushIgnoresAnythingButPost(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(&fakeProcessor{}, nil, 0, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pubsub/push", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestPushAbsorbsARedeliveryOfAFinishedJob(t *testing.T) {
	p := &fakeProcessor{}
	d := &fakeDeduper{}
	h := NewHandler(p, d, 0, nil)

	if rec := push(t, h, job); rec.Code != http.StatusNoContent {
		t.Fatalf("first delivery: code = %d", rec.Code)
	}
	if rec := push(t, h, job); rec.Code != http.StatusNoContent {
		t.Fatalf("redelivery: code = %d", rec.Code)
	}
	if p.calls != 1 {
		t.Fatalf("processor ran %d times, want 1: a redelivery must not repeat the effects", p.calls)
	}
}

func TestPushKeepsWorkingWhenTheSharedStoreIsDown(t *testing.T) {
	p := &fakeProcessor{}
	rec := push(t, NewHandler(p, &fakeDeduper{down: true}, 0, nil), job)
	if rec.Code != http.StatusNoContent || p.calls != 1 {
		t.Fatalf("code = %d, calls = %d; the pipeline's own guard covers a store outage", rec.Code, p.calls)
	}
}

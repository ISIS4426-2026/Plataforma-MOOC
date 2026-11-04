package eventfn

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/media"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

type fakeProcessor struct {
	calls     int
	got       task.MediaProcessPayload
	permanent bool
	err       error
}

func (f *fakeProcessor) ProcessPayload(_ context.Context, p task.MediaProcessPayload) (bool, error) {
	f.calls++
	f.got = p
	return f.permanent, f.err
}

func event(t *testing.T, payload task.MediaProcessPayload, attrs map[string]string) []byte {
	t.Helper()
	data, _ := json.Marshal(payload)
	raw, _ := json.Marshal(map[string]any{"message": map[string]any{
		"data": base64.StdEncoding.EncodeToString(data), "attributes": attrs, "messageId": "m-1",
	}})
	return raw
}

func docJob() task.MediaProcessPayload {
	return task.MediaProcessPayload{
		ResourceID: "res-1", TaskType: media.TaskMalwareScan, StoragePath: "documents/a/b.pdf", IdempotencyKey: "media-documents/a/b.pdf",
	}
}

func TestHandleRunsTheDocumentJobAndConsumesTheEvent(t *testing.T) {
	p := &fakeProcessor{}
	if err := Handle(context.Background(), p, nil, event(t, docJob(), nil)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if p.calls != 1 || p.got.ResourceID != "res-1" || p.got.StoragePath != "documents/a/b.pdf" {
		t.Errorf("processor saw %+v (calls %d)", p.got, p.calls)
	}
}

func TestHandleAsksForRedeliveryOnATransientFailure(t *testing.T) {
	p := &fakeProcessor{err: errors.New("database unavailable")}
	err := Handle(context.Background(), p, nil, event(t, docJob(), nil))
	if err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("a transient failure must come back as an error so the trigger retries, got %v", err)
	}
}

func TestHandleConsumesAPermanentFailure(t *testing.T) {
	p := &fakeProcessor{permanent: true, err: errors.New("document is empty")}
	if err := Handle(context.Background(), p, nil, event(t, docJob(), nil)); err != nil {
		t.Fatalf("a permanent failure is recorded on the resource and then consumed, got %v", err)
	}
}

func TestHandleRefusesAnotherTaskType(t *testing.T) {
	job := docJob()
	job.TaskType = media.TaskHLSTranscode
	p := &fakeProcessor{}
	if err := Handle(context.Background(), p, nil, event(t, job, nil)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if p.calls != 0 {
		t.Error("the function must not run a task it does not own")
	}
}

func TestHandleDropsUnreadableEvents(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	p := &fakeProcessor{}

	cases := map[string][]byte{
		"not json":           []byte("{"),
		"data is not a job":  []byte(`{"message":{"data":"bm90LWpzb24="}}`),
		"data is not base64": []byte(`{"message":{"data":123}}`),
		"empty event":        []byte(`{}`),
	}
	for name, raw := range cases {
		if err := Handle(context.Background(), p, logger, raw); err != nil {
			t.Errorf("%s: an event that can never be read must be consumed, got %v", name, err)
		}
	}
	if p.calls != 0 {
		t.Error("nothing unreadable may reach the processor")
	}
}

func TestHandleTakesTheIdempotencyKeyFromTheAttributes(t *testing.T) {
	job := docJob()
	job.IdempotencyKey = ""
	p := &fakeProcessor{}
	if err := Handle(context.Background(), p, nil, event(t, job, map[string]string{"idempotency_key": "from-attr"})); err != nil {
		t.Fatal(err)
	}
	if p.got.IdempotencyKey != "from-attr" {
		t.Errorf("idempotency key = %q", p.got.IdempotencyKey)
	}
}

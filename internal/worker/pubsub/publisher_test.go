package pubsub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

const testTopic = "projects/demo/topics/media-jobs"

func TestPublisherSendsTheJobAndItsAttributes(t *testing.T) {
	var gotPath string
	var got publishRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		_, _ = w.Write([]byte(`{"messageIds":["1234567890"]}`))
	}))
	defer srv.Close()

	p := NewPublisherWithClient(testTopic, srv.URL, srv.Client(), nil)
	if err := p.EnqueueMediaProcessJob(context.Background(), "res-1", "hls_transcode", "originals/a.mp4", "media-originals/a.mp4"); err != nil {
		t.Fatalf("EnqueueMediaProcessJob: %v", err)
	}

	if want := "/v1/" + testTopic + ":publish"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(got.Messages))
	}
	data, err := base64.StdEncoding.DecodeString(got.Messages[0].Data)
	if err != nil {
		t.Fatalf("data is not base64: %v", err)
	}
	var payload task.MediaProcessPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("data is not a media job: %v", err)
	}
	if payload.ResourceID != "res-1" || payload.StoragePath != "originals/a.mp4" || payload.TaskType != "hls_transcode" {
		t.Errorf("payload = %+v", payload)
	}
	if got.Messages[0].Attributes[AttrIdempotencyKey] != "media-originals/a.mp4" {
		t.Errorf("idempotency attribute = %q", got.Messages[0].Attributes[AttrIdempotencyKey])
	}
}

func TestPublisherReportsAFailureAndAnEmptyAnswer(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"error status":    {http.StatusForbidden, `{"error":"denied"}`},
		"no message id":   {http.StatusOK, `{}`},
		"unreadable body": {http.StatusOK, `not json`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			p := NewPublisherWithClient(testTopic, srv.URL, srv.Client(), nil)
			if err := p.EnqueueMediaProcessJob(context.Background(), "r", "t", "k", "i"); err == nil {
				t.Fatal("expected an error: a job that was not accepted must not look published")
			}
		})
	}
}

func TestNewPublisherRejectsAShortTopicName(t *testing.T) {
	if _, err := NewPublisher(context.Background(), "media-jobs", nil); err == nil ||
		!strings.Contains(err.Error(), "full name") {
		t.Fatalf("err = %v, want a complaint about the topic name", err)
	}
}

func TestNewPublisherBuildsWithApplicationDefaultCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adc.json")
	creds := `{"type":"authorized_user","client_id":"id","client_secret":"secret","refresh_token":"token"}`
	if err := os.WriteFile(path, []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)

	p, err := NewPublisher(context.Background(), testTopic, nil)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	if p.topic != testTopic || p.endpoint != defaultEndpoint {
		t.Errorf("publisher = %+v", p)
	}
}

func TestNewPublisherFailsWithoutCredentials(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "missing.json"))
	if _, err := NewPublisher(context.Background(), testTopic, nil); err == nil {
		t.Fatal("unreadable credentials must be an error")
	}
}

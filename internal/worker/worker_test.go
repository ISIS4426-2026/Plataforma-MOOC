package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

type logEntry struct {
	Level    string `json:"level"`
	Msg      string `json:"msg"`
	ID       string `json:"id"`
	Tipo     string `json:"tipo"`
	Estado   string `json:"estado"`
	Duracion string `json:"duración"`
	Error    string `json:"error"`
}

func TestLoggingMiddleware_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mw := LoggingMiddleware(logger)

	testTask, err := task.NewTestPingTask("middleware test")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	dummyHandler := asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})

	ctx := context.Background()
	wrapped := mw(dummyHandler)
	err = wrapped.ProcessTask(ctx, testTask)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines (started and completed), got %d: %s", len(lines), buf.String())
	}

	var startLog logEntry
	if err := json.Unmarshal(lines[0], &startLog); err != nil {
		t.Fatalf("failed to parse start log line: %v", err)
	}
	if startLog.Tipo != "test:ping" || startLog.Estado != "started" {
		t.Errorf("start log mismatch: %+v", startLog)
	}

	var completeLog logEntry
	if err := json.Unmarshal(lines[1], &completeLog); err != nil {
		t.Fatalf("failed to parse complete log line: %v", err)
	}
	if completeLog.Tipo != "test:ping" || completeLog.Estado != "completed" {
		t.Errorf("complete log mismatch: %+v", completeLog)
	}
	if completeLog.Duracion == "" {
		t.Errorf("expected non-empty 'duración' field in log")
	}
}

func TestLoggingMiddleware_Failure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mw := LoggingMiddleware(logger)

	testTask, err := task.NewTestPingTask("failure test")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	expectedErr := errors.New("simulated task error")
	dummyHandler := asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		return expectedErr
	})

	ctx := context.Background()
	wrapped := mw(dummyHandler)
	err = wrapped.ProcessTask(ctx, testTask)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}

	var failLog logEntry
	if err := json.Unmarshal(lines[1], &failLog); err != nil {
		t.Fatalf("failed to parse fail log line: %v", err)
	}
	if failLog.Tipo != "test:ping" || failLog.Estado != "failed" || failLog.Error != expectedErr.Error() {
		t.Errorf("fail log mismatch: %+v", failLog)
	}
	if failLog.Duracion == "" {
		t.Errorf("expected non-empty 'duración' field in fail log")
	}
}

func TestWorkerEngine_MultiReplicaConcurrency(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		RedisURL:    "localhost:6379",
	}

	// Spin up 2 worker replicas to test concurrent stateless execution against Redis
	w1, err := NewWorkerEngine(cfg, WithConcurrency(2))
	if err != nil {
		t.Fatalf("failed to create worker 1: %v", err)
	}

	w2, err := NewWorkerEngine(cfg, WithConcurrency(2))
	if err != nil {
		t.Fatalf("failed to create worker 2: %v", err)
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())

	errCh1 := make(chan error, 1)
	errCh2 := make(chan error, 1)

	go func() {
		errCh1 <- w1.Start(ctx1)
	}()

	go func() {
		errCh2 <- w2.Start(ctx2)
	}()

	time.Sleep(100 * time.Millisecond)

	// Enqueue test task via client
	client, err := NewClient(cfg.RedisURL)
	if err != nil {
		t.Fatalf("failed to create queue client: %v", err)
	}
	defer client.Close()

	info, err := client.EnqueueTestPing(context.Background(), "multi-replica test job")
	if err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}
	if info.ID == "" {
		t.Errorf("expected non-empty task ID")
	}

	time.Sleep(200 * time.Millisecond)

	// Stop both replicas cleanly
	cancel1()
	cancel2()

	if err := <-errCh1; err != nil {
		t.Errorf("replica 1 error: %v", err)
	}
	if err := <-errCh2; err != nil {
		t.Errorf("replica 2 error: %v", err)
	}
}

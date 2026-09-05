package task

import (
	"encoding/json"
	"testing"
)

func TestNewTestPingTask(t *testing.T) {
	msg := "Hello Asynq Worker"
	task, err := NewTestPingTask(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Type() != TypeTestPing {
		t.Errorf("expected type %s, got %s", TypeTestPing, task.Type())
	}

	var payload TestPingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.Message != msg {
		t.Errorf("expected message %q, got %q", msg, payload.Message)
	}

	if payload.Timestamp.IsZero() {
		t.Errorf("expected non-zero timestamp")
	}
}

func TestNewTestPingTask_DefaultMessage(t *testing.T) {
	task, err := NewTestPingTask("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload TestPingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.Message != "ping test job" {
		t.Errorf("expected default message 'ping test job', got %q", payload.Message)
	}
}

func TestNewMediaProcessTask(t *testing.T) {
	resourceID := "res-123"
	taskType := "hls_transcode"
	storagePath := "courses/video.mp4"

	task, err := NewMediaProcessTask(resourceID, taskType, storagePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Type() != TypeMediaProcess {
		t.Errorf("expected type %s, got %s", TypeMediaProcess, task.Type())
	}

	var payload MediaProcessPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.ResourceID != resourceID || payload.TaskType != taskType || payload.StoragePath != storagePath {
		t.Errorf("payload mismatch: %+v", payload)
	}
}

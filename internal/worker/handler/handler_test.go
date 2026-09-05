package handler

import (
	"context"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

func TestHandleTestPingTask(t *testing.T) {
	pingTask, err := task.NewTestPingTask("unit test message")
	if err != nil {
		t.Fatalf("failed to create test ping task: %v", err)
	}

	ctx := context.Background()
	if err := HandleTestPingTask(ctx, pingTask); err != nil {
		t.Errorf("HandleTestPingTask returned error: %v", err)
	}
}

func TestHandleMediaProcessTask(t *testing.T) {
	mediaTask, err := task.NewMediaProcessTask("res-999", "hls_transcode", "videos/intro.mp4")
	if err != nil {
		t.Fatalf("failed to create media task: %v", err)
	}

	ctx := context.Background()
	if err := HandleMediaProcessTask(ctx, mediaTask); err != nil {
		t.Errorf("HandleMediaProcessTask returned error: %v", err)
	}
}

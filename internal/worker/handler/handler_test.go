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

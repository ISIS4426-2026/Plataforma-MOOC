package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// HandleTestPingTask processes a test ping task (Acceptance Criterion 2).
func HandleTestPingTask(ctx context.Context, t *asynq.Task) error {
	var payload task.TestPingPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal test ping payload: %w", err)
	}

	taskID, _ := asynq.GetTaskID(ctx)
	slog.InfoContext(ctx, "[Worker] Executing test ping job",
		slog.String("task_id", taskID),
		slog.String("message", payload.Message),
		slog.Time("enqueued_at", payload.Timestamp),
	)

	return nil
}

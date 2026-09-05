package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// HandleMediaProcessTask handles media processing tasks (HLS transcoding, malware scan, PDF conversion).
func HandleMediaProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload task.MediaProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		// Fallback for raw text payload if passed directly
		return ProcessMediaTask(ctx, string(t.Payload()), t.Payload())
	}

	taskID, _ := asynq.GetTaskID(ctx)
	slog.InfoContext(ctx, "[Worker] Processing media task",
		slog.String("task_id", taskID),
		slog.String("resource_id", payload.ResourceID),
		slog.String("task_type", payload.TaskType),
		slog.String("storage_path", payload.StoragePath),
	)

	return ProcessMediaTask(ctx, payload.TaskType, t.Payload())
}

// ProcessMediaTask simulates asynchronous media processing (HLS transcoding, malware scan, PDF conversion).
func ProcessMediaTask(ctx context.Context, taskType string, payload []byte) error {
	log.Printf("[Worker] Processing task type: %s, payload size: %d bytes\n", taskType, len(payload))
	// Placeholder for async transcoding, malware scan, and PDF conversion logic
	if taskType == "" {
		return fmt.Errorf("task type cannot be empty")
	}
	return nil
}

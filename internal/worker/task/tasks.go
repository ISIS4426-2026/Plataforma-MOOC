package task

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const (
	// TypeTestPing is the task type for test ping jobs.
	TypeTestPing = "test:ping"

	// TypeMediaProcess is the task type for media processing jobs.
	TypeMediaProcess = "media:process"
)

// TestPingPayload defines the payload structure for test ping jobs.
type TestPingPayload struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// MediaProcessPayload defines the payload structure for media processing jobs.
type MediaProcessPayload struct {
	ResourceID  string `json:"resource_id"`
	TaskType    string `json:"task_type"` // e.g., "hls_transcode", "malware_scan", "pdf_conversion"
	StoragePath string `json:"storage_path"`
}

// NewTestPingTask creates a new test ping task with default 3 retries.
func NewTestPingTask(msg string) (*asynq.Task, error) {
	if msg == "" {
		msg = "ping test job"
	}
	payload, err := json.Marshal(TestPingPayload{
		Message:   msg,
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal test ping payload: %w", err)
	}
	return asynq.NewTask(TypeTestPing, payload, asynq.MaxRetry(3)), nil
}

// NewMediaProcessTask creates a new media processing task with default 3 retries.
func NewMediaProcessTask(resourceID, taskType, storagePath string) (*asynq.Task, error) {
	payload, err := json.Marshal(MediaProcessPayload{
		ResourceID:  resourceID,
		TaskType:    taskType,
		StoragePath: storagePath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal media process payload: %w", err)
	}
	return asynq.NewTask(TypeMediaProcess, payload, asynq.MaxRetry(3)), nil
}

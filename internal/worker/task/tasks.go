package task

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TypeTestPing is the task type for test ping jobs.
	TypeTestPing = "test:ping"

	// TypeMediaProcess is the task type for media processing jobs.
	TypeMediaProcess = "media:process"
)

// TaskOptions holds optional parameters for creating background tasks.
type TaskOptions struct {
	MaxRetry       int
	IdempotencyKey string
}

// Option defines a functional option for configuring a task.
type Option func(*TaskOptions)

// WithMaxRetry overrides the default max retries for a task.
func WithMaxRetry(maxRetry int) Option {
	return func(opts *TaskOptions) {
		opts.MaxRetry = maxRetry
	}
}

// WithIdempotencyKey sets a specific idempotency key for a task.
func WithIdempotencyKey(key string) Option {
	return func(opts *TaskOptions) {
		opts.IdempotencyKey = key
	}
}

// TestPingPayload defines the payload structure for test ping jobs.
type TestPingPayload struct {
	Message        string    `json:"message"`
	Timestamp      time.Time `json:"timestamp"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
}

// MediaProcessPayload defines the payload structure for media processing jobs.
type MediaProcessPayload struct {
	ResourceID     string `json:"resource_id"`
	TaskType       string `json:"task_type"` // e.g., "hls_transcode", "malware_scan", "pdf_conversion"
	StoragePath    string `json:"storage_path"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// NewTestPingTask creates a new test ping task with default 3 retries.
func NewTestPingTask(msg string, opts ...Option) (*asynq.Task, error) {
	if msg == "" {
		msg = "ping test job"
	}
	tOpts := &TaskOptions{
		MaxRetry: 3,
	}
	for _, opt := range opts {
		opt(tOpts)
	}

	if tOpts.IdempotencyKey == "" {
		tOpts.IdempotencyKey = uuid.NewString()
	}

	payload, err := json.Marshal(TestPingPayload{
		Message:        msg,
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: tOpts.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal test ping payload: %w", err)
	}

	asynqOpts := []asynq.Option{
		asynq.MaxRetry(tOpts.MaxRetry),
		asynq.TaskID(tOpts.IdempotencyKey),
	}

	return asynq.NewTask(TypeTestPing, payload, asynqOpts...), nil
}

// NewMediaProcessTask creates a new media processing task with default 3 retries.
func NewMediaProcessTask(resourceID, taskType, storagePath string, opts ...Option) (*asynq.Task, error) {
	tOpts := &TaskOptions{
		MaxRetry: 3,
	}
	for _, opt := range opts {
		opt(tOpts)
	}

	if tOpts.IdempotencyKey == "" {
		tOpts.IdempotencyKey = fmt.Sprintf("media-%s-%s", resourceID, taskType)
	}

	payload, err := json.Marshal(MediaProcessPayload{
		ResourceID:     resourceID,
		TaskType:       taskType,
		StoragePath:    storagePath,
		IdempotencyKey: tOpts.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal media process payload: %w", err)
	}

	asynqOpts := []asynq.Option{
		asynq.MaxRetry(tOpts.MaxRetry),
		asynq.TaskID(tOpts.IdempotencyKey),
	}

	return asynq.NewTask(TypeMediaProcess, payload, asynqOpts...), nil
}

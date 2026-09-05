package worker

import (
	"context"
	"fmt"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// Client is a wrapper around asynq.Client for enqueuing background tasks to Redis.
type Client struct {
	client *asynq.Client
}

// NewClient creates a new Client configured with the specified Redis URL.
func NewClient(redisURL string) (*Client, error) {
	opt, err := ParseRedisOpt(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis configuration for queue client: %w", err)
	}
	c := asynq.NewClient(opt)
	return &Client{client: c}, nil
}

// NewClientWithAsynqClient creates a Client wrapping an existing *asynq.Client instance.
func NewClientWithAsynqClient(c *asynq.Client) *Client {
	return &Client{client: c}
}

// Close closes the underlying Asynq client connection.
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Enqueue enqueues an arbitrary Asynq task.
func (c *Client) Enqueue(ctx context.Context, t *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := c.client.EnqueueContext(ctx, t, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task %s: %w", t.Type(), err)
	}
	return info, nil
}

// EnqueueTestPing enqueues a test ping task (Acceptance Criterion 2).
func (c *Client) EnqueueTestPing(ctx context.Context, message string) (*asynq.TaskInfo, error) {
	t, err := task.NewTestPingTask(message)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, t)
}

// EnqueueMediaProcess enqueues a media processing task.
func (c *Client) EnqueueMediaProcess(ctx context.Context, resourceID, taskType, storagePath string) (*asynq.TaskInfo, error) {
	t, err := task.NewMediaProcessTask(resourceID, taskType, storagePath)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, t)
}

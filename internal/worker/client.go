package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Client is a wrapper around asynq.Client for enqueuing background tasks and managing the DLQ.
type Client struct {
	client   *asynq.Client
	redisURL string
}

// NewClient creates a new Client configured with the specified Redis URL.
func NewClient(redisURL string) (*Client, error) {
	opt, err := ParseRedisOpt(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis configuration for queue client: %w", err)
	}
	c := asynq.NewClient(opt)
	return &Client{
		client:   c,
		redisURL: redisURL,
	}, nil
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

// EnqueueTestPing enqueues a test ping task.
func (c *Client) EnqueueTestPing(ctx context.Context, message string, opts ...task.Option) (*asynq.TaskInfo, error) {
	t, err := task.NewTestPingTask(message, opts...)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, t)
}

// EnqueueMediaProcess enqueues a media processing task.
func (c *Client) EnqueueMediaProcess(ctx context.Context, resourceID, taskType, storagePath string, opts ...task.Option) (*asynq.TaskInfo, error) {
	t, err := task.NewMediaProcessTask(resourceID, taskType, storagePath, opts...)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, t)
}

// Inspector returns a new *asynq.Inspector connected to Redis for queue management.
func (c *Client) Inspector() (*asynq.Inspector, error) {
	opt, err := ParseRedisOpt(c.redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis configuration for inspector: %w", err)
	}
	return asynq.NewInspector(opt), nil
}

// ListDLQTasks returns all tasks currently in the Dead-Letter Queue (archived state) for the given queue.
func (c *Client) ListDLQTasks(ctx context.Context, queue string) ([]*asynq.TaskInfo, error) {
	if queue == "" {
		queue = "default"
	}
	inspector, err := c.Inspector()
	if err != nil {
		return nil, err
	}
	defer inspector.Close()

	tasks, err := inspector.ListArchivedTasks(queue)
	if err != nil {
		return nil, fmt.Errorf("failed to list archived tasks in queue %s: %w", queue, err)
	}
	return tasks, nil
}

// GetDLQTask fetches information for a specific task in the Dead-Letter Queue (archived state).
func (c *Client) GetDLQTask(ctx context.Context, queue string, taskID string) (*asynq.TaskInfo, error) {
	if queue == "" {
		queue = "default"
	}
	inspector, err := c.Inspector()
	if err != nil {
		return nil, err
	}
	defer inspector.Close()

	info, err := inspector.GetTaskInfo(queue, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task info for %s in queue %s: %w", taskID, queue, err)
	}
	return info, nil
}

// ReenqueueFromDLQ moves a task from the Dead-Letter Queue (archived state) back to pending state for execution,
// preserving its original TaskID and Idempotency Key.
func (c *Client) ReenqueueFromDLQ(ctx context.Context, queue string, taskID string) error {
	if queue == "" {
		queue = "default"
	}
	inspector, err := c.Inspector()
	if err != nil {
		return err
	}
	defer inspector.Close()

	var lastErr error
	for i := 0; i < 5; i++ {
		err = inspector.RunTask(queue, taskID)
		if err == nil {
			return nil
		}

		// If Asynq recoverer left task state hash as 'active' after retry exhaustion,
		// force state to 'archived' in Redis so inspector.RunTask can transition it to pending.
		opt, rerr := redis.ParseURL(c.redisURL)
		if rerr != nil {
			opt = &redis.Options{Addr: c.redisURL}
		}
		rClient := redis.NewClient(opt)
		taskKey := fmt.Sprintf("asynq:{%s}:t:%s", queue, taskID)
		_ = rClient.HSet(ctx, taskKey, "state", "archived").Err()
		_ = rClient.Close()

		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("failed to re-enqueue task %s from DLQ in queue %s: %w", taskID, queue, lastErr)
}

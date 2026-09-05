package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func flushRedis(t *testing.T, redisURL string) {
	t.Helper()
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		opt = &redis.Options{Addr: redisURL}
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	_ = rdb.FlushDB(context.Background()).Err()
}

func waitForTaskState(client *Client, queue, taskID string, expectedState asynq.TaskState, timeout time.Duration) (*asynq.TaskInfo, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, err := client.GetDLQTask(context.Background(), queue, taskID)
		if err == nil && info.State == expectedState {
			return info, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return client.GetDLQTask(context.Background(), queue, taskID)
}

func TestWorker_DLQ_3Retries_And_Alert(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		RedisURL:    "localhost:6379",
	}

	flushRedis(t, cfg.RedisURL)
	defer flushRedis(t, cfg.RedisURL)

	queueName := fmt.Sprintf("dlq_q1_%d", time.Now().UnixNano())
	taskTypeFailing := fmt.Sprintf("test:failing_job_%d", time.Now().UnixNano())
	idempotencyKey := fmt.Sprintf("dlq-test-key-%d", time.Now().UnixNano())

	alertChan := make(chan string, 1)
	alertHandler := func(ctx context.Context, task *asynq.Task, err error) {
		taskID, _ := asynq.GetTaskID(ctx)
		alertChan <- taskID
	}

	fastBackoff := func(n int, err error, task *asynq.Task) time.Duration {
		return 20 * time.Millisecond
	}

	memStore := NewMemoryIdempotencyStore()

	engine, err := NewWorkerEngine(cfg,
		WithConcurrency(2),
		WithQueues(map[string]int{queueName: 1}),
		WithRetryDelayFunc(fastBackoff),
		WithDelayedTaskCheckInterval(20*time.Millisecond),
		WithTaskCheckInterval(20*time.Millisecond),
		WithAlertHandler(alertHandler),
		WithIdempotencyStore(memStore),
	)
	if err != nil {
		t.Fatalf("failed to create worker engine: %v", err)
	}

	var executionCount int32
	engine.mux.HandleFunc(taskTypeFailing, func(ctx context.Context, t *asynq.Task) error {
		atomic.AddInt32(&executionCount, 1)
		time.Sleep(2 * time.Millisecond)
		return errors.New("simulated persistent failure")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engineDone := make(chan error, 1)
	go func() {
		engineDone <- engine.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(cfg.RedisURL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	payload, _ := json.Marshal(task.TestPingPayload{
		Message:        "failing job payload",
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	})
	failingTask := asynq.NewTask(taskTypeFailing, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.TaskID(idempotencyKey),
		asynq.Queue(queueName),
	)

	info, err := client.Enqueue(context.Background(), failingTask)
	if err != nil {
		t.Fatalf("failed to enqueue failing task: %v", err)
	}
	if info.ID != idempotencyKey {
		t.Errorf("expected task ID %s, got %s", idempotencyKey, info.ID)
	}

	select {
	case alertedTaskID := <-alertChan:
		if alertedTaskID != idempotencyKey {
			t.Errorf("alert task ID mismatch: got %s, want %s", alertedTaskID, idempotencyKey)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for DLQ alert signal")
	}

	time.Sleep(100 * time.Millisecond)

	dlqTasks, err := client.ListDLQTasks(context.Background(), queueName)
	if err != nil {
		t.Fatalf("failed to list DLQ tasks: %v", err)
	}
	if len(dlqTasks) != 1 {
		t.Fatalf("expected 1 task in DLQ, got %d", len(dlqTasks))
	}
	dlqTask := dlqTasks[0]

	cancel()
	<-engineDone

	totalExecs := atomic.LoadInt32(&executionCount)
	if totalExecs != 4 {
		t.Errorf("expected 4 total executions (1 initial + 3 retries), got %d", totalExecs)
	}

	if dlqTask.State != asynq.TaskStateArchived {
		t.Errorf("expected task state 'archived' (DLQ), got %v", dlqTask.State)
	}
	if dlqTask.Retried != 3 {
		t.Errorf("expected 3 retries in DLQ, got %d", dlqTask.Retried)
	}
	if dlqTask.ID != idempotencyKey {
		t.Errorf("expected DLQ task ID %s, got %s", idempotencyKey, dlqTask.ID)
	}
}

func TestWorker_DLQ_Reenqueue_And_Idempotency(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		RedisURL:    "localhost:6379",
	}

	flushRedis(t, cfg.RedisURL)
	defer flushRedis(t, cfg.RedisURL)

	queueName := fmt.Sprintf("dlq_q2_%d", time.Now().UnixNano())
	taskTypeRecoverable := fmt.Sprintf("test:recoverable_job_%d", time.Now().UnixNano())
	idempotencyKey := fmt.Sprintf("reenqueue-key-%d", time.Now().UnixNano())

	alertChan := make(chan string, 1)
	alertHandler := func(ctx context.Context, task *asynq.Task, err error) {
		taskID, _ := asynq.GetTaskID(ctx)
		alertChan <- taskID
	}

	fastBackoff := func(n int, err error, task *asynq.Task) time.Duration {
		return 20 * time.Millisecond
	}

	memStore := NewMemoryIdempotencyStore()

	var attemptCount int32
	var sideEffectsExecuted int32

	handlerFunc := func(ctx context.Context, t *asynq.Task) error {
		current := atomic.AddInt32(&attemptCount, 1)
		time.Sleep(2 * time.Millisecond)
		if current == 4 {
			return fmt.Errorf("%w: temporary failure", asynq.SkipRetry)
		}
		if current < 4 {
			return errors.New("temporary failure")
		}
		atomic.AddInt32(&sideEffectsExecuted, 1)
		return nil
	}

	engine, err := NewWorkerEngine(cfg,
		WithConcurrency(2),
		WithQueues(map[string]int{queueName: 1}),
		WithRetryDelayFunc(fastBackoff),
		WithDelayedTaskCheckInterval(20*time.Millisecond),
		WithTaskCheckInterval(20*time.Millisecond),
		WithAlertHandler(alertHandler),
		WithIdempotencyStore(memStore),
	)
	if err != nil {
		t.Fatalf("failed to create worker engine: %v", err)
	}
	engine.mux.HandleFunc(taskTypeRecoverable, handlerFunc)

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	engineDone1 := make(chan error, 1)
	go func() {
		engineDone1 <- engine.Start(ctx1)
	}()

	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(cfg.RedisURL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	payload, _ := json.Marshal(task.TestPingPayload{
		Message:        "recoverable job payload",
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	})
	recTask := asynq.NewTask(taskTypeRecoverable, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.TaskID(idempotencyKey),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), recTask)
	if err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	select {
	case alertedKey := <-alertChan:
		if alertedKey != idempotencyKey {
			t.Errorf("alert key mismatch: got %s, want %s", alertedKey, idempotencyKey)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for DLQ alert")
	}

	// Give Asynq 100ms to complete archiveCmd Redis script
	time.Sleep(100 * time.Millisecond)

	// Stop engine1 gracefully
	cancel1()
	if err := <-engineDone1; err != nil {
		t.Fatalf("engine 1 shutdown error: %v", err)
	}

	dlqTasks, err := client.ListDLQTasks(context.Background(), queueName)
	if err != nil {
		t.Fatalf("failed to list DLQ tasks: %v", err)
	}
	if len(dlqTasks) != 1 {
		t.Fatalf("expected 1 task in DLQ, got %d", len(dlqTasks))
	}
	dlqTask := dlqTasks[0]
	if dlqTask.ID != idempotencyKey {
		t.Errorf("DLQ task ID changed: got %s, want %s", dlqTask.ID, idempotencyKey)
	}

	// Re-enqueue task from DLQ
	err = client.ReenqueueFromDLQ(context.Background(), queueName, idempotencyKey)
	if err != nil {
		t.Fatalf("failed to re-enqueue task from DLQ: %v", err)
	}

	// Start engine 2 with the same idempotency store to process the re-enqueued task
	engine2, err := NewWorkerEngine(cfg,
		WithConcurrency(2),
		WithQueues(map[string]int{queueName: 1}),
		WithRetryDelayFunc(fastBackoff),
		WithDelayedTaskCheckInterval(20*time.Millisecond),
		WithTaskCheckInterval(20*time.Millisecond),
		WithIdempotencyStore(memStore),
	)
	if err != nil {
		t.Fatalf("failed to create worker engine 2: %v", err)
	}
	engine2.mux.HandleFunc(taskTypeRecoverable, handlerFunc)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	engineDone2 := make(chan error, 1)
	go func() {
		engineDone2 <- engine2.Start(ctx2)
	}()

	// Wait for engine2 to process re-enqueued task
	time.Sleep(300 * time.Millisecond)

	if atomic.LoadInt32(&sideEffectsExecuted) != 1 {
		t.Errorf("expected side effect to execute 1 time after recovery, got %d", atomic.LoadInt32(&sideEffectsExecuted))
	}

	// Dispatch duplicate task with SAME idempotency key
	duplicateTask := asynq.NewTask(taskTypeRecoverable, payload,
		asynq.MaxRetry(3),
		asynq.TaskID(idempotencyKey),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), duplicateTask)
	if err != nil {
		t.Fatalf("failed to enqueue duplicate task: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	finalSideEffects := atomic.LoadInt32(&sideEffectsExecuted)
	if finalSideEffects != 1 {
		t.Errorf("idempotency check failed: side effects executed %d times; expected exactly 1", finalSideEffects)
	}

	cancel2()
	<-engineDone2
}

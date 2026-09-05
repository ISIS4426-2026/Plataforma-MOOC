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

// TestIdempotencyStore_Memory verifies MemoryIdempotencyStore behavior, including TTL expiration.
func TestIdempotencyStore_Memory(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryIdempotencyStore()

	key := "test-mem-key-1"
	processed, err := store.IsProcessed(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error checking IsProcessed: %v", err)
	}
	if processed {
		t.Errorf("expected key to not be processed initially")
	}

	if err := store.MarkProcessed(ctx, key, 100*time.Millisecond); err != nil {
		t.Fatalf("failed to mark key as processed: %v", err)
	}

	processed, err = store.IsProcessed(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error checking IsProcessed: %v", err)
	}
	if !processed {
		t.Errorf("expected key to be marked as processed")
	}

	time.Sleep(150 * time.Millisecond)
	processed, err = store.IsProcessed(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error checking IsProcessed after TTL: %v", err)
	}
	if processed {
		t.Errorf("expected key to expire after TTL")
	}
}

// TestIdempotencyStore_Redis verifies RedisIdempotencyStore behavior against a live Redis instance.
func TestIdempotencyStore_Redis(t *testing.T) {
	redisURL := "localhost:6379"
	flushRedis(t, redisURL)
	defer flushRedis(t, redisURL)

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		opt = &redis.Options{Addr: redisURL}
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	ctx := context.Background()
	store := NewRedisIdempotencyStore(rdb)

	key := "test-redis-key-1"
	processed, err := store.IsProcessed(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error checking IsProcessed in Redis: %v", err)
	}
	if processed {
		t.Errorf("expected key to not be processed initially")
	}

	if err := store.MarkProcessed(ctx, key, 1*time.Second); err != nil {
		t.Fatalf("failed to mark key as processed in Redis: %v", err)
	}

	processed, err = store.IsProcessed(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error checking IsProcessed in Redis: %v", err)
	}
	if !processed {
		t.Errorf("expected key to be marked as processed in Redis")
	}
}

// Acceptance Criterion 1:
// "Prueba que envía el mismo evento/job dos veces y confirma un solo efecto persistido."
func TestIdempotency_DuplicateEvent_SinglePersistedEffect(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		RedisURL:    "localhost:6379",
	}

	flushRedis(t, cfg.RedisURL)
	defer flushRedis(t, cfg.RedisURL)

	queueName := fmt.Sprintf("dup_q_%d", time.Now().UnixNano())
	taskType := fmt.Sprintf("test:duplicate_job_%d", time.Now().UnixNano())
	idempotencyKey := fmt.Sprintf("dup-event-key-%d", time.Now().UnixNano())

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
		WithIdempotencyStore(memStore),
	)
	if err != nil {
		t.Fatalf("failed to create worker engine: %v", err)
	}

	var persistedEffectsCount int32
	var totalHandlerExecutions int32

	engine.mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
		atomic.AddInt32(&totalHandlerExecutions, 1)
		// Simulate atomic database insert / persisted side effect
		atomic.AddInt32(&persistedEffectsCount, 1)
		return nil
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
		Message:        "initial event payload",
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	})

	// Dispatch Job #1
	task1 := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.TaskID(idempotencyKey),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), task1)
	if err != nil {
		t.Fatalf("failed to enqueue task 1: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&persistedEffectsCount) != 1 {
		t.Fatalf("expected 1 persisted effect after first delivery, got %d", atomic.LoadInt32(&persistedEffectsCount))
	}

	// Dispatch Job #2 (DUPLICATE delivery of the exact same event/job with the same Idempotency-Key)
	duplicateTaskID := fmt.Sprintf("%s-dup", idempotencyKey)
	task2 := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.TaskID(duplicateTaskID),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), task2)
	if err != nil {
		t.Fatalf("failed to enqueue task 2 (duplicate): %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verification of Criterion 1:
	// Total handler executions must be 1, and total persisted effects must remain 1.
	finalPersistedCount := atomic.LoadInt32(&persistedEffectsCount)
	finalHandlerExecs := atomic.LoadInt32(&totalHandlerExecutions)

	if finalHandlerExecs != 1 {
		t.Errorf("Criterion 1 Failure: handler executed %d times; expected exactly 1", finalHandlerExecs)
	}
	if finalPersistedCount != 1 {
		t.Errorf("Criterion 1 Failure: expected 1 persisted effect, got %d", finalPersistedCount)
	}

	cancel()
	<-engineDone
}

// Acceptance Criterion 2:
// "Prueba que reencola con la misma Idempotency-Key y confirma que no se duplica el resultado."
func TestIdempotency_ReenqueueWithSameKey_NoDuplicateResult(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		RedisURL:    "localhost:6379",
	}

	flushRedis(t, cfg.RedisURL)
	defer flushRedis(t, cfg.RedisURL)

	queueName := fmt.Sprintf("reenq_q_%d", time.Now().UnixNano())
	taskType := fmt.Sprintf("test:reenqueue_job_%d", time.Now().UnixNano())
	idempotencyKey := fmt.Sprintf("reenq-key-%d", time.Now().UnixNano())

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

		// Fail first 4 attempts to trigger DLQ (1 initial + 3 retries)
		if current == 4 {
			return fmt.Errorf("%w: temporary failure forcing DLQ", asynq.SkipRetry)
		}
		if current < 4 {
			return errors.New("temporary failure forcing retry")
		}

		// Execution on recovery after re-enqueue from DLQ
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
	engine.mux.HandleFunc(taskType, handlerFunc)

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
		Message:        "reenqueue test payload",
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	})

	initialTask := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.TaskID(idempotencyKey),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), initialTask)
	if err != nil {
		t.Fatalf("failed to enqueue initial task: %v", err)
	}

	// Wait for task to exhaust retries and land in DLQ
	select {
	case alertedKey := <-alertChan:
		if alertedKey != idempotencyKey {
			t.Errorf("alert key mismatch: got %s, want %s", alertedKey, idempotencyKey)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for DLQ alert")
	}

	time.Sleep(100 * time.Millisecond)

	cancel1()
	<-engineDone1

	// Verify task is in DLQ
	dlqTasks, err := client.ListDLQTasks(context.Background(), queueName)
	if err != nil {
		t.Fatalf("failed to list DLQ tasks: %v", err)
	}
	if len(dlqTasks) != 1 {
		t.Fatalf("expected 1 task in DLQ, got %d", len(dlqTasks))
	}

	// Re-enqueue task from DLQ with the SAME Idempotency-Key
	err = client.ReenqueueFromDLQ(context.Background(), queueName, idempotencyKey)
	if err != nil {
		t.Fatalf("failed to re-enqueue task from DLQ: %v", err)
	}

	// Start Engine 2 to process re-enqueued task using the same idempotency store
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
	engine2.mux.HandleFunc(taskType, handlerFunc)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	engineDone2 := make(chan error, 1)
	go func() {
		engineDone2 <- engine2.Start(ctx2)
	}()

	time.Sleep(300 * time.Millisecond)

	if atomic.LoadInt32(&sideEffectsExecuted) != 1 {
		t.Fatalf("expected side effect to execute 1 time upon recovery, got %d", atomic.LoadInt32(&sideEffectsExecuted))
	}

	// Re-enqueue AGAIN (or send duplicate with same Idempotency-Key)
	duplicateReenqueueTask := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.TaskID(fmt.Sprintf("%s-dup-reenq", idempotencyKey)),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), duplicateReenqueueTask)
	if err != nil {
		t.Fatalf("failed to enqueue duplicate reenqueue task: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Verification of Criterion 2:
	// Re-enqueueing with the same Idempotency-Key MUST NOT duplicate the result.
	finalSideEffects := atomic.LoadInt32(&sideEffectsExecuted)
	if finalSideEffects != 1 {
		t.Errorf("Criterion 2 Failure: side effects executed %d times; expected exactly 1", finalSideEffects)
	}

	cancel2()
	<-engineDone2
}

// Acceptance Criterion 3 & Segment 4 Demo Test:
// TestDemo_Segment4_IdempotencyAndFaultTolerance runs an integrated scenario for demo segment 4 (section 10.2).
func TestDemo_Segment4_IdempotencyAndFaultTolerance(t *testing.T) {
	cfg := &config.Config{
		Environment: "demo",
		RedisURL:    "localhost:6379",
	}

	flushRedis(t, cfg.RedisURL)
	defer flushRedis(t, cfg.RedisURL)

	queueName := "demo_segment4_queue"
	taskType := "demo:media_transcode"
	idempotencyKey := "demo-transcode-req-1001"

	alertChan := make(chan string, 1)
	alertHandler := func(ctx context.Context, task *asynq.Task, err error) {
		taskID, _ := asynq.GetTaskID(ctx)
		alertChan <- taskID
	}

	fastBackoff := func(n int, err error, task *asynq.Task) time.Duration {
		return 15 * time.Millisecond
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		opt = &redis.Options{Addr: cfg.RedisURL}
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	redisStore := NewRedisIdempotencyStore(rdb)

	var sideEffectCounter int32

	engine, err := NewWorkerEngine(cfg,
		WithConcurrency(2),
		WithQueues(map[string]int{queueName: 1}),
		WithRetryDelayFunc(fastBackoff),
		WithDelayedTaskCheckInterval(15*time.Millisecond),
		WithTaskCheckInterval(15*time.Millisecond),
		WithAlertHandler(alertHandler),
		WithIdempotencyStore(redisStore),
	)
	if err != nil {
		t.Fatalf("failed to initialize demo worker engine: %v", err)
	}

	engine.mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
		atomic.AddInt32(&sideEffectCounter, 1)
		return nil
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
		t.Fatalf("failed to initialize demo queue client: %v", err)
	}
	defer client.Close()

	payload, _ := json.Marshal(task.MediaProcessPayload{
		ResourceID:     "res-media-001",
		TaskType:       "hls_transcode",
		StoragePath:    "courses/1/video.mp4",
		IdempotencyKey: idempotencyKey,
	})

	// Step 1: Send initial event/job
	job1 := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.TaskID(idempotencyKey),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), job1)
	if err != nil {
		t.Fatalf("demo step 1: failed to enqueue job 1: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&sideEffectCounter) != 1 {
		t.Fatalf("demo step 1: expected 1 persisted effect, got %d", atomic.LoadInt32(&sideEffectCounter))
	}

	// Step 2: Send DUPLICATE event/job with same Idempotency-Key
	job2 := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.TaskID(fmt.Sprintf("%s-dup2", idempotencyKey)),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), job2)
	if err != nil {
		t.Fatalf("demo step 2: failed to enqueue job 2 (duplicate): %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Step 3: Re-enqueue job with same Idempotency-Key
	job3 := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.TaskID(fmt.Sprintf("%s-reenq3", idempotencyKey)),
		asynq.Queue(queueName),
	)

	_, err = client.Enqueue(context.Background(), job3)
	if err != nil {
		t.Fatalf("demo step 3: failed to enqueue job 3 (re-enqueue): %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Final verification
	finalEffectCount := atomic.LoadInt32(&sideEffectCounter)
	if finalEffectCount != 1 {
		t.Fatalf("DEMO FAILED: Expected exactly 1 persisted effect across initial, duplicate, and re-enqueue deliveries, got %d", finalEffectCount)
	}

	cancel()
	<-engineDone
}

package worker

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyStore manages task idempotency status to prevent duplicate execution of task effects.
type IdempotencyStore interface {
	IsProcessed(ctx context.Context, key string) (bool, error)
	MarkProcessed(ctx context.Context, key string, ttl time.Duration) error
}

// MemoryIdempotencyStore provides an in-memory thread-safe implementation of IdempotencyStore.
type MemoryIdempotencyStore struct {
	mu    sync.RWMutex
	store map[string]time.Time
}

// NewMemoryIdempotencyStore creates a new MemoryIdempotencyStore instance.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		store: make(map[string]time.Time),
	}
}

// IsProcessed checks if the given idempotency key has already been processed.
func (m *MemoryIdempotencyStore) IsProcessed(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	expiry, exists := m.store[key]
	if !exists {
		return false, nil
	}
	if !expiry.IsZero() && time.Now().After(expiry) {
		return false, nil
	}
	return true, nil
}

// MarkProcessed records the given idempotency key as processed.
func (m *MemoryIdempotencyStore) MarkProcessed(ctx context.Context, key string, ttl time.Duration) error {
	if key == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var expiry time.Time
	if ttl > 0 {
		expiry = time.Now().Add(ttl)
	}
	m.store[key] = expiry
	return nil
}

// RedisIdempotencyStore provides a Redis-backed implementation of IdempotencyStore.
type RedisIdempotencyStore struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisIdempotencyStore creates a new RedisIdempotencyStore instance.
func NewRedisIdempotencyStore(client redis.UniversalClient) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{
		client: client,
		prefix: "idempotency:worker:",
	}
}

// IsProcessed checks Redis to see if the idempotency key exists.
func (r *RedisIdempotencyStore) IsProcessed(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	res, err := r.client.Exists(ctx, r.prefix+key).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

// MarkProcessed sets the idempotency key in Redis with a TTL.
func (r *RedisIdempotencyStore) MarkProcessed(ctx context.Context, key string, ttl time.Duration) error {
	if key == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return r.client.Set(ctx, r.prefix+key, "1", ttl).Err()
}

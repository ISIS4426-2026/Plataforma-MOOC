package worker

import (
	"context"
	"fmt"
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

// NewRedisIdempotencyStoreFromURL construye el almacen compartido a partir de la
// misma URL con la que el worker habla con la cola.
//
// Por que hace falta, y no es un atajo de comodidad:
//
// El almacen por omision de `NewWorkerEngine` es el de memoria, que vive dentro
// de un proceso. Con un worker suelto eso bastaba. Desde C5 el worker es un
// grupo de 1 a 3 instancias, y la marca de «esto ya se hizo» que deja una
// instancia no la ve ninguna otra: exactamente el caso que C7 tiene que cubrir,
// porque un trabajo reencolado por un drenaje lo recoge **otra** instancia.
//
// Comparte el Redis de la cola a proposito. Es una dependencia mas que ya
// existe --si ese Redis no esta, no hay cola y no hay nada que procesar-- y
// evita un segundo punto unico de falla. Las claves van con el prefijo
// `idempotency:worker:`, asi que no se confunden con las de asynq.
func NewRedisIdempotencyStoreFromURL(redisURL string) (*RedisIdempotencyStore, error) {
	opt, err := ParseRedisOpt(redisURL)
	if err != nil {
		return nil, err
	}

	client, ok := opt.MakeRedisClient().(redis.UniversalClient)
	if !ok {
		return nil, fmt.Errorf("la conexion de redis %T no es un cliente universal", opt)
	}
	return NewRedisIdempotencyStore(client), nil
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

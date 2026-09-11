package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

const idempotencyKeyPrefix = "idempotency:"

// inProgressMarker is stored while a request holds a key but has not finished.
// A retry that finds it knows the original is still running, which is a
// different situation from "never seen" and from "already answered".
const inProgressMarker = "in_progress"

// storedRecord is what actually lives in Redis: either a claim in progress or a
// finished response.
type storedRecord struct {
	State       string                   `json:"state"`
	Fingerprint string                   `json:"fingerprint"`
	Response    *domain.RecordedResponse `json:"response,omitempty"`
}

// IdempotencyStore is the Redis implementation of domain.IdempotencyStore.
//
// Redis rather than PostgreSQL because these records are short-lived by design
// and are read on the hot path of every retried write; they are a cache of
// answers, not part of the transactional record.
type IdempotencyStore struct {
	client *redis.Client
}

var _ domain.IdempotencyStore = (*IdempotencyStore)(nil)

func NewIdempotencyStore(client *redis.Client) *IdempotencyStore {
	return &IdempotencyStore{client: client}
}

// Reserve claims the key for a request that is about to run.
//
// The claim is made with SET NX, which succeeds only if the key does not exist.
// That is what makes it atomic: of two concurrent retries exactly one wins the
// claim, and the loser is told the original is in progress rather than being
// allowed to run the operation a second time.
func (s *IdempotencyStore) Reserve(ctx context.Context, key, fingerprint string, ttl time.Duration) (*domain.RecordedResponse, bool, error) {
	redisKey := idempotencyKeyPrefix + key

	claim, err := json.Marshal(storedRecord{State: inProgressMarker, Fingerprint: fingerprint})
	if err != nil {
		return nil, false, fmt.Errorf("encode idempotency claim: %w", err)
	}

	won, err := s.client.SetNX(ctx, redisKey, claim, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("reserve idempotency key: %w", err)
	}
	if won {
		return nil, false, nil
	}

	// The key was taken. Read what is behind it to tell a finished response
	// from a claim still in flight.
	raw, err := s.client.Get(ctx, redisKey).Bytes()
	if errors.Is(err, redis.Nil) {
		// It expired between the failed claim and this read. Treating it as in
		// progress is the safe answer: the alternative is running the operation
		// again, which is what the key exists to prevent.
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read idempotency record: %w", err)
	}

	var record storedRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, false, fmt.Errorf("decode idempotency record: %w", err)
	}

	if record.State == inProgressMarker || record.Response == nil {
		return nil, true, nil
	}

	return record.Response, false, nil
}

// Store attaches the outcome to a key already claimed by Reserve.
//
// The TTL is reset here so the recorded answer lives for the full retention
// window, rather than expiring with whatever was left of the claim's.
func (s *IdempotencyStore) Store(ctx context.Context, key string, response *domain.RecordedResponse, ttl time.Duration) error {
	record, err := json.Marshal(storedRecord{
		State:       "completed",
		Fingerprint: response.RequestFingerprint,
		Response:    response,
	})
	if err != nil {
		return fmt.Errorf("encode idempotency record: %w", err)
	}

	if err := s.client.Set(ctx, idempotencyKeyPrefix+key, record, ttl).Err(); err != nil {
		return fmt.Errorf("store idempotency record: %w", err)
	}

	return nil
}

// Release drops a claim that produced no reusable answer.
func (s *IdempotencyStore) Release(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, idempotencyKeyPrefix+key).Err(); err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}

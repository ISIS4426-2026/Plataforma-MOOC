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

// Key layout:
//
//	session:<token hash>   the cached session, expiring with the session itself
//	session:user:<user id> set of that user's token hashes
//
// The per-user set exists so revoking every session of a user is a single
// lookup. Scanning the keyspace instead would be O(number of sessions) and, with
// SCAN, not even atomic.
const (
	sessionKeyPrefix     = "session:"
	sessionUserKeyPrefix = "session:user:"
)

// SessionCache is the Redis implementation of domain.SessionCache.
//
// Sessions are stored with a native TTL, so expiry needs no sweeper, and a
// delete takes effect on the very next request, which is what makes revocation
// immediate.
type SessionCache struct {
	client *redis.Client
}

var _ domain.SessionCache = (*SessionCache)(nil)

func NewSessionCache(client *redis.Client) *SessionCache {
	return &SessionCache{client: client}
}

func sessionKey(tokenHash string) string  { return sessionKeyPrefix + tokenHash }
func sessionUserKey(userID string) string { return sessionUserKeyPrefix + userID }

// Save caches the session and indexes it under its user.
//
// A non-positive TTL means the session is already expired, so caching it would
// create an entry Redis never removes. Nothing is written in that case.
func (c *SessionCache) Save(ctx context.Context, session *domain.Session, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := sessionKey(session.TokenHash)
	userKey := sessionUserKey(session.UserID)

	// A pipeline keeps the session and its index entry in one round trip; the
	// index is given the same TTL so it cannot outlive the sessions it tracks.
	pipe := c.client.TxPipeline()
	pipe.Set(ctx, key, payload, ttl)
	pipe.SAdd(ctx, userKey, session.TokenHash)
	pipe.Expire(ctx, userKey, ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache session: %w", err)
	}

	return nil
}

// GetByTokenHash returns the cached session, or domain.ErrNotFound on a miss.
// A miss is ordinary: the entry may have expired or Redis may have been
// restarted, and the caller falls back to the durable record.
func (c *SessionCache) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	payload, err := c.client.Get(ctx, sessionKey(tokenHash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read cached session: %w", err)
	}

	var session domain.Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("unmarshal cached session: %w", err)
	}

	return &session, nil
}

// Delete drops one cached session and its index entry.
//
// The session is read first only to learn which user set to clean; if it is
// already gone the key deletion still runs, so the operation is idempotent.
func (c *SessionCache) Delete(ctx context.Context, tokenHash string) error {
	session, err := c.GetByTokenHash(ctx, tokenHash)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	pipe := c.client.TxPipeline()
	pipe.Del(ctx, sessionKey(tokenHash))
	if session != nil {
		pipe.SRem(ctx, sessionUserKey(session.UserID), tokenHash)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete cached session: %w", err)
	}

	return nil
}

// DeleteAllForUser drops every cached session of a user, so an administrative
// revocation or a password change cannot be outlived by a warm cache entry.
func (c *SessionCache) DeleteAllForUser(ctx context.Context, userID string) error {
	userKey := sessionUserKey(userID)

	tokenHashes, err := c.client.SMembers(ctx, userKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("list cached sessions for user %s: %w", userID, err)
	}

	keys := make([]string, 0, len(tokenHashes)+1)
	for _, tokenHash := range tokenHashes {
		keys = append(keys, sessionKey(tokenHash))
	}
	keys = append(keys, userKey)

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete cached sessions for user %s: %w", userID, err)
	}

	return nil
}

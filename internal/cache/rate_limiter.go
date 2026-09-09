package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

const rateLimitKeyPrefix = "ratelimit:"

// RateLimiter is the Redis implementation of domain.RateLimiter.
//
// It uses a fixed window: one counter per key, expiring when the window closes.
// The trade-off is a burst at the boundary — a caller can spend its whole quota
// at the end of one window and again at the start of the next, so the real
// worst case is twice the limit over a short span. A sliding window would avoid
// that at the cost of storing a timestamp per request; for slowing down
// credential guessing, the fixed window is accurate enough and far cheaper.
//
// Counting lives in Redis rather than in process memory because the API is
// stateless and scales horizontally: a per-instance counter would multiply the
// effective limit by the number of instances.
type RateLimiter struct {
	client *redis.Client
}

var _ domain.RateLimiter = (*RateLimiter)(nil)

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Allow consumes one unit of quota for key.
//
// INCR and the TTL read travel in a single pipeline, so the decision costs one
// round trip. The expiry is set only when the counter is created: refreshing it
// on every hit would let a steady stream of requests keep the window open
// forever and never reset the count.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (domain.RateLimitResult, error) {
	redisKey := rateLimitKeyPrefix + key

	pipe := r.client.TxPipeline()
	incr := pipe.Incr(ctx, redisKey)
	// NX applies the TTL only if the key has none yet, which is exactly the
	// first request of a window.
	pipe.ExpireNX(ctx, redisKey, window)
	ttl := pipe.TTL(ctx, redisKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return domain.RateLimitResult{}, fmt.Errorf("rate limit check: %w", err)
	}

	count := int(incr.Val())
	retryAfter := ttl.Val()
	// A missing or negative TTL means the key exists without expiry, which
	// should not happen; falling back to the window keeps a stuck counter from
	// blocking the key forever.
	if retryAfter <= 0 {
		retryAfter = window
		if err := r.client.Expire(ctx, redisKey, window).Err(); err != nil {
			return domain.RateLimitResult{}, fmt.Errorf("restore rate limit expiry: %w", err)
		}
	}

	remaining := max(limit-count, 0)

	return domain.RateLimitResult{
		Allowed:    count <= limit,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}

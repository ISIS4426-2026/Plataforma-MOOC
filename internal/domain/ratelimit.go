package domain

import (
	"context"
	"time"
)

// RateLimitResult describes the outcome of consuming one unit of quota.
type RateLimitResult struct {
	// Allowed reports whether the request may proceed.
	Allowed bool
	// Limit is the ceiling that applied to this decision, echoed to the client.
	Limit int
	// Remaining is how much quota is left in the current window, never negative.
	Remaining int
	// RetryAfter is how long until the window resets. It is what a rejected
	// caller should wait before trying again.
	RetryAfter time.Duration
}

// RateLimiter throttles requests identified by an opaque key.
//
// It is a port so the throttling policy can be exercised in tests without
// Redis, and so the storage can change without touching the HTTP layer.
type RateLimiter interface {
	// Allow consumes one unit of quota for key and reports whether the request
	// may proceed.
	//
	// Implementations must treat an unknown key as a fresh window. Callers
	// decide what to do when the limiter itself fails; see the middleware for
	// the policy this project applies.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitResult, error)
}

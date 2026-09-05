package worker

import (
	"time"

	"github.com/hibiken/asynq"
)

// DefaultExponentialBackoff calculates exponential backoff delay based on retry count n.
// n is 0-indexed:
// n=0 (1st retry): 2s
// n=1 (2nd retry): 4s
// n=2 (3rd retry): 8s
func DefaultExponentialBackoff(n int, err error, task *asynq.Task) time.Duration {
	base := 2 * time.Second
	maxDelay := 10 * time.Minute

	if n < 0 {
		n = 0
	}
	// Prevent uint overflow for large n
	if n > 30 {
		n = 30
	}

	delay := base * time.Duration(1<<uint(n))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

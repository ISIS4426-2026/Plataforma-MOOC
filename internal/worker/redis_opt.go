package worker

import (
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
)

// ParseRedisOpt parses a Redis connection URL or host:port string into an asynq.RedisConnOpt.
func ParseRedisOpt(redisURL string) (asynq.RedisConnOpt, error) {
	trimmed := strings.TrimSpace(redisURL)
	if trimmed == "" {
		trimmed = "localhost:6379"
	}

	if strings.HasPrefix(trimmed, "redis://") ||
		strings.HasPrefix(trimmed, "rediss://") ||
		strings.HasPrefix(trimmed, "redis-socket://") ||
		strings.HasPrefix(trimmed, "redis-sentinel://") {
		opt, err := asynq.ParseRedisURI(trimmed)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redis URI: %w", err)
		}
		return opt, nil
	}

	return asynq.RedisClientOpt{Addr: trimmed}, nil
}

// Package cache holds the Redis-backed implementations of the caching ports
// declared in internal/domain. Redis carries live sessions, and later rate
// limiting, while PostgreSQL remains the transactional source of truth.
package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
)

// Connect opens the Redis client and verifies it answers.
//
// REDIS_URL is accepted both as a bare "host:port", which is what
// docker-compose.yml provides, and as a full redis:// URL for deployments that
// need credentials or TLS.
func Connect(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	options, err := parseRedisURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

func parseRedisURL(rawURL string) (*redis.Options, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("redis url is empty")
	}

	if strings.HasPrefix(trimmed, "redis://") || strings.HasPrefix(trimmed, "rediss://") {
		options, err := redis.ParseURL(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		return options, nil
	}

	return &redis.Options{Addr: trimmed}, nil
}

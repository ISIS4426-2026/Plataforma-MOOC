package config

import (
	"os"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string
	RedisURL    string
	S3Endpoint  string
	S3Bucket    string
}

func Load() *Config {
	return &Config{
		Environment: getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mooc?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "localhost:6379"),
		S3Endpoint:  getEnv("S3_ENDPOINT", "http://localhost:9000"),
		S3Bucket:    getEnv("S3_BUCKET", "mooc-storage"),
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

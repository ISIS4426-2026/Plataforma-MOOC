package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string
	RedisURL    string
	S3Endpoint  string
	S3Bucket    string

	// Database connection pool. The API and workers are stateless and scale
	// horizontally, so each instance must bound its own share of Postgres
	// connections instead of relying on the driver defaults (unlimited open
	// connections, which exhausts max_connections once instances multiply).
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// Outbound mail. Locally this points at the Mailpit container declared in
	// docker-compose.yml, where verification and password reset messages can be
	// inspected without sending real email.
	SMTPHost     string
	SMTPPort     int
	SMTPFrom     string
	SMTPUsername string
	SMTPPassword string

	// Session lifetime used when issuing credentials.
	SessionTTL time.Duration

	// AppBaseURL is the public origin the frontend is served from. Verification
	// links are built against it, so it must be the address the user's browser
	// can reach, not the container hostname.
	AppBaseURL string

	// EmailVerificationTTL is how long an activation link stays valid.
	EmailVerificationTTL time.Duration
}

func Load() *Config {
	return &Config{
		Environment: getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mooc?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "localhost:6379"),
		S3Endpoint:  getEnv("S3_ENDPOINT", "http://localhost:9000"),
		S3Bucket:    getEnv("S3_BUCKET", "mooc-storage"),

		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 25),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),

		SMTPHost:     getEnv("SMTP_HOST", "mailpit"),
		SMTPPort:     getEnvInt("SMTP_PORT", 1025),
		SMTPFrom:     getEnv("SMTP_FROM", "no-reply@plataforma-mooc.local"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),

		SessionTTL: getEnvDuration("SESSION_TTL", 24*time.Hour),

		AppBaseURL:           getEnv("APP_BASE_URL", "http://localhost:8080"),
		EmailVerificationTTL: getEnvDuration("EMAIL_VERIFICATION_TTL", 24*time.Hour),
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// getEnvInt falls back to defaultValue when the variable is unset or is not a
// valid integer, so a malformed override degrades to a working default instead
// of starting the process with a zero-valued pool or port.
func getEnvInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// getEnvDuration accepts any value understood by time.ParseDuration, e.g. "30s"
// or "15m".
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		return defaultValue
	}
	return parsed
}

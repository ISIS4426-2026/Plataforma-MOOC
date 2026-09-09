// Package postgres provides the PostgreSQL-backed implementations of the
// repository ports declared in internal/domain. The domain layer depends only
// on its own interfaces, so this package is the single place that knows about
// SQL, the driver and the concrete schema.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
)

// Connect opens the connection pool and verifies it is actually reachable.
//
// sql.Open is lazy: it validates the DSN but never contacts the server, so
// without the Ping below a misconfigured DATABASE_URL would surface as a
// request-time failure long after startup instead of failing fast.
func Connect(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		// The pool is discarded here, so close it to avoid leaking the
		// background connection opener goroutine.
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/admin"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/cache"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/course"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/http"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/mailer"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/postgres"
)

// startupTimeout bounds the initial database probe so a missing dependency
// fails the container fast instead of hanging the deployment.
const startupTimeout = 10 * time.Second

// shutdownTimeout is how long in-flight requests are given to finish once a
// termination signal arrives.
const shutdownTimeout = 15 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("api server terminated", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// run holds the startup sequence so every deferred cleanup executes before the
// process exits; calling os.Exit inside main would skip them.
func run(logger *slog.Logger) error {
	cfg := config.Load()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), startupTimeout)
	defer cancelStartup()

	db, err := postgres.Connect(startupCtx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	logger.Info("connected to postgres",
		slog.Int("max_open_conns", cfg.DBMaxOpenConns),
		slog.String("conn_max_lifetime", cfg.DBConnMaxLifetime.String()),
	)

	redisClient, err := cache.Connect(startupCtx, cfg)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	logger.Info("connected to redis", slog.String("session_ttl", cfg.SessionTTL.String()))

	// Composition root: the concrete adapters are chosen here and everything
	// below depends only on the domain ports they satisfy.
	// Shared adapters are built once: the session store in particular is used by
	// both authentication and administration, and two instances would be two
	// connection-pool consumers doing the same job.
	sessionRepo := postgres.NewSessionRepository(db)
	sessionCache := cache.NewSessionCache(redisClient)

	authService := auth.NewService(
		auth.Deps{
			Users:               postgres.NewUserRepository(db),
			Sessions:            sessionRepo,
			SessionCache:        sessionCache,
			VerificationTokens:  postgres.NewEmailVerificationTokenRepository(db),
			PasswordResetTokens: postgres.NewPasswordResetTokenRepository(db),
			Mailer:              mailer.NewSMTPMailer(cfg),
		},
		auth.Config{
			AppBaseURL:           cfg.AppBaseURL,
			SessionTTL:           cfg.SessionTTL,
			EmailVerificationTTL: cfg.EmailVerificationTTL,
			PasswordResetTTL:     cfg.PasswordResetTTL,
		},
		logger,
	)

	adminService := admin.NewService(admin.Deps{
		Admin:        postgres.NewAdminRepository(db),
		Sessions:     sessionRepo,
		SessionCache: sessionCache,
	}, logger)

	courseService := course.NewService(course.Deps{
		Courses: postgres.NewCourseRepository(db),
	}, logger)

	server := http.NewServer(cfg, http.Deps{
		DB:               db,
		Auth:             authService,
		Admin:            adminService,
		Course:           courseService,
		RateLimiter:      cache.NewRateLimiter(redisClient),
		IdempotencyStore: cache.NewIdempotencyStore(redisClient),
	}, logger)

	// Serve in the background so the main goroutine can wait for a termination
	// signal and trigger a graceful shutdown.
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	// SIGTERM is what `docker compose down` and orchestrators send; SIGINT
	// covers Ctrl+C during local development.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-stop:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("api server stopped cleanly")
	return nil
}

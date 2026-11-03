package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/observability"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/push"
)

// servePush corre el worker en modo Pub/Sub (Entrega 4) hasta SIGTERM; la logica vive en worker.ServePush.
func servePush(cfg *config.Config, processor push.Processor, obs *observability.Providers) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := worker.ServePush(ctx, cfg.Port, os.Getenv("REDIS_URL"), processor, obs.MetricsHandler, slog.Default()); err != nil {
		log.Fatalf("el servidor push se detuvo: %v\n", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := obs.Shutdown(shutdownCtx); err != nil {
		log.Printf("failed to shut down observability providers: %v\n", err)
	}
}

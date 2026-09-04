package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting Plataforma MOOC Background Worker [Env: %s]\n", cfg.Environment)
	engine := worker.NewWorkerEngine(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Worker engine encountered fatal error: %v\n", err)
	}
}

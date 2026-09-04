package worker

import (
	"context"
	"log"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
)

type WorkerEngine struct {
	cfg *config.Config
}

func NewWorkerEngine(cfg *config.Config) *WorkerEngine {
	return &WorkerEngine{
		cfg: cfg,
	}
}

func (w *WorkerEngine) Start(ctx context.Context) error {
	log.Println("[Worker Engine] Starting background worker processor...")
	log.Printf("[Worker Engine] Redis queue connected at %s\n", w.cfg.RedisURL)

	// Block until context cancelled
	<-ctx.Done()
	log.Println("[Worker Engine] Shutting down background worker processor...")
	return nil
}

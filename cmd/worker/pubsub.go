package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/observability"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/push"
)

// servePush sirve el endpoint al que Pub/Sub entrega los trabajos y bloquea hasta SIGTERM.
//
//	POST /pubsub/push   el mensaje; se confirma (204) solo cuando el trabajo termino y quedo guardado
//	GET  /healthz       el proceso respira
//	GET  /readyz        igual que /healthz: este worker no depende de una cola que pueda estar caida
//	GET  /metrics       las mismas metricas del worker de asynq
//
// Escucha en $PORT, que Cloud Run inyecta. Quien puede llamarlo lo decide IAM antes de llegar aqui: el
// servicio es privado y solo la cuenta de la suscripcion push tiene run.invoker.
func servePush(cfg *config.Config, processor push.Processor, obs *observability.Providers) {
	var dedupe push.Deduper
	// La idempotencia compartida es opcional en esta ruta: con REDIS_URL accesible se usa el mismo almacen del
	// worker de asynq; sin el, la propia tuberia ya omite un recurso `completed` sobre el mismo objeto.
	if os.Getenv("REDIS_URL") != "" {
		if store, err := worker.NewRedisIdempotencyStoreFromURL(cfg.RedisURL); err == nil {
			dedupe = store
		} else {
			log.Printf("idempotencia compartida deshabilitada: %v\n", err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("POST /pubsub/push", push.NewHandler(processor, dedupe, 24*time.Hour, slog.Default()))
	mux.Handle("GET /healthz", worker.NewLivenessHandler())
	mux.Handle("GET /readyz", worker.NewLivenessHandler())
	mux.Handle("GET /metrics", obs.MetricsHandler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Printf("Worker Pub/Sub escuchando en :%s (POST /pubsub/push)\n", cfg.Port)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("el servidor push se detuvo: %v\n", err)
		}
	case <-ctx.Done():
	}

	// Lo que siga en curso se queda sin confirmar y Pub/Sub lo reentrega: no se espera a un trabajo de minutos.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	if err := obs.Shutdown(shutdownCtx); err != nil {
		log.Printf("failed to shut down observability providers: %v\n", err)
	}
}

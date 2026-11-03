package worker

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/push"
)

// pushGrace es el plazo de cierre: lo que siga en curso se queda sin confirmar y Pub/Sub lo reentrega, asi que
// no se espera a un trabajo de minutos.
const pushGrace = 8 * time.Second

// ServePush sirve el endpoint al que Pub/Sub entrega los trabajos y bloquea hasta que `ctx` termine.
//
// Escucha en `:port` ($PORT, que Cloud Run inyecta). Quien puede llamarlo lo decide IAM antes de llegar aqui: el
// servicio es privado y solo la cuenta de la suscripcion push tiene run.invoker. La idempotencia compartida es
// opcional en esta ruta: con `redisURL` se usa el mismo almacen del worker de asynq; sin el, la propia tuberia ya
// omite un recurso `completed` sobre el mismo objeto.
func ServePush(ctx context.Context, port, redisURL string, processor push.Processor, metrics http.Handler, logger *slog.Logger) error {
	var dedupe push.Deduper
	if redisURL != "" {
		if store, err := NewRedisIdempotencyStoreFromURL(redisURL); err == nil {
			dedupe = store
		} else {
			logger.Warn("idempotencia compartida deshabilitada", slog.String("error", err.Error()))
		}
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	mux := push.NewMux(push.NewHandler(processor, dedupe, 24*time.Hour, logger), NewLivenessHandler(), metrics)
	logger.Info("worker Pub/Sub escuchando", slog.String("addr", ln.Addr().String()), slog.String("path", "POST /pubsub/push"))
	return push.Serve(ctx, ln, mux, pushGrace)
}

package worker

import (
	"context"
	"log/slog"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/pubsub"
)

// MediaQueue es lo que la API necesita de la cola de medios; es el mismo puerto estrecho que declara
// internal/media (media.Enqueuer), repetido aqui para que este paquete no importe al servicio.
type MediaQueue interface {
	EnqueueMediaProcessJob(ctx context.Context, resourceID, taskType, objectKey, idempotencyKey string) error
}

// NewMediaQueue elige como la API entrega los trabajos de medios segun QUEUE_BACKEND: asynq (Redis, lo que
// corrieron las Entregas 2 y 3) o Cloud Pub/Sub (Entrega 4). Devuelve tambien la funcion que cierra la conexion.
func NewMediaQueue(ctx context.Context, cfg *config.Config, logger *slog.Logger) (MediaQueue, func(), error) {
	if cfg.QueueBackend == "pubsub" {
		publisher, err := pubsub.NewPublisher(ctx, cfg.PubSubTopic, logger)
		if err != nil {
			return nil, nil, err
		}
		logger.Info("media jobs go through Cloud Pub/Sub", slog.String("topic", cfg.PubSubTopic))
		return publisher, func() {}, nil
	}

	client, err := NewClient(cfg.RedisURL)
	if err != nil {
		return nil, nil, err
	}
	return client, func() { _ = client.Close() }, nil
}

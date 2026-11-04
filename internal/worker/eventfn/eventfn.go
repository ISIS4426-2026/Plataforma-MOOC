// Package eventfn is the body of the Cloud Function that checks documents (Entrega 5).
//
// Cloud Run functions (2nd gen) deliver a Pub/Sub message as a CloudEvent whose data is
// the same envelope Pub/Sub puts in a push request: {"message": {"data": <base64>,
// "attributes": {...}, "messageId": "..."}}. The job inside is the MediaProcessPayload
// the API has always published, so the function runs the very same code the worker ran
// for this task (handler.MediaProcessor) and the outcome on the resource is identical.
//
// What the function returns is its acknowledgement, and it is the opposite of an HTTP
// status: the trigger is configured to retry, so
//
//	nil     the job is done, or can never be done: the event is consumed
//	error   a transient failure: the event is redelivered with backoff
//
// A permanent failure (a corrupt document, a resource deleted while the job waited) is
// recorded on the resource by the processor and then acknowledged: retrying it would
// repeat the same failure. A redelivery of a job that already finished is harmless: the
// processor skips a resource that is already `completed` on that object.
package eventfn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/media"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

// Processor runs one media job. *handler.MediaProcessor satisfies it.
type Processor interface {
	ProcessPayload(ctx context.Context, payload task.MediaProcessPayload) (permanent bool, err error)
}

// pubsubEvent is the data of a google.cloud.pubsub.topic.v1.messagePublished event.
type pubsubEvent struct {
	Message struct {
		Data        []byte            `json:"data"` // encoding/json decodes the base64 for us
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// Handle processes one event and says whether it should be redelivered (a non-nil error).
func Handle(ctx context.Context, p Processor, logger *slog.Logger, eventData []byte) error {
	if logger == nil {
		logger = slog.Default()
	}

	var ev pubsubEvent
	if err := json.Unmarshal(eventData, &ev); err != nil {
		logger.Error("document event dropped: not a Pub/Sub event", slog.String("error", err.Error()))
		return nil // it will never become readable on a retry
	}
	id := ev.Message.MessageID

	var payload task.MediaProcessPayload
	if err := json.Unmarshal(ev.Message.Data, &payload); err != nil {
		logger.Error("document event dropped: data is not a media job",
			slog.String("message_id", id), slog.String("error", err.Error()))
		return nil
	}
	if payload.IdempotencyKey == "" {
		payload.IdempotencyKey = ev.Message.Attributes["idempotency_key"]
	}

	log := logger.With(
		slog.String("message_id", id),
		slog.String("resource_id", payload.ResourceID),
		slog.String("object_key", payload.StoragePath),
		slog.String("task_type", payload.TaskType),
		slog.String("idempotency_key", payload.IdempotencyKey),
	)

	// This function owns exactly one task type. Anything else was routed here by mistake;
	// running it would be a second owner for a task, so it is refused and logged.
	if payload.TaskType != media.TaskMalwareScan {
		log.ErrorContext(ctx, "document job refused: this function only runs malware_scan")
		return nil
	}

	log.InfoContext(ctx, "document job received")
	started := time.Now()
	permanent, err := p.ProcessPayload(ctx, payload)
	switch {
	case err == nil:
		log.InfoContext(ctx, "document job completed; event consumed", slog.Duration("took", time.Since(started)))
		return nil
	case permanent:
		log.ErrorContext(ctx, "document job failed permanently; event consumed to stop redelivery",
			slog.String("error", err.Error()))
		return nil
	default:
		log.ErrorContext(ctx, "document job failed; event not consumed, it will be redelivered",
			slog.String("error", err.Error()), slog.Duration("took", time.Since(started)))
		return fmt.Errorf("document job %s: %w", id, err)
	}
}

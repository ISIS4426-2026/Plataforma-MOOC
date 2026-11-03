// Package push receives Cloud Pub/Sub push deliveries on a worker (Entrega 4).
//
// Pub/Sub POSTs each message to the worker's URL and treats the HTTP status as
// the acknowledgement: 2xx acknowledges, anything else means "redeliver". The
// handler therefore answers only after the work is done and persisted, which is
// what the assignment asks (confirm after completing, tolerate redeliveries).
//
//	200..204  acknowledge: done, or nothing more can ever be done
//	5xx       redeliver:   a transient failure, the next delivery tries again
//
// A message that cannot be read, or a job that failed for good, is acknowledged
// after being logged and (for the job) recorded on its resource: redelivering it
// would only repeat the same failure until the subscription's dead-letter policy
// parked it, and the resource already says what happened.
//
// Who may call this endpoint is decided before the request arrives here: the
// Cloud Run service is private and only the push subscription's service account
// holds `roles/run.invoker`. The handler does not re-check the OIDC token.
package push

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

// maxEnvelopeBytes bounds the request: a Pub/Sub message is at most 10 MB, and
// ours are a few hundred bytes.
const maxEnvelopeBytes = 1 << 20

// Processor runs one media job. *handler.MediaProcessor satisfies it.
type Processor interface {
	ProcessPayload(ctx context.Context, payload task.MediaProcessPayload) (permanent bool, err error)
}

// Deduper remembers which jobs finished, across instances. It is the same port
// the asynq worker uses for the shared idempotency store; nil disables it.
type Deduper interface {
	IsProcessed(ctx context.Context, key string) (bool, error)
	MarkProcessed(ctx context.Context, key string, ttl time.Duration) error
}

type envelope struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription    string `json:"subscription"`
	DeliveryAttempt int    `json:"deliveryAttempt"`
}

// Handler is the http.Handler for the push endpoint.
type Handler struct {
	processor Processor
	dedupe    Deduper
	dedupeTTL time.Duration
	logger    *slog.Logger
}

// NewHandler builds the handler. `dedupe` may be nil.
func NewHandler(processor Processor, dedupe Deduper, dedupeTTL time.Duration, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if dedupeTTL <= 0 {
		dedupeTTL = 24 * time.Hour
	}
	return &Handler{processor: processor, dedupe: dedupe, dedupeTTL: dedupeTTL, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxEnvelopeBytes))
	if err != nil {
		h.drop(w, "", "unreadable request body", err)
		return
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		h.drop(w, "", "request is not a pub/sub push envelope", err)
		return
	}
	id := env.Message.MessageID

	data, err := base64.StdEncoding.DecodeString(env.Message.Data)
	if err != nil {
		h.drop(w, id, "message data is not base64", err)
		return
	}
	var payload task.MediaProcessPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		h.drop(w, id, "message data is not a media job", err)
		return
	}

	key := payload.IdempotencyKey
	if key == "" {
		key = env.Message.Attributes["idempotency_key"]
	}
	log := h.logger.With(
		slog.String("message_id", id),
		slog.Int("delivery_attempt", env.DeliveryAttempt),
		slog.String("resource_id", payload.ResourceID),
		slog.String("object_key", payload.StoragePath),
		slog.String("idempotency_key", key),
	)
	log.InfoContext(r.Context(), "media job received")

	// A redelivery of a job that already finished is acknowledged without work.
	if h.dedupe != nil && key != "" {
		done, derr := h.dedupe.IsProcessed(r.Context(), "pubsub:"+key)
		switch {
		case derr != nil:
			// The shared store being down must not stop the work: the pipeline
			// has its own guard (a resource already `completed` on this object
			// is skipped), so this is only the first line of defence.
			log.WarnContext(r.Context(), "idempotency store unavailable; relying on the pipeline's own guard",
				slog.String("error", derr.Error()))
		case done:
			log.InfoContext(r.Context(), "media job redelivered after it finished; acknowledged without work")
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	started := time.Now()
	permanent, err := h.processor.ProcessPayload(r.Context(), payload)
	switch {
	case err == nil:
		if h.dedupe != nil && key != "" {
			if merr := h.dedupe.MarkProcessed(r.Context(), "pubsub:"+key, h.dedupeTTL); merr != nil {
				log.WarnContext(r.Context(), "could not record the job as processed",
					slog.String("error", merr.Error()))
			}
		}
		log.InfoContext(r.Context(), "media job acknowledged",
			slog.Duration("took", time.Since(started)))
		w.WriteHeader(http.StatusNoContent)
	case permanent:
		// Recorded on the resource by the processor; redelivering cannot fix it.
		log.ErrorContext(r.Context(), "media job failed permanently; acknowledged to stop redelivery",
			slog.String("error", err.Error()))
		w.WriteHeader(http.StatusNoContent)
	default:
		log.ErrorContext(r.Context(), "media job failed; not acknowledged, Pub/Sub will redeliver",
			slog.String("error", err.Error()), slog.Duration("took", time.Since(started)))
		http.Error(w, "processing failed; retry", http.StatusInternalServerError)
	}
}

// drop acknowledges a message that can never be processed.
func (h *Handler) drop(w http.ResponseWriter, messageID, why string, err error) {
	h.logger.Error("pub/sub message dropped: "+why,
		slog.String("message_id", messageID), slog.String("error", err.Error()))
	w.WriteHeader(http.StatusNoContent)
}

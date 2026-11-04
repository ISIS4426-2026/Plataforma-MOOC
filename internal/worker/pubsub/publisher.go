// Package pubsub is the Cloud Pub/Sub side of the media queue (Entrega 4).
//
// It replaces asynq/Redis as the way the API hands a job to the workers. The API
// publishes and answers without waiting; Pub/Sub delivers each message to a
// worker through an authenticated push subscription, and the worker acknowledges
// it only after the work is persisted (see internal/worker/push).
//
// The publisher speaks the Pub/Sub REST API over an authenticated HTTP client
// rather than pulling in the full client library: the code needs exactly one call
// (`topics.publish`), and the Application Default Credentials the process already
// has on Cloud Run are enough.
package pubsub

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/media"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
)

const (
	defaultEndpoint = "https://pubsub.googleapis.com"
	pubsubScope     = "https://www.googleapis.com/auth/pubsub"

	// AttrIdempotencyKey is the message attribute that carries the job's
	// idempotency key: the receiving side reads it to tell a redelivery from a
	// second job.
	AttrIdempotencyKey = "idempotency_key"
	AttrTaskType       = "task_type"
	AttrResourceID     = "resource_id"
)

// Publisher publishes media jobs to one topic. It satisfies the narrow port
// internal/media declares (media.Enqueuer), so the service never learns which
// queue is behind it.
type Publisher struct {
	topic    string // projects/<project>/topics/<topic>
	docTopic string // where the document checks go (Entrega 5); empty means topic
	endpoint string
	client   *http.Client
	logger   *slog.Logger
}

// NewPublisher builds a publisher for `topic`, authenticated with Application
// Default Credentials. `topic` is the full name: projects/<project>/topics/<topic>.
func NewPublisher(ctx context.Context, topic string, logger *slog.Logger) (*Publisher, error) {
	if !strings.HasPrefix(topic, "projects/") || !strings.Contains(topic, "/topics/") {
		return nil, fmt.Errorf("pubsub topic %q must be a full name like projects/<project>/topics/<topic>", topic)
	}
	client, err := google.DefaultClient(ctx, pubsubScope)
	if err != nil {
		return nil, fmt.Errorf("application default credentials for Pub/Sub: %w", err)
	}
	return NewPublisherWithClient(topic, defaultEndpoint, client, logger), nil
}

// NewPublisherWithClient is the constructor the tests use, and the place to
// point the publisher at the Pub/Sub emulator.
func NewPublisherWithClient(topic, endpoint string, client *http.Client, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{topic: topic, endpoint: strings.TrimRight(endpoint, "/"), client: client, logger: logger}
}

// WithDocumentsTopic sends the document checks (task type malware_scan) to their own
// topic, which is the one the Cloud Function is triggered from (Entrega 5). Everything
// else keeps going to the worker's topic. An empty name changes nothing.
func (p *Publisher) WithDocumentsTopic(topic string) *Publisher {
	p.docTopic = topic
	return p
}

// topicFor picks the topic a job is published to. The routing lives here, on the
// publishing side, so that moving a task to another consumer is a matter of
// where its messages go and not of every consumer filtering what it receives.
func (p *Publisher) topicFor(taskType string) string {
	if taskType == media.TaskMalwareScan && p.docTopic != "" {
		return p.docTopic
	}
	return p.topic
}

type publishRequest struct {
	Messages []publishMessage `json:"messages"`
}

type publishMessage struct {
	Data       string            `json:"data"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type publishResponse struct {
	MessageIDs []string `json:"messageIds"`
}

// EnqueueMediaProcessJob publishes one media job.
//
// Unlike asynq, Pub/Sub does not refuse a second message with the same key, so
// this never returns domain.ErrTaskAlreadyQueued: a re-confirmed upload
// publishes again and the worker side absorbs the duplicate (the resource is
// already `completed` on that object, or another delivery holds the claim). The
// cost of a duplicate is one cheap message; the cost of the queue deciding what
// counts as a duplicate would be a lost job.
func (p *Publisher) EnqueueMediaProcessJob(ctx context.Context, resourceID, taskType, objectKey, idempotencyKey string) error {
	payload, err := json.Marshal(task.MediaProcessPayload{
		ResourceID:     resourceID,
		TaskType:       taskType,
		StoragePath:    objectKey,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("encode media job: %w", err)
	}

	body, err := json.Marshal(publishRequest{Messages: []publishMessage{{
		Data: base64.StdEncoding.EncodeToString(payload),
		Attributes: map[string]string{
			AttrIdempotencyKey: idempotencyKey,
			AttrTaskType:       taskType,
			AttrResourceID:     resourceID,
		},
	}}})
	if err != nil {
		return fmt.Errorf("encode publish request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/%s:publish", p.endpoint, p.topicFor(taskType)), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build publish request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("publish media job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("publish media job: pub/sub answered %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var out publishResponse
	if err := json.Unmarshal(raw, &out); err != nil || len(out.MessageIDs) == 0 {
		return errors.New("publish media job: pub/sub answered 200 without a message id")
	}

	// The message id is what lets a task be followed from here to its
	// completion: the worker logs the same id when it receives it.
	p.logger.Info("media job published",
		slog.String("message_id", out.MessageIDs[0]),
		slog.String("topic", p.topicFor(taskType)),
		slog.String("resource_id", resourceID),
		slog.String("object_key", objectKey),
		slog.String("task_type", taskType),
		slog.String("idempotency_key", idempotencyKey),
	)
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

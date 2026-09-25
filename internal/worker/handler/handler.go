package handler

import (
	"context"
	"fmt"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// RegisterRoutes registers all task handlers onto the provided ServeMux.
//
// media carries the dependencies the media pipeline needs -- object storage,
// the resource repository, ffmpeg -- which is why it is passed in rather than
// being a package-level function like the ping handler.
func RegisterRoutes(mux *asynq.ServeMux, media asynq.Handler) {
	mux.HandleFunc(task.TypeTestPing, HandleTestPingTask)

	if media == nil {
		// Registering nothing would leave media jobs to pile up unacknowledged,
		// and registering a no-op would mark them done without producing a
		// single segment. Failing loudly is the only option that tells the
		// truth about a worker started without its media dependencies.
		media = asynq.HandlerFunc(mediaNotConfigured)
	}
	mux.Handle(task.TypeMediaProcess, media)
}

func mediaNotConfigured(_ context.Context, t *asynq.Task) error {
	return fmt.Errorf("worker received %s but was started without media dependencies: %w",
		t.Type(), asynq.SkipRetry)
}

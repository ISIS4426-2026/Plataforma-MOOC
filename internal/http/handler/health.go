package handler

import (
	"context"
	"net/http"
	"time"
)

// healthCheckTimeout bounds the dependency probe so a hung database makes the
// endpoint answer "fail" quickly instead of blocking until the client gives up.
// It stays below the 5s timeout configured for the Compose healthcheck.
const healthCheckTimeout = 2 * time.Second

// Pinger is the subset of *sql.DB the health endpoint needs, kept narrow so the
// handler can be tested without a real database.
type Pinger interface {
	PingContext(ctx context.Context) error
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// NewHealthHandler builds the /api/v1/health endpoint.
//
// The endpoint reports only dependencies it actually probes. It answers 200
// with "pass" when every dependency responds and 503 with "fail" otherwise, so
// an unhealthy instance is removed from rotation rather than being handed
// traffic it cannot serve.
func NewHealthHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		services := make(map[string]string, 1)
		healthy := true

		if err := db.PingContext(ctx); err != nil {
			services["database"] = "down"
			healthy = false
		} else {
			services["database"] = "up"
		}

		status := "pass"
		code := http.StatusOK
		if !healthy {
			status = "fail"
			code = http.StatusServiceUnavailable
		}

		RespondWithJSON(w, code, HealthResponse{
			Status:    status,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Services:  services,
		})
	}
}

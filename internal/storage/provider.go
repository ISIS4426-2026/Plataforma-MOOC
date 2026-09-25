package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// Backend names the object storage implementation to build.
type Backend string

const (
	// BackendGCS is the managed service used by the deployed environment.
	BackendGCS Backend = "gcs"
	// BackendMinIO is the local development container.
	BackendMinIO Backend = "minio"
)

// Config is what New needs to pick and build an adapter.
type Config struct {
	Backend   Backend
	Bucket    string
	Endpoint  string // MinIO only
	AccessKey string // MinIO only
	SecretKey string // MinIO only
}

// New builds the adapter named by cfg.Backend.
//
// The backend is configuration, not a build tag, so the same binary runs
// locally against MinIO and in the cloud against Cloud Storage. That is what
// keeps the deployed path from being one nobody exercised before the migration.
func New(ctx context.Context, cfg Config) (domain.StorageProvider, error) {
	switch Backend(strings.ToLower(string(cfg.Backend))) {
	case BackendGCS:
		return NewGCS(ctx, cfg.Bucket)
	case BackendMinIO:
		return NewMinIO(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Bucket)
	default:
		return nil, fmt.Errorf("storage: unknown backend %q (want %q or %q)", cfg.Backend, BackendGCS, BackendMinIO)
	}
}

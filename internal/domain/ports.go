package domain

import (
	"context"
	"time"
)

// StorageProvider abstracts object storage operations (S3, MinIO, etc.)
type StorageProvider interface {
	GeneratePresignedUploadURL(ctx context.Context, objectKey string, expiresDuration time.Duration) (string, error)
	GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expiresDuration time.Duration) (string, error)
	DeleteObject(ctx context.Context, objectKey string) error
}

// TaskQueue abstracts asynchronous background job dispatching.
type TaskQueue interface {
	EnqueueTask(ctx context.Context, taskType string, payload []byte) error
}

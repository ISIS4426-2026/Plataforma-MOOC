package domain

import (
	"context"
	"io"
	"time"
)

// ObjectInfo describes what the bucket actually holds at a key. The upload
// confirmation is what needs it: the client claims the transfer finished, and
// this is how the API checks that claim against the provider instead of
// trusting it.
type ObjectInfo struct {
	Key         string
	SizeBytes   int64
	ContentType string
}

// StorageProvider abstracts object storage operations (Cloud Storage, MinIO, etc.)
//
// The upload path never routes bytes through the API: the caller gets a
// presigned URL and transfers straight to the bucket. That is what keeps a
// 500 MB video off the API's memory and CPU, and it is the flow the delivery's
// capacity scenario measures end to end.
type StorageProvider interface {
	GeneratePresignedUploadURL(ctx context.Context, objectKey string, contentType string, expiresDuration time.Duration) (string, error)
	GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expiresDuration time.Duration) (string, error)

	// StatObject reports the stored object, or ErrObjectNotFound when the key
	// holds nothing.
	StatObject(ctx context.Context, objectKey string) (*ObjectInfo, error)

	DeleteObject(ctx context.Context, objectKey string) error

	// GetObject opens the stored object for reading, returning ErrObjectNotFound
	// when the key holds nothing.
	//
	// Only the worker uses it, to pull an original down for transcoding. The API
	// never does: a file that passes through the API is precisely what the
	// direct-upload design exists to avoid.
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)

	// PutObject writes bytes the server itself produced -- HLS manifests and
	// segments -- under objectKey. size may be -1 when it is not known ahead of
	// time.
	PutObject(ctx context.Context, objectKey, contentType string, r io.Reader, size int64) error
}

// TaskQueue abstracts asynchronous background job dispatching.
type TaskQueue interface {
	EnqueueTask(ctx context.Context, taskType string, payload []byte) error
}

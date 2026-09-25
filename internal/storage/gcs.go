package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// GCS is the Cloud Storage adapter, used by the deployed environment.
//
// Signing uses V4 and the credentials the client was built with, which on a VM
// means the attached service account: the API's account may sign and read, the
// worker's may write derivatives. That split is enforced by IAM on the bucket,
// not by this code, so a bug here cannot hand a client more rights than its
// service account holds.
type GCS struct {
	client *storage.Client
	bucket string
}

// NewGCS builds the adapter from Application Default Credentials. On a VM that
// is the attached service account; locally it is whatever gcloud auth
// application-default login wrote. No key file is ever read from the repo.
func NewGCS(ctx context.Context, bucket string) (*GCS, error) {
	if bucket == "" {
		return nil, errors.New("storage: bucket name is required")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: cloud storage client: %w", err)
	}
	return &GCS{client: client, bucket: bucket}, nil
}

// Close releases the underlying client.
func (g *GCS) Close() error {
	if g.client == nil {
		return nil
	}
	return g.client.Close()
}

// GeneratePresignedUploadURL issues a V4 signed URL the client PUTs to.
//
// contentType is bound into the signature, so a caller that asked to upload a
// video cannot use the same URL to store an HTML page and have the bucket serve
// it back under our origin.
func (g *GCS) GeneratePresignedUploadURL(ctx context.Context, objectKey, contentType string, expires time.Duration) (string, error) {
	if IsDerivativeKey(objectKey) {
		return "", fmt.Errorf("storage: refusing to sign an upload for derivative key %q", objectKey)
	}
	opts := &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      http.MethodPut,
		ContentType: contentType,
		Expires:     time.Now().Add(expires),
	}
	url, err := g.client.Bucket(g.bucket).SignedURL(objectKey, opts)
	if err != nil {
		return "", fmt.Errorf("storage: sign upload url: %w", err)
	}
	return url, nil
}

// GeneratePresignedDownloadURL issues a short-lived read URL. Media is served
// straight from the bucket in this stage -- no CDN -- so this is what a player
// ends up fetching.
func (g *GCS) GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  http.MethodGet,
		Expires: time.Now().Add(expires),
	}
	url, err := g.client.Bucket(g.bucket).SignedURL(objectKey, opts)
	if err != nil {
		return "", fmt.Errorf("storage: sign download url: %w", err)
	}
	return url, nil
}

// StatObject reports what the bucket holds, translating a missing object into
// domain.ErrObjectNotFound so callers can tell it apart from an outage.
func (g *GCS) StatObject(ctx context.Context, objectKey string) (*domain.ObjectInfo, error) {
	attrs, err := g.client.Bucket(g.bucket).Object(objectKey).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, domain.ErrObjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("storage: stat object: %w", err)
	}
	return &domain.ObjectInfo{
		Key:         objectKey,
		SizeBytes:   attrs.Size,
		ContentType: attrs.ContentType,
	}, nil
}

// DeleteObject removes a key. A key that is already gone is not an error: the
// caller wanted it absent, and it is.
func (g *GCS) DeleteObject(ctx context.Context, objectKey string) error {
	err := g.client.Bucket(g.bucket).Object(objectKey).Delete(ctx)
	if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return fmt.Errorf("storage: delete object: %w", err)
	}
	return nil
}

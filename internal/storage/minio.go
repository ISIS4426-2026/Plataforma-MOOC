package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO is the adapter for local development, where docker-compose.yml already
// runs a MinIO container. It exists so the upload flow can be exercised without
// a cloud project, and so the same code path -- sign, transfer direct, confirm
// -- is what developers run and what gets deployed.
type MinIO struct {
	client *minio.Client
	bucket string
}

// NewMinIO builds the adapter from the S3_* settings the compose file already
// injects.
func NewMinIO(endpoint, accessKey, secretKey, bucket string) (*MinIO, error) {
	if bucket == "" {
		return nil, errors.New("storage: bucket name is required")
	}
	host, secure, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: minio client: %w", err)
	}
	return &MinIO{client: client, bucket: bucket}, nil
}

// parseEndpoint splits the configured S3_ENDPOINT into the host:port and TLS
// flag minio-go expects, accepting it with or without a scheme.
func parseEndpoint(endpoint string) (host string, secure bool, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, errors.New("storage: endpoint is required")
	}
	if !strings.Contains(endpoint, "://") {
		return endpoint, false, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, fmt.Errorf("storage: invalid endpoint %q: %w", endpoint, err)
	}
	return u.Host, u.Scheme == "https", nil
}

// GeneratePresignedUploadURL issues a presigned PUT, mirroring the Cloud
// Storage adapter.
func (m *MinIO) GeneratePresignedUploadURL(ctx context.Context, objectKey, contentType string, expires time.Duration) (string, error) {
	if IsDerivativeKey(objectKey) {
		return "", fmt.Errorf("storage: refusing to sign an upload for derivative key %q", objectKey)
	}
	u, err := m.client.PresignedPutObject(ctx, m.bucket, objectKey, expires)
	if err != nil {
		return "", fmt.Errorf("storage: sign upload url: %w", err)
	}
	return u.String(), nil
}

// GeneratePresignedDownloadURL issues a presigned GET.
func (m *MinIO) GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	u, err := m.client.PresignedGetObject(ctx, m.bucket, objectKey, expires, url.Values{})
	if err != nil {
		return "", fmt.Errorf("storage: sign download url: %w", err)
	}
	return u.String(), nil
}

// StatObject reports the stored object, mapping a 404 to domain.ErrObjectNotFound.
func (m *MinIO) StatObject(ctx context.Context, objectKey string) (*domain.ObjectInfo, error) {
	info, err := m.client.StatObject(ctx, m.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return nil, domain.ErrObjectNotFound
		}
		return nil, fmt.Errorf("storage: stat object: %w", err)
	}
	return &domain.ObjectInfo{
		Key:         objectKey,
		SizeBytes:   info.Size,
		ContentType: info.ContentType,
	}, nil
}

// DeleteObject removes a key, treating an already-absent object as success.
func (m *MinIO) DeleteObject(ctx context.Context, objectKey string) error {
	err := m.client.RemoveObject(ctx, m.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil && minio.ToErrorResponse(err).StatusCode != http.StatusNotFound {
		return fmt.Errorf("storage: delete object: %w", err)
	}
	return nil
}

// GetObject opens the stored object for reading.
func (m *MinIO) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: open object %q: %w", objectKey, err)
	}
	// GetObject is lazy: it does not talk to the server until the first read, so
	// a missing key would otherwise surface as a read error much later, far from
	// the call that caused it.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return nil, domain.ErrObjectNotFound
		}
		return nil, fmt.Errorf("storage: open object %q: %w", objectKey, err)
	}
	return obj, nil
}

// PutObject writes an object the worker produced.
func (m *MinIO) PutObject(ctx context.Context, objectKey, contentType string, r io.Reader, size int64) error {
	if size <= 0 {
		size = -1 // minio-go streams with an unknown length when size is negative.
	}
	_, err := m.client.PutObject(ctx, m.bucket, objectKey, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("storage: write object %q: %w", objectKey, err)
	}
	return nil
}

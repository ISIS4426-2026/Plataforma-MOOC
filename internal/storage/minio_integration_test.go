package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// This exercises the whole round trip against a real MinIO: sign, transfer over
// plain HTTP the way a browser would, read back, and delete. The unit tests use
// doubles and therefore prove nothing about whether the signatures we produce
// are ones the server accepts -- which is the part most likely to be wrong.
//
// It skips when MinIO is not reachable, so `go test ./...` stays green on a
// machine without Docker. Bring it up with:
//
//	docker compose up -d minio
//
// and point S3_ENDPOINT at it if it is not on the default localhost:9000.
func TestMinIORoundTripIntegration(t *testing.T) {
	endpoint := envOr("S3_ENDPOINT", "http://localhost:9000")
	accessKey := envOr("S3_ACCESS_KEY", envOr("MINIO_ROOT_USER", "minioadmin"))
	secretKey := envOr("S3_SECRET_KEY", envOr("MINIO_ROOT_PASSWORD", "minioadmin"))
	bucket := envOr("S3_BUCKET", "mooc-storage")

	host, secure, err := parseEndpoint(endpoint)
	if err != nil {
		t.Fatalf("parseEndpoint(%q): %v", endpoint, err)
	}
	if !reachable(host) {
		t.Skipf("MinIO is not listening on %s; run `docker compose up -d minio` to exercise this test", host)
	}

	ctx := context.Background()

	// The bucket is normally created by the minio-init service in
	// docker-compose.yml; create it here too so the test does not depend on
	// that container having run.
	admin, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		t.Fatalf("minio admin client: %v", err)
	}
	exists, err := admin.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}
	if !exists {
		if err := admin.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("MakeBucket: %v", err)
		}
	}

	adapter, err := NewMinIO(endpoint, accessKey, secretKey, bucket)
	if err != nil {
		t.Fatalf("NewMinIO: %v", err)
	}

	const (
		stableID    = "11111111-2222-3333-4444-555555555555"
		contentType = "video/mp4"
	)
	body := []byte("not really an mp4, but the bytes round-trip the same")

	objectKey, err := OriginalKey(stableID, "clase.mp4")
	if err != nil {
		t.Fatalf("OriginalKey: %v", err)
	}
	t.Cleanup(func() { _ = adapter.DeleteObject(context.Background(), objectKey) })

	// 1. Sign, then transfer straight to the bucket -- no API in the path.
	uploadURL, err := adapter.GeneratePresignedUploadURL(ctx, objectKey, contentType, 10*time.Minute)
	if err != nil {
		t.Fatalf("GeneratePresignedUploadURL: %v", err)
	}
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build PUT: %v", err)
	}
	putReq.Header.Set("Content-Type", contentType)
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("direct upload: %v", err)
	}
	defer func() { _ = putResp.Body.Close() }()
	if putResp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(putResp.Body)
		t.Fatalf("direct upload returned %d: %s", putResp.StatusCode, strings.TrimSpace(string(detail)))
	}

	// 2. The confirmation step's evidence that the bytes actually landed.
	info, err := adapter.StatObject(ctx, objectKey)
	if err != nil {
		t.Fatalf("StatObject after upload: %v", err)
	}
	if info.SizeBytes != int64(len(body)) {
		t.Errorf("stored size = %d, want %d", info.SizeBytes, len(body))
	}
	if !strings.EqualFold(info.ContentType, contentType) {
		t.Errorf("stored content type = %q, want %q", info.ContentType, contentType)
	}

	// 3. Read it back through a signed URL, the way a player would.
	downloadURL, err := adapter.GeneratePresignedDownloadURL(ctx, objectKey, 10*time.Minute)
	if err != nil {
		t.Fatalf("GeneratePresignedDownloadURL: %v", err)
	}
	getResp, err := http.Get(downloadURL) //nolint:gosec // the URL is one this test just signed
	if err != nil {
		t.Fatalf("signed download: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("signed download returned %d", getResp.StatusCode)
	}
	got, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read downloaded body: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("downloaded %d bytes, want the %d uploaded", len(got), len(body))
	}

	// 4. Unsigned access must not work: the bucket is not a public origin.
	rawURL := strings.SplitN(downloadURL, "?", 2)[0]
	unsigned, err := http.Get(rawURL) //nolint:gosec // same host, deliberately unsigned
	if err != nil {
		t.Fatalf("unsigned request: %v", err)
	}
	defer func() { _ = unsigned.Body.Close() }()
	if unsigned.StatusCode == http.StatusOK {
		t.Error("the object was readable without a signature; the bucket is public")
	}

	// 5. Delete, and confirm the absence is reported as such.
	if err := adapter.DeleteObject(ctx, objectKey); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, err := adapter.StatObject(ctx, objectKey); !errors.Is(err, domain.ErrObjectNotFound) {
		t.Fatalf("StatObject after delete = %v, want ErrObjectNotFound", err)
	}

	// Deleting twice is not an error: the caller wanted it gone, and it is.
	if err := adapter.DeleteObject(ctx, objectKey); err != nil {
		t.Errorf("second DeleteObject: %v", err)
	}
}

// TestMinIORefusesToSignDerivativeUploads guards the rule that derivatives are
// written by the worker's credentials, never signed for a client.
func TestMinIORefusesToSignDerivativeUploads(t *testing.T) {
	adapter, err := NewMinIO("http://localhost:9000", "minioadmin", "minioadmin", "mooc-storage")
	if err != nil {
		t.Fatalf("NewMinIO: %v", err)
	}
	// No network is touched: the refusal happens before any request.
	_, err = adapter.GeneratePresignedUploadURL(context.Background(),
		HLSMasterKey("11111111-2222-3333-4444-555555555555"), "application/vnd.apple.mpegurl", time.Minute)
	if err == nil {
		t.Fatal("expected the adapter to refuse signing an upload into the derivatives prefix")
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func reachable(hostPort string) bool {
	conn, err := net.DialTimeout("tcp", hostPort, 750*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

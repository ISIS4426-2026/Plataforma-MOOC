package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGCSDirectUploadAndSignIntegration(t *testing.T) {
	bucket := envOr("STORAGE_BUCKET", envOr("S3_BUCKET", "plataforma-mooc-entrega2-media"))
	if envOr("STORAGE_BACKEND", "") != "gcs" && envOr("TEST_GCS", "") != "true" {
		t.Skip("skipping GCS live test; set STORAGE_BACKEND=gcs or TEST_GCS=true to run")
	}

	ctx := context.Background()
	gcs, err := NewGCS(ctx, bucket)
	if err != nil {
		t.Fatalf("NewGCS(%q): %v", bucket, err)
	}
	defer gcs.Close()

	objectKey := "originals/test-integration-" + time.Now().Format("20060102150405") + ".mp4"
	contentType := "video/mp4"

	// 1. Generate presigned upload URL
	uploadURL, err := gcs.GeneratePresignedUploadURL(ctx, objectKey, contentType, 15*time.Minute)
	if err != nil {
		t.Fatalf("GeneratePresignedUploadURL: %v", err)
	}

	t.Logf("Generated Upload URL: %s", uploadURL)
	if !strings.Contains(uploadURL, bucket) {
		t.Errorf("upload URL does not contain bucket %q: %s", bucket, uploadURL)
	}

	// 2. Direct upload PUT from client
	data := []byte("FAKE_MP4_CONTENT_FOR_INTEGRATION_TEST")
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("NewRequest PUT: %v", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP PUT to upload URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("HTTP PUT failed with status %d: %s", resp.StatusCode, string(body))
	}
	t.Logf("Direct upload succeeded with status %d", resp.StatusCode)

	// 3. StatObject
	info, err := gcs.StatObject(ctx, objectKey)
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if info.SizeBytes != int64(len(data)) {
		t.Errorf("StatObject size = %d, want %d", info.SizeBytes, len(data))
	}

	// 4. Presigned Download URL
	downloadURL, err := gcs.GeneratePresignedDownloadURL(ctx, objectKey, 15*time.Minute)
	if err != nil {
		t.Fatalf("GeneratePresignedDownloadURL: %v", err)
	}
	t.Logf("Generated Download URL: %s", downloadURL)

	getResp, err := http.Get(downloadURL)
	if err != nil {
		t.Fatalf("HTTP GET download URL: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("HTTP GET download URL failed with status %d: %s", getResp.StatusCode, string(body))
	}

	// 5. Clean up
	if err := gcs.DeleteObject(ctx, objectKey); err != nil {
		t.Errorf("DeleteObject: %v", err)
	}
}

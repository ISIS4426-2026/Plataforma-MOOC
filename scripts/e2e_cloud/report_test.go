package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResponseInstanceAndRequestID(t *testing.T) {
	hdr := make(http.Header)
	hdr.Set("X-Instance-ID", "mooc-api-vm-01")
	hdr.Set("X-Request-ID", "req-abc-123")
	res := &response{
		method: "GET",
		path:   "/api/v1/auth/sessions",
		status: http.StatusOK,
		header: hdr,
	}

	if got := res.instanceID(); got != "mooc-api-vm-01" {
		t.Fatalf("instanceID() = %q, want %q", got, "mooc-api-vm-01")
	}
	if got := res.requestID(); got != "req-abc-123" {
		t.Fatalf("requestID() = %q, want %q", got, "req-abc-123")
	}
}

func TestProbeSessionAndRevocationAcrossInstances(t *testing.T) {
	var seq atomic.Int64
	var revoked atomic.Bool
	instances := []string{"mooc-api-vm-a", "mooc-api-vm-b"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := seq.Add(1)
		inst := instances[int(n-1)%len(instances)]
		w.Header().Set("X-Instance-ID", inst)
		w.Header().Set("X-Request-ID", "req-test")
		w.Header().Set("Content-Type", "application/json")
		if revoked.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"unauthorized","message":"session revoked"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"id":"s1"}]}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	r := &run{
		api:     &client{base: srv.URL, hc: srv.Client()},
		log:     &logBuf,
		started: time.Now(),
	}
	r.flowIs("7. Continuidad de sesión, revocación y suspensión entre réplicas (E1)")

	seen, lastRes, ok := probeSessionAcrossInstances(r, "tok-1", 2, 8)
	if !ok || lastRes == nil {
		t.Fatalf("probeSessionAcrossInstances returned ok=%v, lastRes=%v", ok, lastRes)
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 distinct instances, got %v", seen)
	}

	revoked.Store(true)
	seenRev, lastRevRes, all401, total := probeRevokedAcrossInstances(r, "tok-1", seen, 2, 8)
	if !all401 || lastRevRes == nil || total < 4 {
		t.Fatalf("probeRevokedAcrossInstances returned all401=%v, total=%d", all401, total)
	}
	if len(seenRev) != 2 {
		t.Fatalf("expected 2 distinct instances on revocation, got %v", seenRev)
	}

	counts := r.instanceCounts()
	if len(counts) != 2 {
		t.Fatalf("instanceCounts() len = %d, want 2", len(counts))
	}

	r.recordJob(mediaJob{
		ResourceID:  "res-001",
		StableID:    "stable-001",
		Kind:        "video",
		ObjectKey:   "uploads/res-001/clase.mp4",
		Instance:    "mooc-api-vm-a",
		RequestID:   "req-202",
		FinalStatus: "completed",
		Elapsed:     12 * time.Second,
		Derivatives: "master.m3u8 + 2 variantes",
		Reconciled:  true,
	})

	outPath := filepath.Join(t.TempDir(), "resultados.md")
	if err := r.writeTable(outPath, "evidencias/E1/recorrido.log"); err != nil {
		t.Fatalf("writeTable failed: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	md := string(raw)
	for _, want := range []string{
		"Instancias de API observadas (`X-Instance-ID`)",
		"mooc-api-vm-a",
		"mooc-api-vm-b",
		"Reconciliación de trabajos multimedia aceptados",
		"res-001",
		"X-Request-ID",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("resultados.md missing %q", want)
		}
	}
}

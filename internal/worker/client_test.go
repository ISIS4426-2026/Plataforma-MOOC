package worker

import (
	"context"
	"testing"
)

func TestParseRedisOpt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string defaults to localhost:6379", "", false},
		{"simple host:port", "localhost:6379", false},
		{"redis URL scheme", "redis://localhost:6379/0", false},
		{"rediss URL scheme", "rediss://:secret@localhost:6379/1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := ParseRedisOpt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRedisOpt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if opt == nil {
				t.Fatalf("expected non-nil RedisConnOpt")
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client, err := NewClient("localhost:6379")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()
}

func TestClient_Enqueue_Validation(t *testing.T) {
	client, err := NewClient("localhost:6379")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	info, err := client.EnqueueTestPing(ctx, "unit test ping")
	if err != nil {
		t.Fatalf("EnqueueTestPing failed (ensure Redis is running): %v", err)
	}

	if info == nil || info.ID == "" {
		t.Errorf("expected non-empty task info ID")
	}
	if info.Type != "test:ping" {
		t.Errorf("expected task type test:ping, got %s", info.Type)
	}
}

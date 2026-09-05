package worker

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultExponentialBackoff(t *testing.T) {
	tests := []struct {
		n        int
		expected time.Duration
	}{
		{n: 0, expected: 2 * time.Second},
		{n: 1, expected: 4 * time.Second},
		{n: 2, expected: 8 * time.Second},
		{n: 3, expected: 16 * time.Second},
		{n: 4, expected: 32 * time.Second},
	}

	err := errors.New("test failure")

	for _, tt := range tests {
		got := DefaultExponentialBackoff(tt.n, err, nil)
		if got != tt.expected {
			t.Errorf("DefaultExponentialBackoff(%d) = %v; want %v", tt.n, got, tt.expected)
		}
	}
}

func TestDefaultExponentialBackoff_Cap(t *testing.T) {
	got := DefaultExponentialBackoff(20, nil, nil)
	maxExpected := 10 * time.Minute
	if got > maxExpected {
		t.Errorf("DefaultExponentialBackoff capped check failed: got %v, want max %v", got, maxExpected)
	}
}

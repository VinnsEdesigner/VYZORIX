package crypto

import (
	"strconv"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
)

func TestCommandSigner_ValidateTimestamp(t *testing.T) {
	signer := NewCommandSigner()
	nowMs := time.Now().UnixMilli()

	tests := []struct {
		name      string
		offsetMs  int64
		maxDrift  int64
		wantValid bool
	}{
		{"fresh timestamp", 0, 30_000, true},
		{"10s ago", 10_000, 30_000, true},
		{"29s ago", 29_000, 30_000, true},
		{"31s ago", 31_000, 30_000, false},
		{"1min ago", 60_000, 30_000, false},
		{"10s in future", -10_000, 30_000, true},
		{"31s in future", -31_000, 30_000, false},
		{"custom window 60s", 55_000, 60_000, true},
		{"custom window 60s too old", 65_000, 60_000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &command.CommandFrame{
				Timestamp: nowMs + tt.offsetMs,
			}
			got := signer.ValidateTimestamp(frame, tt.maxDrift)
			if got != tt.wantValid {
				t.Errorf("ValidateTimestamp() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestCommandSigner_GenerateNonce(t *testing.T) {
	signer := NewCommandSigner()

	nonce1, err := signer.GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}

	nonce2, err := signer.GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}

	if len(nonce1) != 32 {
		t.Errorf("nonce1 length = %d, want 32", len(nonce1))
	}

	if nonce1 == nonce2 {
		t.Error("GenerateNonce() should produce unique nonces")
	}
}

func TestCommandSigner_GenerateTimestamp(t *testing.T) {
	signer := NewCommandSigner()

	before := time.Now().Unix()
	ts := signer.GenerateTimestamp()
	after := time.Now().Unix()

	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		t.Fatalf("GenerateTimestamp() returned non-numeric: %v", err)
	}

	if tsInt < before || tsInt > after {
		t.Errorf("GenerateTimestamp() = %d, want between %d and %d", tsInt, before, after)
	}
}

func TestCommandSigner_GenerateTimestampMs(t *testing.T) {
	signer := NewCommandSigner()

	before := time.Now().UnixMilli()
	ts := signer.GenerateTimestampMs()
	after := time.Now().UnixMilli()

	if ts < before || ts > after {
		t.Errorf("GenerateTimestampMs() = %d, want between %d and %d", ts, before, after)
	}
}

// Edge case tests.

func TestCommandSigner_TimestampBoundary(t *testing.T) {
	signer := NewCommandSigner()
	nowMs := time.Now().UnixMilli()

	// Test exactly at boundary.
	frame := &command.CommandFrame{
		Timestamp: nowMs + 30_000, // Exactly 30s in future.
	}

	if !signer.ValidateTimestamp(frame, 30_000) {
		t.Error("Timestamp exactly at boundary should be valid")
	}

	frame.Timestamp = nowMs + 30_001 // One ms over.
	if signer.ValidateTimestamp(frame, 30_000) {
		t.Error("Timestamp one ms over boundary should be invalid")
	}
}

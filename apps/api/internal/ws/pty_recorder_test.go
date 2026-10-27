package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorderWritesValidCast(t *testing.T) {
	dir := t.TempDir()

	rec, err := NewPTYRecorder(dir, "s1", 80, 24, 0)
	if err != nil {
		t.Fatalf("NewPTYRecorder: %v", err)
	}

	if !rec.WriteOutput([]byte("hello ")) {
		t.Fatal("first write must succeed")
	}
	if !rec.WriteOutput([]byte("world\n")) {
		t.Fatal("second write must succeed")
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(rec.Path())
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("cast has %d lines, want 1 header + 2 events", len(lines))
	}

	// Header must be valid asciinema v2 JSON.
	var header castHeader
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("header is not valid JSON: %v", err)
	}
	if header.Version != 2 {
		t.Fatalf("version = %d, want 2", header.Version)
	}
	if header.Width != 80 || header.Height != 24 {
		t.Fatalf("size = %dx%d, want 80x24", header.Width, header.Height)
	}

	// Each event is [time, "o", data].
	for i, line := range lines[1:] {
		var event []any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("event %d is not valid JSON: %v", i, err)
		}
		if len(event) != 3 || event[1] != "o" {
			t.Fatalf("event %d = %v, want [time, o, data]", i, event)
		}
	}

	if !strings.Contains(string(raw), "hello ") || !strings.Contains(string(raw), "world") {
		t.Fatal("cast must contain the recorded output")
	}
}

func TestRecorderEnforcesSizeCap(t *testing.T) {
	dir := t.TempDir()

	// A cap smaller than a single event forces the second write to be refused.
	rec, err := NewPTYRecorder(dir, "s1", 80, 24, 1)
	if err != nil {
		t.Fatalf("NewPTYRecorder: %v", err)
	}
	defer func() { _ = rec.Close() }()

	if rec.WriteOutput([]byte("x")) {
		t.Fatal("a write past the size cap must be refused")
	}
}

func TestRecorderCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	rec, err := NewPTYRecorder(dir, "s1", 80, 24, 0)
	if err != nil {
		t.Fatalf("NewPTYRecorder: %v", err)
	}

	if err := rec.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("second close must be a no-op, got %v", err)
	}
}

func TestRecorderWriteAfterCloseIsRefused(t *testing.T) {
	dir := t.TempDir()

	rec, err := NewPTYRecorder(dir, "s1", 80, 24, 0)
	if err != nil {
		t.Fatalf("NewPTYRecorder: %v", err)
	}
	_ = rec.Close()

	if rec.WriteOutput([]byte("x")) {
		t.Fatal("a write after close must be refused, not panic")
	}
}

func TestRecorderEmptyWriteIsNoOp(t *testing.T) {
	dir := t.TempDir()

	rec, err := NewPTYRecorder(dir, "s1", 80, 24, 0)
	if err != nil {
		t.Fatalf("NewPTYRecorder: %v", err)
	}
	defer func() { _ = rec.Close() }()

	if !rec.WriteOutput(nil) {
		t.Fatal("an empty write must report success without writing")
	}
}

func TestSweepRecordingsRemovesOldFiles(t *testing.T) {
	dir := t.TempDir()

	old := filepath.Join(dir, "old.cast")
	if err := os.WriteFile(old, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write old cast: %v", err)
	}
	// Backdate it well past the max age.
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	recent := filepath.Join(dir, "recent.cast")
	if err := os.WriteFile(recent, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write recent cast: %v", err)
	}

	removed, err := SweepRecordings(dir, 24*time.Hour)
	if err != nil {
		t.Fatalf("SweepRecordings: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("the old cast must be gone")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatal("the recent cast must survive")
	}
}

func TestSweepRecordingsMissingDirIsNotAnError(t *testing.T) {
	removed, err := SweepRecordings(filepath.Join(t.TempDir(), "nope"), time.Hour)
	if err != nil {
		t.Fatalf("a missing directory must not be an error, got %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
}

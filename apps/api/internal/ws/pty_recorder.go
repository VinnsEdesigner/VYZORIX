// Package hub provides WebSocket functionality.
package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PTY recording limits. Recording is opt-in per session; when enabled, the
// session's output is written as an asciinema v2 cast file.
const (
	// DefaultPTYRecordingMaxBytes rotates a cast file once it exceeds this size,
	// so a long session cannot fill the disk.
	// 50 MiB.
	DefaultPTYRecordingMaxBytes int64 = 50 << 20

	// DefaultPTYRecordingMaxAge bounds a cast file's lifetime. A recording older
	// than this is deleted on the next rotation sweep.
	DefaultPTYRecordingMaxAge = 7 * 24 * time.Hour
)

// asciinema v2 cast header. The format is a JSON header line followed by
// newline-delimited JSON event arrays: [time, "o", "data"]. It is plain text,
// greppable, and replayable with `asciinema play`.
type castHeader struct {
	Env       map[string]string `json:"env,omitempty"`
	Timestamp int64             `json:"timestamp"`
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
}

// PTYRecorder writes one session's output to an asciinema cast file.
//
// Recording is deliberately server-side: the device cannot be trusted to record
// itself, and the operator's browser is the wrong place to keep an audit copy.
// Only output is recorded (what the operator saw), not keystrokes, which keeps
// secrets typed into a prompt out of the cast.
type PTYRecorder struct {
	file      *os.File
	started   time.Time
	path      string
	sessionID string
	bytes     int64
	maxBytes  int64
	mu        sync.Mutex
	closed    bool
}

// NewPTYRecorder opens a cast file for a session. dir must exist and be writable;
// the caller owns the directory's lifetime.
func NewPTYRecorder(dir, sessionID string, cols, rows int, maxBytes int64) (*PTYRecorder, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultPTYRecordingMaxBytes
	}

	path := filepath.Join(dir, sessionID+".cast")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open cast file: %w", err)
	}

	now := time.Now()
	rec := &PTYRecorder{
		file:      file,
		path:      path,
		sessionID: sessionID,
		started:   now,
		maxBytes:  maxBytes,
	}

	header, err := json.Marshal(castHeader{
		Version:   2,
		Width:     cols,
		Height:    rows,
		Timestamp: now.Unix(),
		Env:       map[string]string{"TERM": "xterm-256color"},
	})
	if err != nil {
		_ = file.Close()

		return nil, err
	}

	if _, err := file.Write(append(header, '\n')); err != nil {
		_ = file.Close()

		return nil, err
	}

	rec.bytes = int64(len(header)) + 1

	return rec, nil
}

// WriteOutput appends a chunk of terminal output. Frames past the size cap are
// dropped and the recorder reports false, so a runaway producer cannot fill the
// disk; the caller can then decide to stop recording or close the session.
func (r *PTYRecorder) WriteOutput(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed || r.bytes >= r.maxBytes {
		return false
	}

	elapsed := time.Since(r.started).Seconds()
	event, err := json.Marshal([]any{elapsed, "o", string(data)})
	if err != nil {
		return false
	}

	event = append(event, '\n')
	if _, err := r.file.Write(event); err != nil {
		return false
	}

	r.bytes += int64(len(event))

	return true
}

// Path returns the cast file's path.
func (r *PTYRecorder) Path() string { return r.path }

// Bytes returns how many bytes have been written.
func (r *PTYRecorder) Bytes() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.bytes
}

// Close flushes and closes the file. It is idempotent.
func (r *PTYRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	return r.file.Close()
}

// SweepRecordings deletes cast files older than maxAge from dir. It returns the
// number removed. A missing directory is not an error: recording may be disabled.
func SweepRecordings(dir string, maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cast" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

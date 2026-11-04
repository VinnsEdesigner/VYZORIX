package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
)

// newCommandTestDB opens an in-memory database with the commands schema the
// migration creates, plus the columns the repository selects.
func newCommandTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`CREATE TABLE commands (
		id TEXT PRIMARY KEY, device_id TEXT NOT NULL, dispatch_id TEXT NOT NULL,
		command TEXT NOT NULL, args TEXT, status TEXT NOT NULL,
		delivered_at INTEGER, completed_at INTEGER,
		created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL,
		failure_reason TEXT, result TEXT,
		retry_count INTEGER NOT NULL DEFAULT 0, max_retries INTEGER NOT NULL DEFAULT 3,
		next_retry_at INTEGER, wake_sent INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	return db
}

// TestCommandReadHandlesATimestampWrittenAsTime is the guard on the bug that made
// every command result fail to record: the repository writes created_at/updated_at
// as a time.Time, which the driver stores as a TEXT timestamp, but the read scanned
// the column straight into time.Time and failed with "unsupported Scan, storing
// driver.Value type string into type *time.Time". The server logged
// "failed to mark command delivered" and the command stayed pending forever.
func TestCommandReadHandlesATimestampWrittenAsTime(t *testing.T) {
	db := newCommandTestDB(t)
	repo := NewCommandRepository(db)
	ctx := context.Background()

	now := time.Now()
	cmd := &command.Command{
		ID:         "cmd-1",
		DeviceID:   "dev-1",
		DispatchID: "d1",
		Command:    "SHELL_EXEC",
		Status:     command.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := repo.Create(ctx, cmd); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByID(ctx, "cmd-1")
	if err != nil {
		t.Fatalf("a command written with a time.Time must be readable: %v", err)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at must be parsed, not left zero")
	}
}

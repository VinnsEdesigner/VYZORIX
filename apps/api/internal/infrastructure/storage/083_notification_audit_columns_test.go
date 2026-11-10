package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestAddNotificationAuditColumns covers the failure that shipped: the table was
// created without `device_id`, `success` and `error_msg`, but the audit repository
// inserts all three, so every audit write failed with "no such column" and the
// notification path logged and moved on.
func TestAddNotificationAuditColumns(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// The table as migration 26 created it — without the three columns.
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE notification_audit_log (
			id          TEXT PRIMARY KEY,
			operator_id TEXT NOT NULL,
			event_type  TEXT NOT NULL,
			channel     TEXT NOT NULL,
			payload     TEXT,
			sent_at     INTEGER NOT NULL
		)
	`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// The insert the repository performs must fail before the migration.
	insert := `INSERT INTO notification_audit_log
		(id, operator_id, event_type, channel, device_id, payload, success, error_msg, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := db.ExecContext(context.Background(), insert,
		"a1", "op1", "device_online", "push", "dev1", "{}", true, "", 1,
	); err == nil {
		t.Fatal("the insert should fail before the migration adds the columns")
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := migrateAddNotificationAuditColumns(tx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// After the migration the same insert succeeds.
	if _, err := db.ExecContext(context.Background(), insert,
		"a1", "op1", "device_online", "push", "dev1", "{}", true, "", 1,
	); err != nil {
		t.Fatalf("insert after migration: %v", err)
	}

	// And it is idempotent: running it again is a no-op, not a duplicate-column error.
	tx2, err := db.Begin()
	if err != nil {
		t.Fatalf("begin 2: %v", err)
	}
	if err := migrateAddNotificationAuditColumns(tx2); err != nil {
		t.Fatalf("re-running the migration must be a no-op: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit 2: %v", err)
	}
}

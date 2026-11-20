package storage

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestMigration96AddsUserDisabled guards the same defect class as migration 087:
// the users table is created with `CREATE TABLE IF NOT EXISTS`, so a column
// added to that CREATE would never appear on an already-migrated database. Only
// the ALTER in 096 adds `disabled_at`, so the test starts from a database
// migrated up to just before it.
func TestMigration96AddsUserDisabled(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, m := range migrations {
		if m.Version >= 96 {
			break
		}
		applyTx(t, db, m)
	}

	if columnExists(t, db, "users", "disabled_at") {
		t.Fatal("precondition failed: disabled_at should not exist before migration 96")
	}

	for _, m := range migrations {
		if m.Version == 96 {
			applyTx(t, db, m)
		}
	}

	if !columnExists(t, db, "users", "disabled_at") {
		t.Fatal("migration 96 did not add the users.disabled_at column")
	}
}

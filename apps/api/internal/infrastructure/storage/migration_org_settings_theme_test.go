package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestMigration87AddsOrgSettingsTheme runs the real migration registry against a
// database that has already been migrated up to the version before this one.
//
// This is the regression guard for the defect it fixes: `organization_settings`
// is created by migration 041 with `CREATE TABLE IF NOT EXISTS`, so on any
// database where that table already exists the CREATE is a no-op and a column
// added to it there is never created. Only the ALTER in 087 adds `theme`. A test
// that starts from a fresh database would not catch a regression here, because
// the CREATE would build the column directly.
func TestMigration87AddsOrgSettingsTheme(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Migrate to just before 087, so organization_settings exists without theme.
	for _, m := range migrations {
		if m.Version >= 87 {
			break
		}
		applyTx(t, db, m)
	}

	if columnExists(t, db, "organization_settings", "theme") {
		t.Fatal("precondition failed: theme column should not exist before migration 87")
	}

	// Apply 087 on top.
	for _, m := range migrations {
		if m.Version == 87 {
			applyTx(t, db, m)
		}
	}

	if !columnExists(t, db, "organization_settings", "theme") {
		t.Fatal("migration 87 did not add the organization_settings.theme column")
	}
}

// applyTx runs one migration the way runMigrations does, so the test exercises
// the real Apply function rather than a copy.
func applyTx(t *testing.T, db *sql.DB, m Migration) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin %d: %v", m.Version, err)
	}
	if err := m.Apply(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("apply %d (%s): %v", m.Version, m.Name, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit %d: %v", m.Version, err)
	}
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid          int
			name, ctype  string
			notNull, pk  int
			defaultValue sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

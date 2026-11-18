package storage

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestMigration89AddsOrgSettingsWeekStart is the regression guard for the same
// defect class as 087: `organization_settings` is created by migration 041 with
// `CREATE TABLE IF NOT EXISTS`, so on any database where that table already
// exists the CREATE is a no-op and a column added to it there is never created.
// Only the ALTER in 089 adds `week_start`. Starting from a fresh database would
// not catch a regression, because the CREATE would build the column directly.
func TestMigration89AddsOrgSettingsWeekStart(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Migrate to just before 089, so organization_settings exists without week_start.
	for _, m := range migrations {
		if m.Version >= 89 {
			break
		}
		applyTx(t, db, m)
	}

	if columnExists(t, db, "organization_settings", "week_start") {
		t.Fatal("precondition failed: week_start should not exist before migration 89")
	}

	for _, m := range migrations {
		if m.Version == 89 {
			applyTx(t, db, m)
		}
	}

	if !columnExists(t, db, "organization_settings", "week_start") {
		t.Fatal("migration 89 did not add the organization_settings.week_start column")
	}
}

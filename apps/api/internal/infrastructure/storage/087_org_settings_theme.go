package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddOrgSettingsTheme adds the organisation's default UI theme id.
//
// This is a separate migration rather than a line in the CREATE TABLE in 041:
// that table already exists on every deployed database, so `CREATE TABLE IF NOT
// EXISTS` is a no-op and a new column added there would never be created. The
// ALTER is what actually adds it; the duplicate-column guard keeps the migration
// idempotent on a database that already has it.
//
// The default is empty, not 'dark': an organisation with no explicit choice must
// contribute nothing to the theme cascade, so the system default (dark) applies
// at the bottom. Defaulting the column to 'dark' would shadow that tier.
func migrateAddOrgSettingsTheme(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE organization_settings ADD COLUMN theme TEXT NOT NULL DEFAULT ''`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

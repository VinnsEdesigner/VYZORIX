package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddOrgSettingsWeekStart adds the organisation's default week start.
//
// A separate migration rather than a line in the CREATE TABLE in 041: that table
// already exists on every deployed database, so `CREATE TABLE IF NOT EXISTS` is a
// no-op and a new column added there would never be created.
//
// The default is empty, not 'monday': an organisation with no explicit choice
// must contribute nothing to the preference cascade, so the system default
// applies at the bottom. Defaulting the column would shadow that tier.
func migrateAddOrgSettingsWeekStart(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE organization_settings ADD COLUMN week_start TEXT NOT NULL DEFAULT ''`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

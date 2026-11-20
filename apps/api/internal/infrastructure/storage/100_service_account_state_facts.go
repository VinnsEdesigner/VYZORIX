package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateServiceAccountStateFacts records who changed a service account's
// enabled state, when, and why.
//
// The service account row previously carried only `enabled`, so a disabled
// account could say *that* it was disabled and nothing else. An operator looking
// at a disabled account needs the rest: when it happened, who did it, and the
// stated reason — otherwise the only way to find out is the audit log, which is
// not where someone looks when staring at the account.
//
// Separate migration because the table already exists on deployed databases, so
// a column added to the CREATE TABLE in 068 would never be created.
func migrateServiceAccountStateFacts(tx *sql.Tx) error {
	ctx := context.Background()

	// Each column is added independently: SQLite has no
	// "ADD COLUMN IF NOT EXISTS", so a partially-migrated database must not fail
	// the whole migration on a column it already has.
	for _, stmt := range []string{
		`ALTER TABLE service_accounts ADD COLUMN disabled_at INTEGER`,
		`ALTER TABLE service_accounts ADD COLUMN disabled_by TEXT`,
		`ALTER TABLE service_accounts ADD COLUMN enabled_by TEXT`,
		`ALTER TABLE service_accounts ADD COLUMN disabled_reason TEXT`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil &&
			!strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}

	return nil
}

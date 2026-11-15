package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddInstanceRole adds the user's deployment-wide role: the third grant
// store, alongside the implicit personal grants and the org membership.
//
// It is deliberately not folded into the org role ladder. An instance admin is
// not an admin of every organisation — making it the top of that ladder would
// assert exactly that, and one flag would silently grant every tenant's
// telemetry. Authority across organisations is a separate axis.
//
// Existing rows default to 'none' (no instance authority), so the migration
// grants no one new power. A fresh database already creates the column (the
// CREATE TABLE in the users migration was updated), so the ALTER is a no-op
// there and the duplicate-column error is tolerated, as in the other
// add-column migrations.
func migrateAddInstanceRole(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE users ADD COLUMN instance_role TEXT NOT NULL DEFAULT 'none'`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddUserLastLogin adds the timestamp of the user's most recent login.
//
// A separate migration rather than a line in the CREATE TABLE: the users table
// already exists on every deployed database, so `CREATE TABLE IF NOT EXISTS`
// there is a no-op and a new column added to it would never be created.
//
// NULL means "never recorded" — an account created before this migration has no
// known last login, and the profile shows a dash rather than a fabricated time.
func migrateAddUserLastLogin(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE users ADD COLUMN last_login_at INTEGER`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

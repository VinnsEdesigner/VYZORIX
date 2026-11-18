package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddUserAvatarURL adds the user's avatar image URL.
//
// A separate migration rather than a line in the CREATE TABLE: the users table
// already exists on every deployed database, so `CREATE TABLE IF NOT EXISTS`
// there is a no-op and a new column added to it would never be created.
func migrateAddUserAvatarURL(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT ''`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

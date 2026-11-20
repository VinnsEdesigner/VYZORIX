package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddUserDisabled adds the account-disable timestamp; NULL means active.
//
// A separate migration because the users table already exists on deployed
// databases, so a column added to its CREATE TABLE would never be created.
func migrateAddUserDisabled(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE users ADD COLUMN disabled_at INTEGER`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

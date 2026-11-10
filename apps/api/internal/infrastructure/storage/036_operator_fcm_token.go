package storage

import (
	"context"
	"database/sql"
)

// migrateOperatorFCMToken adds fcm_token column to operators table.
// for storing operator FCM tokens for push notifications.
func migrateOperatorFCMToken(tx *sql.Tx) error {
	ctx := context.Background()

	// Add fcm_token column to operators table if it doesn't exist.
	// SQLite doesn't support ADD COLUMN IF NOT EXISTS, so we check first.
	query := `
	ALTER TABLE operators ADD COLUMN fcm_token TEXT`

	_, err := tx.ExecContext(ctx, query)
	if err != nil && !isDuplicateColumnErr(err) {
		// Any other failure is real. The duplicate-column case is matched by a
		// substring because the error text is driver-dependent: the libSQL/Turso
		// driver prefixes it with "SQLite error: ".
		return err
	}

	return nil
}

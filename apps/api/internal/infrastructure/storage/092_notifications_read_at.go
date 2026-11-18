package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateNotificationsReadAt adds the per-user notification read watermark.
//
// The notification history is an audit trail with no per-row read flag; "unread"
// is derived from whether an entry is newer than the user's last "mark all read"
// timestamp. NULL means the user has never marked anything read, so everything
// is treated as read (an empty history shows no badge).
func migrateNotificationsReadAt(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE user_settings ADD COLUMN notifications_read_at INTEGER`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}

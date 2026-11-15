package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddNotificationAuditColumns adds the columns the audit repository writes
// but the original table never declared.
//
// The table was created with `id, user_id, event_type, channel, payload,
// sent_at`, but `notification.Repository.LogEntry` inserts `device_id, success,
// error_msg` as well. Every audit write therefore failed with "table
// notification_audit_log has no column named device_id", and because the caller
// only logs the error, notifications were sent with no audit trail and nothing
// surfaced until it was read in the server log.
//
// `device_id` is nullable: not every notification concerns a device. `success`
// defaults to 0 and `error_msg` is nullable; both are supplied on every insert, so
// the defaults only matter for rows written before this migration.
func migrateAddNotificationAuditColumns(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE notification_audit_log ADD COLUMN device_id TEXT`,
		`ALTER TABLE notification_audit_log ADD COLUMN success INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE notification_audit_log ADD COLUMN error_msg TEXT`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			// Re-running the migration on a database that already has the column is
			// not an error: SQLite has no `ADD COLUMN IF NOT EXISTS`, so the
			// duplicate-column error is the idempotency signal.
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}
	}

	return nil
}

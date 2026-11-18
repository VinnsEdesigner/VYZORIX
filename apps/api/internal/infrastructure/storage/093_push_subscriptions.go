package storage

import (
	"context"
	"database/sql"
)

// migratePushSubscriptions creates the table of browser push subscriptions.
//
// One row per browser/device a user enabled Web Push on. The endpoint and keys
// come from the browser's PushSubscription.toJSON(); a user may have several
// (one per browser). The endpoint is the unique key: re-subscribing the same
// browser updates its keys rather than duplicating.
func migratePushSubscriptions(tx *sql.Tx) error {
	_, err := tx.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS push_subscriptions (
			endpoint   TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			p256dh     TEXT NOT NULL,
			auth       TEXT NOT NULL,
			user_agent TEXT,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
		ON push_subscriptions(user_id)
	`)
	return err
}

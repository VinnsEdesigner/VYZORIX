package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateSlackNotifications adds every column the notification-channel work
// needs, in one migration, because it is one unshipped feature:
//
//   - notify_email_address: a custom "from me" address. Empty means "use the
//     account email", so the default path needs no backfill.
//   - notify_slack: the slack channel flag. Channels are persisted as one boolean
//     column per channel (notify_email, notify_push, notify_webhook) and rebuilt
//     from those flags on read, so a channel without a column would be dropped on
//     write and never reappear.
//   - slack_*: the result of the Slack OAuth link-up (team, user, bot token and
//     the target channel). All empty until the user connects.
//   - oauth_states.user_id: the user who started a link-up from an authenticated
//     session, so the callback can attach the workspace to the right account.
//
// Columns are added by ALTER rather than in CREATE TABLE: user_settings and
// oauth_states already exist on every deployed database, so CREATE TABLE IF NOT
// EXISTS there is a no-op and a new column would never be created. Each ALTER
// tolerates "duplicate column" so the migration is safe to re-run.
func migrateSlackNotifications(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE user_settings ADD COLUMN notify_email_address TEXT`,
		`ALTER TABLE user_settings ADD COLUMN notify_slack INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE user_settings ADD COLUMN slack_enabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE user_settings ADD COLUMN slack_team_id TEXT`,
		`ALTER TABLE user_settings ADD COLUMN slack_team_name TEXT`,
		`ALTER TABLE user_settings ADD COLUMN slack_user_id TEXT`,
		`ALTER TABLE user_settings ADD COLUMN slack_access_token TEXT`,
		`ALTER TABLE user_settings ADD COLUMN slack_channel_id TEXT`,
		`ALTER TABLE user_settings ADD COLUMN slack_channel_name TEXT`,
		`ALTER TABLE oauth_states ADD COLUMN user_id TEXT`,
	}

	for _, statement := range statements {
		if _, err := tx.ExecContext(context.Background(), statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return err
		}
	}

	return nil
}

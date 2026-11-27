package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateProvisioningEventDismissal adds the column that hides an audit event from the
// activity list without deleting it.
//
// The activity trail answers "who changed this, and was it a person or a machine". Deleting a
// row destroys that answer permanently, so a user who wants to clear noise must not be given a
// button that removes evidence. Dismissing hides the row from the list and records nothing
// else; the row, its actor and its detail stay in the table.
func migrateProvisioningEventDismissal(tx *sql.Tx) error {
	ctx := context.Background()

	for _, stmt := range []string{
		// Nullable: NULL means "not dismissed", which is the state of every existing row.
		`ALTER TABLE provisioning_events ADD COLUMN dismissed_at INTEGER`,
		// The list filters on this, so the common query does not scan dismissed rows.
		`CREATE INDEX IF NOT EXISTS idx_provisioning_events_org_dismissed
		     ON provisioning_events(org_id, dismissed_at)`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil &&
			!strings.Contains(err.Error(), "duplicate column") &&
			!strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

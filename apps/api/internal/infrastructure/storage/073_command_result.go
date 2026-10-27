package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddCommandResult adds a result column so a completed command can carry
// the output a device reports (for example SHELL_EXEC stdout). It is deliberately
// separate from failure_reason, which is reserved for failures: a successful
// command with its output stored under a "failure reason" would read as a failed
// command in the dashboard and the command history API.
func migrateAddCommandResult(tx *sql.Tx) error {
	_, err := tx.ExecContext(context.Background(), `
		ALTER TABLE commands ADD COLUMN result TEXT
	`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

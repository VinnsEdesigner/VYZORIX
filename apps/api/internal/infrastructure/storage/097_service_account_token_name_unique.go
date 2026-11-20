package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateServiceAccountTokenNameUnique enforces one token name per service account.
// The scope is the account, not the org: two accounts may each own a "ci" token.
func migrateServiceAccountTokenNameUnique(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_service_account_tokens_service_name
		 ON service_account_tokens(service_id, name)`,
	)
	// An index that cannot be created because existing rows already collide is
	// not a reason to refuse to start: the service layer rejects new duplicates
	// regardless. Surface it and carry on rather than bricking the migration.
	if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return err
	}

	return nil
}

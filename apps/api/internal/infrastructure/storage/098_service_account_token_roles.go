package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateServiceAccountTokenRoles adds basic_role and roles. Both are nullable, so
// a pre-existing token keeps exactly its coarse scopes.
func migrateServiceAccountTokenRoles(tx *sql.Tx) error {
	ctx := context.Background()

	if _, err := tx.ExecContext(ctx,
		`ALTER TABLE service_account_tokens ADD COLUMN basic_role TEXT`,
	); err != nil && !isDuplicateColumn(err) {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`ALTER TABLE service_account_tokens ADD COLUMN roles TEXT`,
	); err != nil && !isDuplicateColumn(err) && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

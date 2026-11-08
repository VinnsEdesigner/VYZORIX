package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateCreateDeployCredential creates the device-scoped deploy credential
// store (VYZORIX_DEPLOY_CONTRACT.md §7.4).
//
// Only `token_hash` is stored. The plaintext credential exists once, at mint
// time, and is never written here or returned by any read, so a database copy
// does not yield a usable credential.
//
// `device_id` is a plain column, not a foreign key to `devices(id)`, to match the
// sibling deploy tables (078/079), which are also device-scoped without a FK.
func migrateCreateDeployCredential(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS device_deploy_credentials (
			id          TEXT NOT NULL PRIMARY KEY,
			org_id      TEXT NOT NULL,
			device_id   TEXT NOT NULL,
			name        TEXT NOT NULL,
			prefix      TEXT NOT NULL,
			token_hash  TEXT NOT NULL UNIQUE,
			created_by  TEXT NOT NULL,
			created_at  INTEGER NOT NULL,
			last_used_at INTEGER,
			revoked_at  INTEGER
		)`,
		// A credential is looked up by its hash and listed by its device; both are
		// indexed so neither path scans.
		`CREATE INDEX IF NOT EXISTS idx_deploy_credentials_device ON device_deploy_credentials (device_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue
			}

			return err
		}
	}

	return nil
}

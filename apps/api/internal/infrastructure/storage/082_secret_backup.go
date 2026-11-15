package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateCreateSecretBackup creates the user secret-backup store.
//
// This holds the deployment's own secrets — the server signing key, the APK
// keystore, the admin password, the generated HMAC/JWT secrets — so they survive
// the sandbox that produced them. Without a table they lived only in a workspace
// directory, which is wiped between sessions, and losing the APK keystore means
// no future update can install over an existing install (Android rejects a
// different signing key).
//
// `value_b64` is base64 for a reason: the keystore is binary and the PEM keys are
// multi-line, and base64 makes both round-trip exactly and makes a restore's
// encoding explicit rather than implicit.
//
// The table is deliberately NOT reachable from any API route. It exists for
// out-of-band restore; nothing in the server reads it at runtime.
func migrateCreateSecretBackup(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS secret_backup (
                        name       TEXT NOT NULL PRIMARY KEY,
                        value_b64  TEXT NOT NULL,
                        note       TEXT NOT NULL DEFAULT '',
                        updated_at INTEGER NOT NULL
                )`,
		`CREATE INDEX IF NOT EXISTS idx_secret_backup_updated ON secret_backup (updated_at)`,
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

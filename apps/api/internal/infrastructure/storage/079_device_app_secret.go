package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateCreateDeviceAppSecret creates the per-device secret store.
//
// The ciphertext column holds the value already wrapped to the device's public
// key (RSA-OAEP-SHA1). The server stores only that: a server compromise that
// reaches this table yields nothing for a device whose private key the attacker
// does not hold (§9b.7). There is deliberately no plaintext column and no
// column for the device's private key.
//
// The primary key is (device_id, app, key): a secret is scoped to one device,
// so the same key on two devices is two independent secrets.
func migrateCreateDeviceAppSecret(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS device_app_secrets (
			device_id  TEXT NOT NULL,
			app        TEXT NOT NULL,
			key        TEXT NOT NULL,
			ciphertext TEXT NOT NULL,
			generation INTEGER NOT NULL DEFAULT 1,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (device_id, app, key)
		)`,
		// The device asks for one app's secrets, and compares the app's highest
		// generation to decide whether to re-fetch.
		`CREATE INDEX IF NOT EXISTS idx_app_secrets_device_app ON device_app_secrets (device_id, app)`,
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

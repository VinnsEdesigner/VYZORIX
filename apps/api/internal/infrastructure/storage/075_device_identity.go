package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddDeviceIdentity adds the columns the keypair registration model needs.
//
// . The device generates its own signing keypair
// and sends only the public half, so nothing secret is ever transmitted — which is
// what removes the FCM delivery of the old command secret. `pairing_code` and
// `pairing_expires_at` carry the short code a user types to claim the device.
func migrateAddDeviceIdentity(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE devices ADD COLUMN public_key TEXT`,
		`ALTER TABLE devices ADD COLUMN pairing_code TEXT`,
		`ALTER TABLE devices ADD COLUMN pairing_expires_at INTEGER`,
		// A claim looks the device up by code, so the column needs an index. It is
		// not unique: a code is cleared once claimed, and SQLite treats multiple
		// NULLs as distinct, so uniqueness would not hold anyway.
		`CREATE INDEX IF NOT EXISTS idx_devices_pairing_code ON devices(pairing_code)`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			// SQLite reports a duplicate column for a re-run, which is not an error.
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return err
		}
	}

	return nil
}

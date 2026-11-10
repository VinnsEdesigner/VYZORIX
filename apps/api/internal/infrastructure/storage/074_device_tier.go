package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddDeviceTier adds the device tier column, so a device's state
// capability level is read from its record rather than asserted by a caller.
// The tier names device state
// (unrooted / rooted / advanced) and resolution intersects on it, so an unrooted
// device can never be handed a rooted-only API.
//
// The default is 1 (unrooted), matching device.TierUnrooted. A row written before
// this migration is treated as the least privileged, never as advanced.
func migrateAddDeviceTier(tx *sql.Tx) error {
	_, err := tx.ExecContext(
		context.Background(),
		`ALTER TABLE devices ADD COLUMN tier INTEGER NOT NULL DEFAULT 1`,
	)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}

	return nil
}

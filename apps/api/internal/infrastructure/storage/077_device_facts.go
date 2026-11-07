package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddDeviceFacts adds the columns the enriched enroll payload carries, so a
// device's hardware and build are recorded at registration rather than inferred.
//
// The device already collected apiLevel, ABIs, RAM and storage at enroll, but the
// request dropped them and the server had nowhere to put them; security_patch and
// build_id existed as columns but were never written. This reconciles the record
// with what the device reports, so the operator sees the whole picture — model,
// build, patch level, capacity — when they claim it.
//
// Every column is nullable/zero-defaulted: a device that omits a fact (an older
// build, or a platform that does not expose one) must still enroll, and a row
// written before this migration reads as zero rather than failing.
func migrateAddDeviceFacts(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE devices ADD COLUMN build_id TEXT`,
		`ALTER TABLE devices ADD COLUMN api_level INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE devices ADD COLUMN supported_abis TEXT`,
		`ALTER TABLE devices ADD COLUMN ram_bytes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE devices ADD COLUMN storage_bytes INTEGER NOT NULL DEFAULT 0`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			// SQLite reports a duplicate column for a re-run, which is not an error.
			// security_patch and build_id may already exist from migration 027.
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}

			return err
		}
	}

	return nil
}

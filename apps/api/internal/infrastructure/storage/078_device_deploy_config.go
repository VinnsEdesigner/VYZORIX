package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateCreateDeviceDeployConfig creates the per-device signed config store.
//
// One row per device and kind: `current` is what the device should run, and
// `last_good` is what last applied. Retention is two, enforced by the repository
// rather than by a constraint, because the demotion is a read-modify-write.
//
// The document is the signed vyzorix.v text, stored verbatim so a re-check sees
// the same bytes that were signed. It is not parsed into columns: the signature
// covers the text, and decomposing it would mean re-rendering it on read, which
// is the one thing that must not happen to a signed artefact.
func migrateCreateDeviceDeployConfig(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS device_deploy_configs (
			device_id  TEXT NOT NULL,
			kind       TEXT NOT NULL,
			version    INTEGER NOT NULL,
			document   TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			applied_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (device_id, kind)
		)`,
		// The device reports an applied version; the console reads held-vs-target,
		// so the lookup is by device.
		`CREATE INDEX IF NOT EXISTS idx_deploy_configs_device ON device_deploy_configs (device_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			// SQLite reports "already exists" on a re-run, which is not an error.
			if strings.Contains(err.Error(), "already exists") {
				continue
			}

			return err
		}
	}

	return nil
}

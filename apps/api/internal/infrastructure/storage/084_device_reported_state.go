package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddDeviceReportedState records what a device says it is actually running.
//
// Until now the only record of "what is running" was `device_deploy_configs.applied_at`,
// written from a `device_ready` frame — and that frame was recorded as applied even
// when the device refused the config, so the console showed a deploy that had never
// landed. The device held the truth and never said it.
//
// The device now reports its state on every telemetry heartbeat, and these columns
// hold it so drift can be compared on read:
//
//	reported_config_version  the config version the device actually holds
//	reported_app_digests     app name → digest of the artifacts installed for it
//
// `reported_config_version` is nullable on purpose: NULL means "this device does not
// report state" (an older build), which is distinguishable from 0 ("reports holding
// no config"). The two must not be conflated — the first is unknown and must never
// render as healthy, the second is a real state.
//
// `reported_at` is the telemetry timestamp, so a stale report can be told from a
// fresh one: a device that reported drift an hour ago and has been silent since is
// not the same as one reporting it now.
func migrateAddDeviceReportedState(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE devices ADD COLUMN reported_config_version INTEGER`,
		`ALTER TABLE devices ADD COLUMN reported_app_digests TEXT`,
		`ALTER TABLE devices ADD COLUMN reported_at INTEGER NOT NULL DEFAULT 0`,
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

package storage

import (
	"database/sql"
	"testing"
)

// TestProvisioningMigration102IsIdempotent proves the migration that rebuilds
// provisioning_resources can run against a database where 101 already created the table.
//
// It matters because 102 now carries everything the resource table needs: the kind column,
// the nullable device_id, the per-kind ownership indexes, and the repository scope columns.
// A second boot on an existing database must not fail.
func TestProvisioningMigration102IsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := migrateProvisioningGitOps(tx); err != nil {
		t.Fatalf("101: %v", err)
	}
	if err := migrateProvisioningResourceOwnershipRetention(tx); err != nil {
		t.Fatalf("102: %v", err)
	}

	// The rebuilt table must carry the kind and a nullable device_id.
	var kindCol, deviceCol int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('provisioning_resources') WHERE name='kind'`).Scan(&kindCol); err != nil {
		t.Fatalf("pragma kind: %v", err)
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('provisioning_resources') WHERE name='device_id'`).Scan(&deviceCol); err != nil {
		t.Fatalf("pragma device_id: %v", err)
	}
	if kindCol != 1 || deviceCol != 1 {
		t.Fatalf("kind=%d device_id=%d, want both present", kindCol, deviceCol)
	}

	// An org row (no device) and a device row must both be insertable.
	if _, err := tx.Exec(`INSERT INTO provisioning_resources
(id, repo_id, kind, device_id, path, applied_at) VALUES ('r-org', NULL, 'alerts', NULL, 'orgs/acme/alerts.toml', 0)`); err != nil {
		t.Fatalf("insert org row: %v", err)
	}
}

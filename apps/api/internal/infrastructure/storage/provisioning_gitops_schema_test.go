package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// provisioningTestDB opens an in-memory database with the real migration
// registry and the foreign-key pragma the server uses, so the constraints under
// test are the ones production gets.
func provisioningTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := runMigrations(db); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	return db
}

// seedProvisioningOrg inserts the org and user rows the foreign keys require.
func seedProvisioningOrg(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	stmts := []string{
		`INSERT INTO users (id, email, name, created_at, updated_at) VALUES ('u1','a@b.c','A',0,0)`,
		`INSERT INTO organizations (id, name, created_by, created_at, updated_at) VALUES ('org-1','Org','u1',0,0)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func insertProvisioningConnection(t *testing.T, db *sql.DB, id string) {
	t.Helper()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO provisioning_connections (id, org_id, name, type, created_at, updated_at)
		 VALUES (?, 'org-1', ?, 'github', 0, 0)`, id, id)
	if err != nil {
		t.Fatalf("insert connection %s: %v", id, err)
	}
}

func insertProvisioningRepo(t *testing.T, db *sql.DB, id, connectionID string) {
	t.Helper()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO provisioning_repositories
		 (id, org_id, connection_id, name, type, url, branch, created_at, updated_at)
		 VALUES (?, 'org-1', ?, ?, 'github', 'https://example.test/r.git', 'main', 0, 0)`,
		id, connectionID, id)
	if err != nil {
		t.Fatalf("insert repo %s: %v", id, err)
	}
}

// TestProvisioningResourceOneOwner is the guard for the design's central rule:
// at most one repository may own a device, and a released device may be adopted
// by another repository.
//
// A plain UNIQUE(device_id) would satisfy the first half and make the second
// impossible, because the released row still exists. The partial index is what
// lets both hold at once.
func TestProvisioningResourceOneOwner(t *testing.T) {
	db := provisioningTestDB(t)
	seedProvisioningOrg(t, db)
	insertProvisioningConnection(t, db, "conn-1")
	insertProvisioningRepo(t, db, "repo-a", "conn-1")
	insertProvisioningRepo(t, db, "repo-b", "conn-1")

	ctx := context.Background()
	insertResource := func(id, repoID, deviceID, path string, releasedAt interface{}) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO provisioning_resources (id, repo_id, device_id, path, applied_at, released_at)
			 VALUES (?, ?, ?, ?, 0, ?)`, id, repoID, deviceID, path, releasedAt)
		return err
	}

	if err := insertResource("r1", "repo-a", "device-1", "devices/device-1/vyzorix.v", nil); err != nil {
		t.Fatalf("first owner must be allowed: %v", err)
	}

	if err := insertResource("r2", "repo-b", "device-1", "other/device-1/vyzorix.v", nil); err == nil {
		t.Fatal("a second repository must not own the same device")
	}

	// Release repo-a's ownership, then repo-b may adopt the device.
	if _, err := db.ExecContext(ctx,
		`UPDATE provisioning_resources SET released_at = 123 WHERE id = 'r1'`); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := insertResource("r3", "repo-b", "device-1", "other/device-1/vyzorix.v", nil); err != nil {
		t.Fatalf("a released device must be adoptable: %v", err)
	}

	// Same repo, same path, still refused.
	if err := insertResource("r4", "repo-b", "device-2", "other/device-1/vyzorix.v", nil); err == nil {
		t.Fatal("a repository must not hold two resources at the same path")
	}

	// A distinct device in the same repo is fine.
	if err := insertResource("r5", "repo-b", "device-2", "other/device-2/vyzorix.v", nil); err != nil {
		t.Fatalf("a distinct device must be allowed: %v", err)
	}
}

// TestProvisioningConnectionInUseIsRestricted pins that the foreign key, not
// only an application check, refuses to delete a connection a repository uses.
func TestProvisioningConnectionInUseIsRestricted(t *testing.T) {
	db := provisioningTestDB(t)
	seedProvisioningOrg(t, db)
	insertProvisioningConnection(t, db, "conn-1")
	insertProvisioningRepo(t, db, "repo-a", "conn-1")

	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`DELETE FROM provisioning_connections WHERE id = 'conn-1'`); err == nil {
		t.Fatal("a connection referenced by a repository must not be deletable")
	}

	// Once the repository is gone the connection is free.
	if _, err := db.ExecContext(ctx, `DELETE FROM provisioning_repositories WHERE id = 'repo-a'`); err != nil {
		t.Fatalf("delete repo: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM provisioning_connections WHERE id = 'conn-1'`); err != nil {
		t.Fatalf("an unreferenced connection must be deletable: %v", err)
	}
}

// TestProvisioningRepositoryCascade pins that a deleted repository takes its jobs
// and ownership rows with it when the caller did not ask to keep them. The delete is
// explicit rather than a cascade, because the FK is RESTRICT so the schema cannot
// silently take a released row the caller meant to keep.
func TestProvisioningRepositoryCascade(t *testing.T) {
	db := provisioningTestDB(t)
	seedProvisioningOrg(t, db)
	insertProvisioningConnection(t, db, "conn-1")
	insertProvisioningRepo(t, db, "repo-a", "conn-1")

	ctx := context.Background()
	stmts := []string{
		`INSERT INTO provisioning_resources (id, repo_id, device_id, path, applied_at)
		 VALUES ('r1','repo-a','device-1','devices/device-1/vyzorix.v',0)`,
		`INSERT INTO provisioning_jobs (id, repo_id, state, started_at) VALUES ('j1','repo-a','success',0)`,
		`INSERT INTO provisioning_job_results (id, job_id, path, outcome) VALUES ('jr1','j1','devices/device-1/vyzorix.v','applied')`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	store := NewProvisioningStorage(db)

	// The default delete removes the ownership rows explicitly, then the repository.
	if err := store.DeleteRepository(ctx, "org-1", "repo-a", false); err != nil {
		t.Fatalf("delete repo: %v", err)
	}

	for _, table := range []string{"provisioning_resources", "provisioning_jobs", "provisioning_job_results"} {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s still holds %d rows after the repository was deleted", table, n)
		}
	}
}

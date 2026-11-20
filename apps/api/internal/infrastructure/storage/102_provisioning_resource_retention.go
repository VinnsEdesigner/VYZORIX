package storage

import (
	"context"
	"database/sql"
)

// migrateProvisioningResourceOwnershipRetention rebuilds provisioning_resources so a
// released device keeps its history after the repository is deleted, and adds the
// job duration the health-over-time view aggregates.
//
// Migration 101 declared repo_id NOT NULL with ON DELETE CASCADE. That contradicted
// the contract's keep_resources=true mode: releasing a row (released_at set) and then
// deleting the parent cascaded the row away, so the history the release exists to
// preserve was lost.
//
// repo_id becomes nullable, and the FK is RESTRICT. A released row has its repo_id
// cleared, which both severs the link to a repository that no longer exists and keeps
// the row out of the FK's way. CASCADE is refused rather than used because it cannot
// distinguish the rows a delete means to take from the ones it means to keep.
//
// duration_ms is stored on a job rather than derived on every read: a trend query
// aggregates many rows, and recomputing finished_at - started_at per row on each
// request is work the job can do once when it finishes.
func migrateProvisioningResourceOwnershipRetention(tx *sql.Tx) error {
	ctx := context.Background()

	// SQLite cannot alter a column or a foreign key, so the table is rebuilt: rows are
	// copied, the old table dropped, and the new one renamed into place.
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS provisioning_resources_new (
			id TEXT PRIMARY KEY,
			repo_id TEXT,
			kind TEXT NOT NULL DEFAULT 'device',
			device_id TEXT,
			path TEXT NOT NULL,
			config_version INTEGER NOT NULL DEFAULT 0,
			content_hash TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'applied',
			last_error TEXT NOT NULL DEFAULT '',
			last_applied_at INTEGER,
			applied_at INTEGER NOT NULL,
			released_at INTEGER,
			FOREIGN KEY (repo_id) REFERENCES provisioning_repositories(id) ON DELETE RESTRICT
		)`,
		// Every pre-existing row is a device manifest, so the copy stamps the kind.
		`INSERT INTO provisioning_resources_new
			(id, repo_id, kind, device_id, path, config_version, content_hash, state,
			 last_error, last_applied_at, applied_at, released_at)
		 SELECT id, repo_id, 'device', device_id, path, config_version, content_hash, state,
			last_error, last_applied_at, applied_at, released_at
		 FROM provisioning_resources`,
		`DROP TABLE provisioning_resources`,
		`ALTER TABLE provisioning_resources_new RENAME TO provisioning_resources`,
		// One owner per device, for device resources only: an org resource has no device, so
		// it must not participate or every org row would collide on NULL.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_provisioning_resources_device_owner
			ON provisioning_resources(device_id)
			WHERE released_at IS NULL AND kind = 'device'`,
		// One owner per org object path: two repositories cannot both declare
		// orgs/acme/alerts.toml.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_provisioning_resources_kind_path
			ON provisioning_resources(kind, path)
			WHERE released_at IS NULL AND kind != 'device'`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_resources_repo
			ON provisioning_resources(repo_id)`,
		// (repo_id, path) is unique only among owned rows: a released row keeps its path
		// but no longer reserves it, so a re-adopting repository can reuse the path.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_provisioning_resources_repo_path
                        ON provisioning_resources(repo_id, path) WHERE released_at IS NULL`,
		// The job's duration, written once when the run finishes so a trend query
		// aggregates instead of subtracting timestamps per row on every read.
		`ALTER TABLE provisioning_jobs ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0`,
		`UPDATE provisioning_jobs
                 SET duration_ms = finished_at - started_at
                 WHERE finished_at IS NOT NULL AND finished_at > started_at AND duration_ms = 0`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_repo_state
                 ON provisioning_jobs(repo_id, state, started_at DESC)`,
		// Who caused each provisioning action. A dedicated table rather than
		// audit_logs because the question is different: audit_logs answers "what did
		// this user do", this answers "who caused this change to this repository, and
		// was it a person or a machine". It also carries org_id, which audit_logs does
		// not, and it distinguishes a scheduled sync from a manual one.
		`CREATE TABLE IF NOT EXISTS provisioning_events (
                        id TEXT PRIMARY KEY,
                        org_id TEXT NOT NULL,
                        actor_kind TEXT NOT NULL,
                        actor_id TEXT NOT NULL,
                        actor_label TEXT NOT NULL DEFAULT '',
                        type TEXT NOT NULL,
                        resource_type TEXT NOT NULL,
                        resource_id TEXT NOT NULL DEFAULT '',
                        result TEXT NOT NULL,
                        detail TEXT NOT NULL DEFAULT '',
                        created_at INTEGER NOT NULL
                )`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_events_org
                        ON provisioning_events(org_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_events_resource
                        ON provisioning_events(org_id, resource_type, resource_id, created_at DESC)`,
		// The per-repository webhook secret. Encrypted at rest with the same cipher as a
		// connection token, and write-only: no route returns it.
		`ALTER TABLE provisioning_repositories
                        ADD COLUMN webhook_secret_ciphertext TEXT NOT NULL DEFAULT ''`,
		// The repository's include/exclude scope, stored as JSON arrays. They are a small,
		// ordered, opaque set read whole, so a join table would add a write per pattern for
		// no query the code needs.
		`ALTER TABLE provisioning_repositories
			ADD COLUMN include_patterns TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE provisioning_repositories
			ADD COLUMN exclude_patterns TEXT NOT NULL DEFAULT '[]'`,
		// Whether the connection's token has push scope. Pulling needs read only, so this
		// defaults off: a push-back attempt on a read-only credential is refused here
		// rather than failing at the provider with an opaque 403.
		`ALTER TABLE provisioning_connections
                        ADD COLUMN write_enabled INTEGER NOT NULL DEFAULT 0`,
		// The webhook receiver resolves a repository from the URL's path, then verifies
		// the signature against that repository's secret. A lookup by id is the hot path.
		`CREATE INDEX IF NOT EXISTS idx_provisioning_repositories_id
                        ON provisioning_repositories(id)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

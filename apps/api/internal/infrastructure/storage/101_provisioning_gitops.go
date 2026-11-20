package storage

import (
	"context"
	"database/sql"
)

// migrateProvisioningGitOps creates the GitOps provisioning tables: a stored git
// credential, the repositories pulled on an interval, the manifests each owns,
// the sync job history, and the per-resource outcome of each job.
//
// Two constraints carry the design:
//
//   - The partial unique index on provisioning_resources(device_id) allows at
//     most one *owning* repository per device. It is partial (WHERE released_at
//     IS NULL) so a released device can be adopted by another repository; a plain
//     UNIQUE(device_id) would make adoption permanently impossible, because the
//     released row still exists.
//   - FOREIGN KEY ... ON DELETE RESTRICT turns "a connection in use cannot be
//     deleted" into a schema guarantee rather than one code path that must
//     remember to check.
func migrateProvisioningGitOps(tx *sql.Tx) error {
	ctx := context.Background()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS provisioning_connections (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			host TEXT NOT NULL DEFAULT '',
			token_ciphertext TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			last_error TEXT NOT NULL DEFAULT '',
			checked_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			UNIQUE(org_id, name),
			FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_connections_org
			ON provisioning_connections(org_id)`,

		`CREATE TABLE IF NOT EXISTS provisioning_repositories (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			connection_id TEXT NOT NULL,
			name TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			branch TEXT NOT NULL,
			path TEXT NOT NULL DEFAULT '/',
			sync_target TEXT NOT NULL DEFAULT 'device',
			sync_enabled INTEGER NOT NULL DEFAULT 1,
			sync_interval_seconds INTEGER NOT NULL DEFAULT 300,
			sync_state TEXT NOT NULL DEFAULT 'pending',
			last_error TEXT NOT NULL DEFAULT '',
			last_sync_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			UNIQUE(org_id, name),
			FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
			FOREIGN KEY (connection_id) REFERENCES provisioning_connections(id) ON DELETE RESTRICT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_repositories_org
			ON provisioning_repositories(org_id)`,

		`CREATE TABLE IF NOT EXISTS provisioning_resources (
			id TEXT PRIMARY KEY,
			repo_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			path TEXT NOT NULL,
			config_version INTEGER NOT NULL DEFAULT 0,
			content_hash TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'applied',
			last_error TEXT NOT NULL DEFAULT '',
			last_applied_at INTEGER,
			applied_at INTEGER NOT NULL,
			released_at INTEGER,
			UNIQUE(repo_id, path),
			FOREIGN KEY (repo_id) REFERENCES provisioning_repositories(id) ON DELETE CASCADE
		)`,
		// At most one owning repository per device. Partial, so releasing a device
		// frees it for adoption without deleting its history.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_provisioning_resources_device_owner
			ON provisioning_resources(device_id) WHERE released_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_resources_repo
			ON provisioning_resources(repo_id)`,

		`CREATE TABLE IF NOT EXISTS provisioning_jobs (
			id TEXT PRIMARY KEY,
			repo_id TEXT NOT NULL,
			state TEXT NOT NULL,
			dry_run INTEGER NOT NULL DEFAULT 0,
			applied INTEGER NOT NULL DEFAULT 0,
			skipped INTEGER NOT NULL DEFAULT 0,
			failed INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL,
			finished_at INTEGER,
			FOREIGN KEY (repo_id) REFERENCES provisioning_repositories(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_repo
			ON provisioning_jobs(repo_id, started_at DESC)`,

		// One row per manifest per job: the difference between "3 failed" and
		// "these 3 failed, and here is why".
		`CREATE TABLE IF NOT EXISTS provisioning_job_results (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL,
			outcome TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (job_id) REFERENCES provisioning_jobs(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_provisioning_job_results_job
			ON provisioning_job_results(job_id)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

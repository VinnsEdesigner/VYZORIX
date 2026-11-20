package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// RepositoryStore is the repository half of the provisioning store.
type RepositoryStore interface {
	SaveRepository(ctx context.Context, r *provisioning.Repository) error
	GetRepository(ctx context.Context, orgID, id string) (*provisioning.Repository, error)
	ListRepositories(ctx context.Context, orgID string) ([]*provisioning.Repository, error)
	DeleteRepository(ctx context.Context, orgID, id string, keepResources bool) error
	UpdateSyncStatus(ctx context.Context, repoID string, state provisioning.SyncState, lastError string, lastSyncAt time.Time) error
	ListDueForSync(ctx context.Context, now time.Time) ([]*provisioning.Repository, error)
}

var _ RepositoryStore = (*ProvisioningStorage)(nil)

const provisioningRepositoryColumns = `id, org_id, connection_id, name, title, description,
	type, url, branch, path, sync_target, sync_enabled, sync_interval_seconds,
	sync_state, last_error, last_sync_at, created_at, updated_at,
	include_patterns, exclude_patterns`

// encodePatterns stores a pattern list as a JSON array, never NULL.
func encodePatterns(patterns []string) string {
	if len(patterns) == 0 {
		return "[]"
	}

	b, err := json.Marshal(patterns)
	if err != nil {
		return "[]"
	}

	return string(b)
}

// decodePatterns reads a stored pattern list. A malformed value reads as empty, which is the
// permissive default rather than a filter that silently matches nothing.
func decodePatterns(stored string) []string {
	if stored == "" {
		return nil
	}

	var patterns []string
	if err := json.Unmarshal([]byte(stored), &patterns); err != nil {
		return nil
	}

	return patterns
}

// SaveRepository upserts a repository.
func (r *ProvisioningStorage) SaveRepository(ctx context.Context, repo *provisioning.Repository) error {
	enabled := 0
	if repo.Enabled {
		enabled = 1
	}

	query := `
		INSERT INTO provisioning_repositories
			(id, org_id, connection_id, name, title, description, type, url, branch,
			 path, sync_target, sync_enabled, sync_interval_seconds, sync_state,
			 last_error, last_sync_at, created_at, updated_at, include_patterns, exclude_patterns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			connection_id = excluded.connection_id,
			name = excluded.name,
			title = excluded.title,
			description = excluded.description,
			type = excluded.type,
			url = excluded.url,
			branch = excluded.branch,
			path = excluded.path,
			sync_target = excluded.sync_target,
			sync_enabled = excluded.sync_enabled,
			sync_interval_seconds = excluded.sync_interval_seconds,
			sync_state = excluded.sync_state,
			last_error = excluded.last_error,
			last_sync_at = excluded.last_sync_at,
			updated_at = excluded.updated_at,
			include_patterns = excluded.include_patterns,
			exclude_patterns = excluded.exclude_patterns
	`
	_, err := r.db.ExecContext(ctx, query,
		repo.ID, repo.OrgID, repo.ConnectionID, repo.Name, repo.Title, repo.Description,
		string(repo.Type), repo.URL, repo.Branch, repo.Path, string(repo.SyncTarget),
		enabled, repo.IntervalSeconds, string(repo.SyncState), repo.LastError,
		nullTimeMillis(repo.LastSyncAt), repo.CreatedAt.UnixMilli(), repo.UpdatedAt.UnixMilli(),
		encodePatterns(repo.Include), encodePatterns(repo.Exclude),
	)

	return err
}

// GetRepository returns one repository.
func (r *ProvisioningStorage) GetRepository(ctx context.Context, orgID, id string) (*provisioning.Repository, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+provisioningRepositoryColumns+`
		 FROM provisioning_repositories WHERE org_id = ? AND id = ?`, orgID, id)

	return scanProvisioningRepository(row)
}

// ListRepositories returns an organisation's repositories.
func (r *ProvisioningStorage) ListRepositories(ctx context.Context, orgID string) ([]*provisioning.Repository, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+provisioningRepositoryColumns+`
		 FROM provisioning_repositories WHERE org_id = ? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.Repository
	for rows.Next() {
		repo, err := scanProvisioningRepository(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, repo)
	}

	return out, rows.Err()
}

// DeleteRepository removes a repository.
//
// keepResources decides the fate of the devices it owned. With true the ownership rows
// are RELEASED (released_at set, repo_id cleared), which frees each device for adoption
// while keeping the history of who applied what; the device keeps running the config it
// has. With false the rows are deleted outright, which also frees the devices but forgets
// the history. The name matches the query parameter, so "keep" must mean the release path
// — getting it backwards would silently erase the history a release exists to preserve.
//
// Jobs and job results always go: a sync history is about a repository that no
// longer exists.
func (r *ProvisioningStorage) DeleteRepository(ctx context.Context, orgID, id string, keepResources bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM provisioning_repositories WHERE org_id = ? AND id = ?`,
		orgID, id).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return provisioning.ErrRepositoryNotFound
	}

	if keepResources {
		if _, err := tx.ExecContext(ctx,
			`UPDATE provisioning_resources SET released_at = ?, repo_id = NULL
                         WHERE repo_id = ? AND released_at IS NULL`,
			time.Now().UnixMilli(), id); err != nil {
			return err
		}
	} else {
		// The FK is RESTRICT, so the rows the caller wants gone are removed explicitly
		// rather than by a cascade that would also take the retained ones.
		if _, err := tx.ExecContext(ctx, `DELETE FROM provisioning_resources WHERE repo_id = ?`, id); err != nil {
			return err
		}
	}

	// Jobs are deleted explicitly so their results cascade, and the repository row
	// goes last: the resources FK is what the schema would cascade, so it is handled
	// above before the parent is removed.
	if _, err := tx.ExecContext(ctx, `DELETE FROM provisioning_jobs WHERE repo_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM provisioning_repositories WHERE org_id = ? AND id = ?`,
		orgID, id); err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateSyncStatus writes only the columns a sync owns.
//
// It exists so a sync finishing cannot clobber a concurrent PATCH: a full-row
// upsert would overwrite branch/path/interval with values read before the user's
// edit landed, silently reverting it.
func (r *ProvisioningStorage) UpdateSyncStatus(ctx context.Context, repoID string, state provisioning.SyncState, lastError string, lastSyncAt time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_repositories
                 SET sync_state = ?, last_error = ?, last_sync_at = ?, updated_at = ?
                 WHERE id = ?`,
		string(state), lastError, lastSyncAt.UnixMilli(), time.Now().UnixMilli(), repoID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return provisioning.ErrRepositoryNotFound
	}

	return nil
}

// UpdateRepositoriesTypeForConnection re-stamps the provider type on every repository
// that uses a connection.
//
// A repository's type is a denormalised copy of its connection's, and the fetcher and
// pull requester use it to pick the right provider API and git username. Without this,
// changing a connection from github to gitlab left its repositories on the old type, so
// they authenticated with the wrong username and asked the wrong provider's API.
func (r *ProvisioningStorage) UpdateRepositoriesTypeForConnection(ctx context.Context, connectionID, typ string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_repositories SET type = ?, updated_at = ? WHERE connection_id = ?`,
		typ, time.Now().UnixMilli(), connectionID)

	return err
}

// Counts reports how many resources and connections an organisation holds. It backs
// the instance summary, which must report what the server can prove rather than a
// cached figure that could drift from the tables.
func (r *ProvisioningStorage) Counts(ctx context.Context, orgID string) (resources, connections int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM provisioning_resources res
                 JOIN provisioning_repositories repo ON repo.id = res.repo_id
                 WHERE repo.org_id = ? AND res.released_at IS NULL`, orgID).Scan(&resources)
	if err != nil {
		return 0, 0, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM provisioning_connections WHERE org_id = ?`, orgID).Scan(&connections)
	if err != nil {
		return 0, 0, err
	}

	return resources, connections, nil
}

// GetRepositoryForWebhook resolves a repository from its id alone.
//
// A webhook URL carries only the repository id, so the receiver cannot require an
// organisation header it will never receive. The repository's own org_id is returned in
// the row, which is what the sync then scopes to.
func (r *ProvisioningStorage) GetRepositoryForWebhook(ctx context.Context, id string) (*provisioning.Repository, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+provisioningRepositoryColumns+`
                 FROM provisioning_repositories WHERE id = ?`, id)

	return scanProvisioningRepository(row)
}

// RepositoryWebhookSecret returns the decrypted webhook secret, or an empty Secret when
// none is configured. It is only reachable from the receiver, never from a route.
func (r *ProvisioningStorage) RepositoryWebhookSecret(ctx context.Context, repoID string) (redaction.Secret, error) {
	var stored string

	err := r.db.QueryRowContext(ctx,
		`SELECT webhook_secret_ciphertext FROM provisioning_repositories WHERE id = ?`,
		repoID).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return "", provisioning.ErrRepositoryNotFound
	}
	if err != nil {
		return "", err
	}

	return redaction.NewSecret(r.decrypt(stored)), nil
}

// SetRepositoryWebhookSecret stores the secret, encrypted. An empty Secret clears it.
func (r *ProvisioningStorage) SetRepositoryWebhookSecret(ctx context.Context, repoID string, secret redaction.Secret) error {
	ciphertext, err := r.encrypt(secret.Reveal())
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_repositories SET webhook_secret_ciphertext = ?, updated_at = ?
                 WHERE id = ?`, ciphertext, time.Now().UnixMilli(), repoID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return provisioning.ErrRepositoryNotFound
	}

	return nil
}

// ListDueForSync returns enabled repositories whose interval has elapsed. A
// repository that has never synced is due immediately.
func (r *ProvisioningStorage) ListDueForSync(ctx context.Context, now time.Time) ([]*provisioning.Repository, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+provisioningRepositoryColumns+`
		 FROM provisioning_repositories
		 WHERE sync_enabled = 1
		   AND (last_sync_at IS NULL
		        OR last_sync_at + (sync_interval_seconds * 1000) <= ?)
		 ORDER BY last_sync_at IS NULL DESC, last_sync_at ASC`, now.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.Repository
	for rows.Next() {
		repo, err := scanProvisioningRepository(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, repo)
	}

	return out, rows.Err()
}

func scanProvisioningRepository(scanner interface{ Scan(...any) error }) (*provisioning.Repository, error) {
	var repo provisioning.Repository
	var typ, syncTarget, syncState, include, exclude string
	var enabled int
	var lastSyncAt sql.NullInt64
	var createdAt, updatedAt int64

	err := scanner.Scan(&repo.ID, &repo.OrgID, &repo.ConnectionID, &repo.Name, &repo.Title,
		&repo.Description, &typ, &repo.URL, &repo.Branch, &repo.Path, &syncTarget,
		&enabled, &repo.IntervalSeconds, &syncState, &repo.LastError, &lastSyncAt,
		&createdAt, &updatedAt, &include, &exclude)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provisioning.ErrRepositoryNotFound
	}
	if err != nil {
		return nil, err
	}

	repo.Type = provisioning.ConnectionType(typ)
	repo.SyncTarget = provisioning.SyncTarget(syncTarget)
	repo.SyncState = provisioning.SyncState(syncState)
	repo.Enabled = enabled == 1
	repo.Include = decodePatterns(include)
	repo.Exclude = decodePatterns(exclude)
	repo.CreatedAt = time.UnixMilli(createdAt)
	repo.UpdatedAt = time.UnixMilli(updatedAt)
	if lastSyncAt.Valid {
		t := time.UnixMilli(lastSyncAt.Int64)
		repo.LastSyncAt = &t
	}

	return &repo, nil
}

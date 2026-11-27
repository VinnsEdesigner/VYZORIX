package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// ResourceStore is the owned-manifest half of the provisioning store.
type ResourceStore interface {
	Upsert(ctx context.Context, r *provisioning.Resource) error
	GetByDevice(ctx context.Context, deviceID string) (*provisioning.Resource, error)
	ListByRepo(ctx context.Context, repoID string) ([]*provisioning.Resource, error)
	SetState(ctx context.Context, id string, state provisioning.ResourceState, lastError string) error
}

var _ ResourceStore = (*ProvisioningStorage)(nil)

const provisioningResourceColumns = `id, repo_id, kind, device_id, path, config_version,
        content_hash, state, last_error, last_applied_at, applied_at, released_at`

// Upsert records an applied manifest.
//
// Ownership is enforced by the partial unique index on (device_id) WHERE
// released_at IS NULL, not by a read-then-write here: two concurrent syncs could
// both read "no owner" and both write, and only the index stops the second. The
// index failure is translated to ErrDeviceOwned so the caller reports a conflict
// rather than a raw constraint error.
//
// The conflict target is the row id, not (repo_id, path). A manifest can be renamed
// within its repository (vyzorix.toml -> vyzorix.v); the service carries the existing
// row's id across that rename and the update moves the path with it. Conflicting on
// (repo_id, path) instead tried to insert a second row for the same device and tripped
// the one-owner index, reporting a rename as "owned by another repository".
func (r *ProvisioningStorage) Upsert(ctx context.Context, res *provisioning.Resource) error {
	query := `
		INSERT INTO provisioning_resources
		(id, repo_id, kind, device_id, path, config_version, content_hash, state,
		last_error, last_applied_at, applied_at, released_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT(id) DO UPDATE SET
		repo_id = excluded.repo_id,
		kind = excluded.kind,
		device_id = excluded.device_id,
		path = excluded.path,
		config_version = excluded.config_version,
		content_hash = excluded.content_hash,
		state = excluded.state,
		last_error = excluded.last_error,
		last_applied_at = excluded.last_applied_at,
		applied_at = excluded.applied_at
		`
	_, err := r.db.ExecContext(ctx, query,
		res.ID, res.RepoID, string(res.Kind), nullString(res.DeviceID), res.Path,
		res.ConfigVersion, res.ContentHash, string(res.State), res.LastError,
		nullTimeMillis(res.LastAppliedAt), res.AppliedAt.UnixMilli(),
	)
	if err != nil && isUniqueConstraintError(err) {
		return r.deviceOwned(ctx, res.DeviceID, res.RepoID)
	}

	return err
}

// deviceOwned builds the conflict error, naming the repository that holds the
// device so the message can say who.
func (r *ProvisioningStorage) deviceOwned(ctx context.Context, deviceID, repoID string) error {
	owner, err := r.GetByDevice(ctx, deviceID)
	if err == nil && owner.RepoID != repoID {
		return &provisioning.DeviceOwnedError{DeviceID: deviceID, Owner: owner.RepoID}
	}

	// The owner is this repository. Saying "owned by another repository" would be false
	// and unactionable; the real cause is a second manifest path for a device this repo
	// already tracks.
	if err == nil && owner.RepoID == repoID && owner.Path != "" {
		return fmt.Errorf("%w: device %s is already tracked at %s in this repository",
			provisioning.ErrDeviceOwned, deviceID, owner.Path)
	}

	return &provisioning.DeviceOwnedError{DeviceID: deviceID}
}

// GetByDevice returns the owning resource for a device, ignoring released rows.
func (r *ProvisioningStorage) GetByDevice(ctx context.Context, deviceID string) (*provisioning.Resource, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+provisioningResourceColumns+`
                 FROM provisioning_resources
                 WHERE device_id = ? AND released_at IS NULL`, deviceID)

	return scanProvisioningResource(row)
}

// GetByID returns one resource, scoped to its repository so a resource id from
// another repository cannot be read through the wrong route.
func (r *ProvisioningStorage) GetByID(ctx context.Context, repoID, id string) (*provisioning.Resource, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+provisioningResourceColumns+`
                 FROM provisioning_resources WHERE repo_id = ? AND id = ?`, repoID, id)

	return scanProvisioningResource(row)
}

// ListByRepo returns every resource a repository owns.
func (r *ProvisioningStorage) ListByRepo(ctx context.Context, repoID string) ([]*provisioning.Resource, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+provisioningResourceColumns+`
                 FROM provisioning_resources
                 WHERE repo_id = ? AND released_at IS NULL ORDER BY path`, repoID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.Resource
	for rows.Next() {
		res, err := scanProvisioningResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}

	return out, rows.Err()
}

// UpdatePath re-points a resource at a new manifest path after a rename.
func (r *ProvisioningStorage) UpdatePath(ctx context.Context, id, path string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_resources SET path = ? WHERE id = ?`, path, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return provisioning.ErrResourceNotFound
	}

	return nil
}

// StatsByRepo returns the per-state counts of a repository's owned resources.
//
// It is one grouped query rather than four counts, and it filters on released_at so
// a device the repository gave up is not counted against it.
func (r *ProvisioningStorage) StatsByRepo(ctx context.Context, repoID string) (*provisioning.ResourceStats, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT state, COUNT(*) FROM provisioning_resources
                 WHERE repo_id = ? AND released_at IS NULL
                 GROUP BY state`, repoID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	stats := &provisioning.ResourceStats{}
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return nil, err
		}

		switch provisioning.ResourceState(state) {
		case provisioning.ResourceApplied:
			stats.Applied = n
		case provisioning.ResourceDrifted:
			stats.Drifted = n
		case provisioning.ResourceMissing:
			stats.Missing = n
		case provisioning.ResourceError:
			stats.Error = n
		case provisioning.ResourceIgnored:
			stats.Ignored = n
		case provisioning.ResourcePending:
			stats.Pending = n
		}
		stats.Total += n
	}

	return stats, rows.Err()
}

// SetState records a resource's derived state.
func (r *ProvisioningStorage) SetState(ctx context.Context, id string, state provisioning.ResourceState, lastError string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_resources SET state = ?, last_error = ? WHERE id = ?`,
		string(state), lastError, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return provisioning.ErrResourceNotFound
	}

	return nil
}

func scanProvisioningResource(scanner interface{ Scan(...any) error }) (*provisioning.Resource, error) {
	var res provisioning.Resource
	var state, kind string
	var repoID, deviceID sql.NullString
	var lastApplied, released sql.NullInt64
	var appliedAt int64

	err := scanner.Scan(&res.ID, &repoID, &kind, &deviceID, &res.Path, &res.ConfigVersion,
		&res.ContentHash, &state, &res.LastError, &lastApplied, &appliedAt, &released)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provisioning.ErrResourceNotFound
	}
	if err != nil {
		return nil, err
	}

	res.RepoID = repoID.String
	// An org resource has no device, so the column reads NULL and the field stays empty.
	res.Kind = provisioning.ResourceKind(kind)
	res.DeviceID = deviceID.String
	res.State = provisioning.ResourceState(state)
	res.AppliedAt = time.UnixMilli(appliedAt)
	if lastApplied.Valid {
		t := time.UnixMilli(lastApplied.Int64)
		res.LastAppliedAt = &t
	}
	if released.Valid {
		t := time.UnixMilli(released.Int64)
		res.ReleasedAt = &t
	}

	return &res, nil
}

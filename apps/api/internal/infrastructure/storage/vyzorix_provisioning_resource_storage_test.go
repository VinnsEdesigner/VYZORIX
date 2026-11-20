package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// newResourceFixture seeds the rows a resource and a job need, and returns the
// store over the real schema.
func newResourceFixture(t *testing.T) (*ProvisioningStorage, context.Context) {
	t.Helper()

	db := provisioningTestDB(t)
	seedProvisioningOrg(t, db)
	insertProvisioningConnection(t, db, "conn-1")
	insertProvisioningRepo(t, db, "repo-1", "conn-1")
	insertProvisioningRepo(t, db, "repo-2", "conn-1")

	return NewProvisioningStorage(db), context.Background()
}

// TestProvisioningResourceUpsertAndState round-trips a resource: an upsert stores
// the hash and version, and a state change is read back.
func TestProvisioningResourceUpsertAndState(t *testing.T) {
	store, ctx := newResourceFixture(t)

	res := &provisioning.Resource{
		ID:            "res-1",
		RepoID:        "repo-1",
		DeviceID:      "device-a",
		Path:          "devices/device-a/vyzorix.toml",
		ContentHash:   "abc123",
		ConfigVersion: 7,
		State:         provisioning.ResourceApplied,
		AppliedAt:     time.Now(),
	}
	if err := store.Upsert(ctx, res); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := store.GetByDevice(ctx, "device-a")
	if err != nil {
		t.Fatalf("GetByDevice: %v", err)
	}
	if got.ContentHash != "abc123" || got.ConfigVersion != 7 || got.State != provisioning.ResourceApplied {
		t.Fatalf("got %+v, want hash/version/state preserved", got)
	}

	// A second upsert is the update path a sync takes when a manifest's hash changes.
	// It exercises ON CONFLICT(repo_id, path) against the PARTIAL unique index migration
	// 102 created; a partial index is only a valid conflict target with its WHERE clause,
	// so this fails loudly if that clause is ever dropped.
	res.ContentHash = "def456"
	res.ConfigVersion = 8
	if err := store.Upsert(ctx, res); err != nil {
		t.Fatalf("second Upsert (update path): %v", err)
	}
	got, err = store.GetByDevice(ctx, "device-a")
	if err != nil {
		t.Fatalf("GetByDevice after update: %v", err)
	}
	if got.ContentHash != "def456" || got.ConfigVersion != 8 {
		t.Fatalf("update did not apply: %+v", got)
	}

	if err := store.SetState(ctx, "res-1", provisioning.ResourceMissing, "gone"); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	got, err = store.GetByDevice(ctx, "device-a")
	if err != nil {
		t.Fatalf("GetByDevice: %v", err)
	}
	if got.State != provisioning.ResourceMissing || got.LastError != "gone" {
		t.Fatalf("state not recorded: %+v", got)
	}
}

// TestProvisioningResourceSecondOwnerIsRefused is the schema rule: a second
// repository cannot claim a device another owns, and the error names the owner.
func TestProvisioningResourceSecondOwnerIsRefused(t *testing.T) {
	store, ctx := newResourceFixture(t)

	first := &provisioning.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}
	if err := store.Upsert(ctx, first); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	second := &provisioning.Resource{
		ID: "res-2", RepoID: "repo-2", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}
	err := store.Upsert(ctx, second)
	if !errors.Is(err, provisioning.ErrDeviceOwned) {
		t.Fatalf("got %v, want ErrDeviceOwned", err)
	}

	var owned *provisioning.DeviceOwnedError
	if !errors.As(err, &owned) || owned.Owner != "repo-1" {
		t.Fatalf("conflict does not name the owner: %v", err)
	}
}

// TestProvisioningDeleteRepositoryReleasesWithoutReverting proves that deleting a
// repository with keep_resources=true releases its devices, which the partial index
// then allows to be adopted, while keep_resources=false removes the rows outright.
func TestProvisioningDeleteRepositoryReleasesWithoutReverting(t *testing.T) {
	store, ctx := newResourceFixture(t)

	first := &provisioning.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}
	if err := store.Upsert(ctx, first); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.DeleteRepository(ctx, "org-1", "repo-1", true); err != nil {
		t.Fatalf("DeleteRepository(keepResources): %v", err)
	}

	if _, err := store.GetByDevice(ctx, "device-a"); !errors.Is(err, provisioning.ErrResourceNotFound) {
		t.Fatalf("a released device still reads as owned: %v", err)
	}

	adopter := &provisioning.Resource{
		ID: "res-2", RepoID: "repo-2", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}
	if err := store.Upsert(ctx, adopter); err != nil {
		t.Fatalf("a released device cannot be adopted: %v", err)
	}
}

// TestProvisioningJobsRecordAndInterrupt round-trips a job and its results, and
// proves a stale working job is failed on the next boot.
func TestProvisioningJobsRecordAndInterrupt(t *testing.T) {
	store, ctx := newResourceFixture(t)

	started := time.Now().Add(-2 * time.Hour)
	job := &provisioning.Job{
		ID: "job-1", RepoID: "repo-1", State: provisioning.SyncWorking, StartedAt: started,
	}
	if err := store.StartJob(ctx, job); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	if err := store.SaveResult(ctx, &provisioning.JobResult{
		ID: "jr-1", JobID: "job-1", Path: "devices/device-a/vyzorix.toml",
		Outcome: provisioning.ResultApplied,
	}); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	results, err := store.ListResults(ctx, "job-1")
	if err != nil || len(results) != 1 || results[0].Outcome != provisioning.ResultApplied {
		t.Fatalf("results = %+v, err = %v", results, err)
	}

	n, err := store.FailInterrupted(ctx, time.Now().Add(-time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("FailInterrupted = %d, err = %v, want 1", n, err)
	}

	got, err := store.GetJob(ctx, "repo-1", "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.State != provisioning.SyncError || got.Error == "" {
		t.Fatalf("interrupted job = %+v, want error with a reason", got)
	}
}

// TestProvisioningDeleteRepositoryKeepResources proves the contract's delete modes:
// keep_resources releases the device (frees it for adoption, keeps the history),
// while the default removes the ownership row.
func TestProvisioningDeleteRepositoryKeepResources(t *testing.T) {
	for _, tc := range []struct {
		name          string
		keepResources bool
		wantReleased  bool
	}{
		{"keep releases", true, true},
		{"default deletes", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, ctx := newResourceFixture(t)

			res := &provisioning.Resource{
				ID: "res-1", RepoID: "repo-1", DeviceID: "device-a",
				Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
			}
			if err := store.Upsert(ctx, res); err != nil {
				t.Fatalf("Upsert: %v", err)
			}

			if err := store.DeleteRepository(ctx, "org-1", "repo-1", tc.keepResources); err != nil {
				t.Fatalf("DeleteRepository: %v", err)
			}

			// Either way the device is adoptable, which is what "release" must guarantee.
			adopter := &provisioning.Resource{
				ID: "res-2", RepoID: "repo-2", DeviceID: "device-a",
				Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
			}
			if err := store.Upsert(ctx, adopter); err != nil {
				t.Fatalf("the device was not released for adoption: %v", err)
			}

			// With keep_resources the old row survives, marked released; without it the
			// row is gone.
			var count int
			if err := store.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM provisioning_resources WHERE id = 'res-1'`).Scan(&count); err != nil {
				t.Fatalf("count: %v", err)
			}

			if tc.wantReleased && count != 1 {
				t.Fatalf("the ownership history was deleted, want it released")
			}
			if !tc.wantReleased && count != 0 {
				t.Fatalf("the ownership row survived, want it deleted")
			}
		})
	}
}

// TestProvisioningUpdateSyncStatusKeepsConfiguration is the lost-update guard: a
// sync stamping its result must not revert a configuration change made meanwhile.
func TestProvisioningUpdateSyncStatusKeepsConfiguration(t *testing.T) {
	store, ctx := newResourceFixture(t)

	repo, err := store.GetRepository(ctx, "org-1", "repo-1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	// The user edits the branch while a sync is running.
	repo.Branch = "release"
	if err := store.SaveRepository(ctx, repo); err != nil {
		t.Fatalf("SaveRepository: %v", err)
	}

	// The sync finishes and stamps only its own columns.
	if err := store.UpdateSyncStatus(ctx, "repo-1", provisioning.SyncSuccess, "", time.Now()); err != nil {
		t.Fatalf("UpdateSyncStatus: %v", err)
	}

	got, err := store.GetRepository(ctx, "org-1", "repo-1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if got.Branch != "release" {
		t.Fatalf("branch = %q, want the user's edit to survive", got.Branch)
	}
	if got.SyncState != provisioning.SyncSuccess {
		t.Fatalf("sync state = %q, want success", got.SyncState)
	}
}

// TestProvisioningCounts proves the instance summary's totals reflect the tables.
func TestProvisioningCounts(t *testing.T) {
	store, ctx := newResourceFixture(t)

	if err := store.Upsert(ctx, &provisioning.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	resources, connections, err := store.Counts(ctx, "org-1")
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if resources != 1 || connections != 1 {
		t.Fatalf("counts = %d resources, %d connections, want 1 and 1", resources, connections)
	}

	// Another org sees none of it.
	resources, connections, err = store.Counts(ctx, "org-other")
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if resources != 0 || connections != 0 {
		t.Fatalf("cross-org leak: %d resources, %d connections", resources, connections)
	}
}

// TestProvisioningStatsByRepo proves the per-state counts the contract's stats[]
// field reports: only this repository's owned rows, one bucket per state, and a
// released device excluded.
func TestProvisioningStatsByRepo(t *testing.T) {
	store, ctx := newResourceFixture(t)
	now := time.Now()

	seed := []*provisioning.Resource{
		{ID: "r-applied", RepoID: "repo-1", DeviceID: "d1", Path: "devices/d1/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: now},
		{ID: "r-missing", RepoID: "repo-1", DeviceID: "d2", Path: "devices/d2/vyzorix.toml", State: provisioning.ResourceMissing, AppliedAt: now},
		{ID: "r-error", RepoID: "repo-1", DeviceID: "d3", Path: "devices/d3/vyzorix.toml", State: provisioning.ResourceError, AppliedAt: now},
		// Another repository's row must not be counted.
		{ID: "r-other", RepoID: "repo-2", DeviceID: "d4", Path: "devices/d4/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: now},
	}
	for _, r := range seed {
		if err := store.Upsert(ctx, r); err != nil {
			t.Fatalf("Upsert %s: %v", r.ID, err)
		}
	}

	stats, err := store.StatsByRepo(ctx, "repo-1")
	if err != nil {
		t.Fatalf("StatsByRepo: %v", err)
	}

	if stats.Applied != 1 || stats.Missing != 1 || stats.Error != 1 || stats.Drifted != 0 {
		t.Fatalf("stats = %+v, want applied/missing/error = 1/1/1", stats)
	}
	if stats.Total != 3 {
		t.Fatalf("total = %d, want 3 (another repository's row must not count)", stats.Total)
	}

	// Releasing the devices removes them from the counts: the repository no longer
	// owns them. keep_resources=true is the path a delete takes, so the stats are read
	// after the same call the service makes.
	if err := store.DeleteRepository(ctx, "org-1", "repo-1", true); err != nil {
		t.Fatalf("DeleteRepository(keepResources): %v", err)
	}

	stats, err = store.StatsByRepo(ctx, "repo-1")
	if err != nil {
		t.Fatalf("StatsByRepo after release: %v", err)
	}
	if stats.Total != 0 {
		t.Fatalf("total after release = %d, want 0", stats.Total)
	}
}

// TestProvisioningJobScopedToRepo proves a job id from another repository cannot
// be read through the wrong repository.
func TestProvisioningJobScopedToRepo(t *testing.T) {
	store, ctx := newResourceFixture(t)

	job := &provisioning.Job{ID: "job-1", RepoID: "repo-1", State: provisioning.SyncWorking, StartedAt: time.Now()}
	if err := store.StartJob(ctx, job); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	if _, err := store.GetJob(ctx, "repo-2", "job-1"); !errors.Is(err, provisioning.ErrJobNotFound) {
		t.Fatalf("got %v, want ErrJobNotFound", err)
	}
}

// TestProvisioningRenameWithinRepoUpdatesTheRow is the guard for a manifest rename.
//
// A repository that renames vyzorix.toml to vyzorix.v keeps the same device. The service
// carries the existing row's id across the rename, and Upsert must move the path with it.
// Conflicting on (repo_id, path) instead tried to insert a second row for the same device
// and tripped the one-owner index, so the rename read as "owned by another repository".
func TestProvisioningRenameWithinRepoUpdatesTheRow(t *testing.T) {
	store, ctx := newResourceFixture(t)

	if err := store.Upsert(ctx, &provisioning.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if err := store.Upsert(ctx, &provisioning.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.v", State: provisioning.ResourceApplied, AppliedAt: time.Now(),
	}); err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	got, err := store.GetByID(ctx, "repo-1", "res-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Path != "devices/device-a/vyzorix.v" {
		t.Fatalf("path = %q, want the renamed path", got.Path)
	}
}

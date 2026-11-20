package storage

import (
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestRepositoryScopeRoundTrips proves the include/exclude patterns survive a save and a
// read, and that an empty filter stays empty rather than becoming NULL.
func TestRepositoryScopeRoundTrips(t *testing.T) {
	store, ctx := newResourceFixture(t)

	now := time.Now()
	repo := &provisioning.Repository{
		ID: "repo-scope", OrgID: "org-1", ConnectionID: "conn-1", Name: "scoped",
		Type: provisioning.ConnectionGeneric, URL: "https://example.test/r.git", Branch: "main",
		Path: "/", SyncTarget: provisioning.SyncTargetDevice, IntervalSeconds: 300,
		Enabled: true, SyncState: provisioning.SyncPending,
		Include:   []string{"devices/*/vyzorix.toml"},
		Exclude:   []string{"devices/staging/*"},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.SaveRepository(ctx, repo); err != nil {
		t.Fatalf("SaveRepository: %v", err)
	}

	got, err := store.GetRepository(ctx, "org-1", "repo-scope")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	if len(got.Include) != 1 || got.Include[0] != "devices/*/vyzorix.toml" {
		t.Fatalf("include = %v", got.Include)
	}
	if len(got.Exclude) != 1 || got.Exclude[0] != "devices/staging/*" {
		t.Fatalf("exclude = %v", got.Exclude)
	}

	// An unscoped repository must read back with no patterns, not a stray empty string.
	bare := &provisioning.Repository{
		ID: "repo-bare", OrgID: "org-1", ConnectionID: "conn-1", Name: "bare",
		Type: provisioning.ConnectionGeneric, URL: "https://example.test/b.git", Branch: "main",
		Path: "/", SyncTarget: provisioning.SyncTargetDevice, IntervalSeconds: 300,
		Enabled: true, SyncState: provisioning.SyncPending,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.SaveRepository(ctx, bare); err != nil {
		t.Fatalf("SaveRepository(bare): %v", err)
	}

	gotBare, err := store.GetRepository(ctx, "org-1", "repo-bare")
	if err != nil {
		t.Fatalf("GetRepository(bare): %v", err)
	}
	if len(gotBare.Include) != 0 || len(gotBare.Exclude) != 0 {
		t.Fatalf("unscoped repository read back with patterns: include=%v exclude=%v",
			gotBare.Include, gotBare.Exclude)
	}
}

// TestStatsCountIgnored proves the per-state stats include ignored, so a repository's
// counts add up after a file falls out of scope.
func TestStatsCountIgnored(t *testing.T) {
	store, ctx := newResourceFixture(t)

	now := time.Now()
	for _, r := range []*provisioning.Resource{
		{ID: "r-applied", RepoID: "repo-1", DeviceID: "d1", Path: "devices/d1/vyzorix.toml",
			State: provisioning.ResourceApplied, AppliedAt: now},
		{ID: "r-ignored", RepoID: "repo-1", DeviceID: "d2", Path: "devices/d2/vyzorix.toml",
			State: provisioning.ResourceIgnored, AppliedAt: now},
	} {
		if err := store.Upsert(ctx, r); err != nil {
			t.Fatalf("Upsert %s: %v", r.ID, err)
		}
	}

	stats, err := store.StatsByRepo(ctx, "repo-1")
	if err != nil {
		t.Fatalf("StatsByRepo: %v", err)
	}
	if stats.Ignored != 1 || stats.Applied != 1 {
		t.Fatalf("stats = %+v, want applied=1 ignored=1", stats)
	}
	if stats.Total != 2 {
		t.Fatalf("total = %d, want 2", stats.Total)
	}
}

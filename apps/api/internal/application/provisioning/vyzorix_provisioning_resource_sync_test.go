package provisioning

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// twoDeviceTree is a repository with two device manifests, for the single-resource tests.
func twoDeviceTree(t *testing.T) string {
	t.Helper()

	return writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
		"devices/device-b/vyzorix.toml": validManifest,
	})
}

// TestSyncResourceAppliesOnlyThatResource is the core guarantee: a single-resource sync
// applies one file and leaves the other alone, even though the plan knew about it.
func TestSyncResourceAppliesOnlyThatResource(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})
	resources.seed(&domain.Resource{
		ID: "res-b", RepoID: "repo-1", DeviceID: "device-b",
		Path: "devices/device-b/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	deployer := &fakeDeployer{}
	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, twoDeviceTree(t), resources, jobs, deployer)

	if _, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{}); err != nil {
		t.Fatalf("SyncResource: %v", err)
	}

	if len(deployer.calls) != 1 {
		t.Fatalf("deployed %d times, want 1", len(deployer.calls))
	}
	if deployer.calls[0].DeviceID != "device-a" {
		t.Fatalf("deployed to %q, want device-a", deployer.calls[0].DeviceID)
	}
}

// TestSyncResourceRecordsTheRestAsSkipped proves the job tells the truth about what it did
// not do. A job that applied one file must not imply the others were unchanged.
func TestSyncResourceRecordsTheRestAsSkipped(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})
	resources.seed(&domain.Resource{
		ID: "res-b", RepoID: "repo-1", DeviceID: "device-b",
		Path: "devices/device-b/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, twoDeviceTree(t), resources, jobs, &fakeDeployer{})

	result, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{})
	if err != nil {
		t.Fatalf("SyncResource: %v", err)
	}

	var skipped int
	for _, r := range jobs.results {
		if r.JobID != result.Job.ID {
			continue
		}
		if r.Outcome == domain.ResultSkipped {
			skipped++
		}
	}

	if skipped != 1 {
		t.Fatalf("recorded %d skipped results, want 1 (the other device)", skipped)
	}
	if result.Job.Skipped != 1 {
		t.Fatalf("job skipped count = %d, want 1", result.Job.Skipped)
	}
}

// TestSyncResourceRejectsAResourceFromAnotherRepository proves the id is scoped. A resource
// id belonging to a different repository must be a not-found, never a cross-repository
// apply.
func TestSyncResourceRejectsAResourceFromAnotherRepository(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-other", RepoID: "repo-2", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied,
	})

	svc := newSyncServiceForTest(t, twoDeviceTree(t), resources, newFakeJobs(), &fakeDeployer{})

	_, err := svc.SyncResource(context.Background(), testRepo(), "res-other", SyncOptions{})
	if !errors.Is(err, domain.ErrResourceNotFound) {
		t.Fatalf("got %v, want ErrResourceNotFound", err)
	}
}

// TestSyncResourceDoesNotStampTheRepository proves a single-resource sync leaves the
// repository's sync state alone. It did not sync the repository, so claiming the whole
// repository is current would be false.
func TestSyncResourceDoesNotStampTheRepository(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	repos := newStampingRepositoryStore()
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{ID: "conn-1", OrgID: "org-1"}
	conns.tokens["conn-1"] = "token"

	svc := NewSyncService(&fakeFetcher{dir: twoDeviceTree(t)}, resources, conns, repos, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(1000, 0) }

	if _, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{}); err != nil {
		t.Fatalf("SyncResource: %v", err)
	}

	if repos.stamps != 0 {
		t.Fatalf("a single-resource sync stamped the repository %d time(s)", repos.stamps)
	}
}

// TestSyncResourceDryRunAppliesNothing proves a preview of one resource still writes
// nothing, like a full preview.
func TestSyncResourceDryRunAppliesNothing(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	deployer := &fakeDeployer{}
	svc := newSyncServiceForTest(t, twoDeviceTree(t), resources, newFakeJobs(), deployer)

	if _, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("SyncResource dry run: %v", err)
	}

	if len(deployer.calls) != 0 {
		t.Fatalf("a dry run deployed %d times", len(deployer.calls))
	}
}

// TestSyncResourceRespectsTheLock proves a single-resource sync takes the same per-device
// lock, so it cannot interleave with a full sync.
func TestSyncResourceRespectsTheLock(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	svc := newSyncServiceForTest(t, twoDeviceTree(t), resources, newFakeJobs(), &fakeDeployer{})

	if !svc.locks.acquire("repo-1") {
		t.Fatal("could not take the lock for setup")
	}
	defer svc.locks.release("repo-1")

	if _, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{}); !errors.Is(err, domain.ErrSyncInProgress) {
		t.Fatalf("got %v, want ErrSyncInProgress", err)
	}
}

// TestSyncResourceDryRunDoesNotReportSkippedAsFailure is the guard for a preview misreading
// the narrowed plan. A skipped item has no action, and the preview's default branch treats
// an unknown action as a failure, so a dry run of one resource used to report the other
// device as failed.
func TestSyncResourceDryRunDoesNotReportSkippedAsFailure(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})
	resources.seed(&domain.Resource{
		ID: "res-b", RepoID: "repo-1", DeviceID: "device-b",
		Path: "devices/device-b/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	svc := newSyncServiceForTest(t, twoDeviceTree(t), resources, newFakeJobs(), &fakeDeployer{})

	result, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("SyncResource dry run: %v", err)
	}

	if result.Job.Failed != 0 {
		t.Fatalf("a dry run of one resource reported %d failures", result.Job.Failed)
	}
	if result.Job.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", result.Job.Skipped)
	}
}

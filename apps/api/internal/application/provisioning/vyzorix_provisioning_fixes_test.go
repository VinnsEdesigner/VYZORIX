package provisioning

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestDiscoverRejectsSymlink is the guard for the file-disclosure defect: a
// committed symlink must never be followed, because the host files it can reach
// include the server's own secrets.
func TestDiscoverRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "devices", "device-a")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	secret := filepath.Join(root, "host-secret.txt")
	if err := os.WriteFile(secret, []byte("TOP_SECRET_HOST_CONTENT"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "vyzorix.toml")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	manifests, err := Discover(root, "/")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("got %d manifests, want 1", len(manifests))
	}

	m := manifests[0]
	if m.Err == nil {
		t.Fatal("a symlinked manifest was accepted")
	}
	if len(m.Bytes) != 0 {
		t.Fatalf("a symlinked manifest was read: %d bytes", len(m.Bytes))
	}
}

// TestDiscoverRejectsEscapingPath proves a configured subdirectory cannot walk out
// of the checkout.
func TestDiscoverRejectsEscapingPath(t *testing.T) {
	root := t.TempDir()

	if _, err := Discover(root, "../../etc"); err == nil {
		t.Fatal("a path escaping the checkout was accepted")
	}
}

// TestDiscoverAnchorsPathAtRoot is the guard for the path/device-binding defect: a
// repository that syncs a subdirectory must still resolve devices, and the
// resource path must not change when the subdirectory does.
func TestDiscoverAnchorsPathAtRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "devices", "device-a")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vyzorix.toml"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	manifests, err := Discover(root, "/apps")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("got %d manifests, want 1", len(manifests))
	}
	if manifests[0].Err != nil {
		t.Fatalf("a manifest under a synced subdirectory failed: %v", manifests[0].Err)
	}
	if manifests[0].DeviceID != "device-a" {
		t.Fatalf("device = %q, want device-a", manifests[0].DeviceID)
	}
	if want := "devices/device-a/vyzorix.toml"; manifests[0].Path != want {
		t.Fatalf("path = %q, want %q", manifests[0].Path, want)
	}
}

// TestLocksAreThreadSafe is the guard for the data race: the service is shared by
// every handler and the scheduler, so concurrent acquisition must be safe and must
// still admit exactly one holder.
func TestLocksAreThreadSafe(t *testing.T) {
	l := newLocks()

	const goroutines = 64

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		acquired int
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if l.acquire("repo-1") {
				mu.Lock()
				acquired++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if acquired != 1 {
		t.Fatalf("%d goroutines acquired the lock, want exactly 1", acquired)
	}
}

// TestSyncRevertsMissingOnlyWhenAsked proves revert_missing performs a real
// rollback, and that the default does not touch the device.
func TestSyncRevertsMissingOnlyWhenAsked(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// Remove the manifest, then sync with revert.
	s.fetcher = &fakeFetcher{dir: writeTree(t, map[string]string{"README.md": "empty"})}

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{RevertMissing: true}); err != nil {
		t.Fatalf("revert sync: %v", err)
	}

	if len(dep.rollbacks) != 1 || dep.rollbacks[0] != "device-a" {
		t.Fatalf("rollbacks = %v, want one for device-a", dep.rollbacks)
	}
}

// TestSyncDefaultDoesNotRevert pins the default: a deletion must not touch a device.
func TestSyncDefaultDoesNotRevert(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	s.fetcher = &fakeFetcher{dir: writeTree(t, map[string]string{"README.md": "empty"})}

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	if len(dep.rollbacks) != 0 {
		t.Fatalf("the default reverted %d devices, want 0", len(dep.rollbacks))
	}
}

// TestSyncRevertFailureIsReported proves a failed rollback is a failure, not a
// success message describing work that did not happen.
func TestSyncRevertFailureIsReported(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	jobs := newFakeJobs()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, jobs, dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	dep.rollbackErr = deploy.ErrNothingToRevert
	s.fetcher = &fakeFetcher{dir: writeTree(t, map[string]string{"README.md": "empty"})}

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{RevertMissing: true})
	if err != nil {
		t.Fatalf("revert sync: %v", err)
	}

	if res.Job.Failed != 1 {
		t.Fatalf("failed = %d, want 1 (the revert did not happen)", res.Job.Failed)
	}
}

// TestSyncDetectsDrift proves a device changed by something else is re-applied even
// when the manifest is byte-identical.
func TestSyncDetectsDrift(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	dep := &fakeDeployer{versions: map[string]int{}}
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if len(dep.calls) != 1 {
		t.Fatalf("first sync deployed %d, want 1", len(dep.calls))
	}

	// The device now runs a different version: something else deployed to it.
	dep.versions["device-a"] = 99

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("drift sync: %v", err)
	}

	if len(dep.calls) != 2 {
		t.Fatalf("a drifted device was not re-applied (calls = %d)", len(dep.calls))
	}
	if res.Job.Applied != 1 {
		t.Fatalf("applied = %d, want 1", res.Job.Applied)
	}
}

// TestSyncConflictOutcomeMatchesPreview pins the parity: a conflict must read the
// same in the preview and the real run.
func TestSyncConflictOutcomeMatchesPreview(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	resources.seed(&domain.Resource{ID: "r-other", RepoID: "repo-other", DeviceID: "device-a", Path: "devices/device-a/vyzorix.toml"})

	jobs := newFakeJobs()
	s := newSyncServiceForTest(t, root, resources, jobs, &fakeDeployer{})

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("dry run: %v", err)
	}

	preview := jobs.results[0].Outcome

	jobs.results = nil

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if jobs.results[0].Outcome != preview {
		t.Fatalf("preview said %q, apply said %q", preview, jobs.results[0].Outcome)
	}
}

// TestSchedulerSyncsDueRepositories proves the interval loop actually runs a sync,
// which is the whole point of the scheduler.
func TestSchedulerSyncsDueRepositories(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	repos := newFakeRepositoryStore()
	repo := testRepo()
	repos.repos[repo.ID] = repo

	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{ID: "conn-1", OrgID: "org-1", Type: domain.ConnectionGitHub}
	conns.tokens["conn-1"] = "token"

	dep := &fakeDeployer{}
	s := NewSyncService(
		&fakeFetcher{dir: root},
		newFakeResources(),
		conns,
		dueStore{repo: repo},
		dep,
		newFakeJobs(),
		nil,
	)

	scheduler := NewScheduler(dueStore{repo: repo}, s, nil, 0)
	scheduler.tickOnce(context.Background())

	if len(dep.calls) != 1 {
		t.Fatalf("the scheduler deployed %d manifests, want 1", len(dep.calls))
	}
}

// dueStore reports one due repository, so a scheduler tick has work to do.
type dueStore struct{ repo *domain.Repository }

func (d dueStore) ListDueForSync(context.Context, time.Time) ([]*domain.Repository, error) {
	return []*domain.Repository{d.repo}, nil
}

func (d dueStore) GetRepository(context.Context, string, string) (*domain.Repository, error) {
	return d.repo, nil
}

func (d dueStore) UpdateSyncStatus(context.Context, string, domain.SyncState, string, time.Time) error {
	return nil
}

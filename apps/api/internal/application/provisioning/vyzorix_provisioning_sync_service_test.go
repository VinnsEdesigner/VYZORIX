package provisioning

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

const validManifest = `[device]
id = "device-a"
org = "org-1"
name = "Device A"

[[app]]
name = "web"
cmd = "serve"
`

// fakeFetcher returns a prepared tree, so discovery and the diff can be tested
// without a network. Each Fetch copies the tree into a fresh directory, because a
// real fetch does: the checkout is owned by the sync and removed when it ends.
type fakeFetcher struct {
	err error
	dir string
}

func (f *fakeFetcher) Fetch(context.Context, *domain.Repository, redaction.Secret) (*Checkout, error) {
	if f.err != nil {
		return nil, f.err
	}

	dir, err := os.MkdirTemp("", "fake-checkout-*")
	if err != nil {
		return nil, err
	}
	if err := copyTree(f.dir, dir); err != nil {
		_ = os.RemoveAll(dir)

		return nil, err
	}

	return &Checkout{Dir: dir}, nil
}

// copyTree recursively copies src into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}

		data, err := os.ReadFile(path) // #nosec G304 -- path walked from the fixture root.
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, 0o600)
	})
}

// fakeResources is an in-memory ResourceRepository.
type fakeResources struct {
	byDevice map[string]*domain.Resource
	byPath   map[string]*domain.Resource
	states   []domain.ResourceState
	upserts  int
}

func newFakeResources() *fakeResources {
	return &fakeResources{
		byDevice: map[string]*domain.Resource{},
		byPath:   map[string]*domain.Resource{},
	}
}

func (f *fakeResources) seed(r *domain.Resource) {
	copied := *r
	f.byDevice[r.DeviceID] = &copied
	f.byPath[r.Path] = &copied
}

func (f *fakeResources) Upsert(_ context.Context, r *domain.Resource) error {
	if owner, ok := f.byDevice[r.DeviceID]; ok && owner.RepoID != r.RepoID {
		return &domain.DeviceOwnedError{DeviceID: r.DeviceID, Owner: owner.RepoID}
	}
	f.upserts++
	copied := *r
	f.byDevice[r.DeviceID] = &copied
	f.byPath[r.Path] = &copied

	return nil
}

func (f *fakeResources) GetByDevice(_ context.Context, deviceID string) (*domain.Resource, error) {
	r, ok := f.byDevice[deviceID]
	if !ok {
		return nil, domain.ErrResourceNotFound
	}
	copied := *r

	return &copied, nil
}

func (f *fakeResources) GetByID(_ context.Context, repoID, id string) (*domain.Resource, error) {
	for _, r := range f.byPath {
		if r.ID == id && r.RepoID == repoID {
			copied := *r

			return &copied, nil
		}
	}

	return nil, domain.ErrResourceNotFound
}

func (f *fakeResources) ListByRepo(_ context.Context, repoID string) ([]*domain.Resource, error) {
	var out []*domain.Resource
	for _, r := range f.byPath {
		if r.RepoID == repoID {
			copied := *r
			out = append(out, &copied)
		}
	}

	return out, nil
}

func (f *fakeResources) SetState(_ context.Context, id string, state domain.ResourceState, lastError string) error {
	f.states = append(f.states, state)
	for _, r := range f.byDevice {
		if r.ID == id {
			r.State = state
			r.LastError = lastError
		}
	}
	for _, r := range f.byPath {
		if r.ID == id {
			r.State = state
			r.LastError = lastError
		}
	}

	return nil
}

// UpdatePath moves a resource's path, mirroring the storage update.
func (f *fakeResources) UpdatePath(_ context.Context, id, path string) error {
	for _, r := range f.byDevice {
		if r.ID == id {
			delete(f.byPath, r.Path)
			r.Path = path
			f.byPath[path] = r

			return nil
		}
	}

	return domain.ErrResourceNotFound
}

// StatsByRepo counts the fake's resources by state, mirroring the storage query.
func (f *fakeResources) StatsByRepo(_ context.Context, repoID string) (*domain.ResourceStats, error) {
	stats := &domain.ResourceStats{}

	for _, r := range f.byPath {
		if r.RepoID != repoID || r.ReleasedAt != nil {
			continue
		}

		switch r.State {
		case domain.ResourceApplied:
			stats.Applied++
		case domain.ResourceDrifted:
			stats.Drifted++
		case domain.ResourceMissing:
			stats.Missing++
		case domain.ResourceError:
			stats.Error++
		}
		stats.Total++
	}

	return stats, nil
}

// fakeJobs is an in-memory JobRepository.
type fakeJobs struct {
	jobs    map[string]*domain.Job
	results []*domain.JobResult
}

func newFakeJobs() *fakeJobs { return &fakeJobs{jobs: map[string]*domain.Job{}} }

func (f *fakeJobs) StartJob(_ context.Context, j *domain.Job) error {
	copied := *j
	f.jobs[j.ID] = &copied

	return nil
}

func (f *fakeJobs) FinishJob(_ context.Context, j *domain.Job) error {
	copied := *j
	f.jobs[j.ID] = &copied

	return nil
}

func (f *fakeJobs) GetJob(_ context.Context, _, id string) (*domain.Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return nil, domain.ErrJobNotFound
	}
	copied := *j

	return &copied, nil
}

func (f *fakeJobs) ListJobs(context.Context, string, int) ([]*domain.Job, error) { return nil, nil }

func (f *fakeJobs) SaveResult(_ context.Context, r *domain.JobResult) error {
	copied := *r
	f.results = append(f.results, &copied)

	return nil
}

func (f *fakeJobs) ListResults(context.Context, string) ([]*domain.JobResult, error) {
	return f.results, nil
}

func (f *fakeJobs) FailInterrupted(context.Context, time.Time) (int, error) { return 0, nil }

// Health derives the same aggregate the storage query produces, from the fake's
// recorded jobs, so the service tests exercise the real shape.
func (f *fakeJobs) Health(_ context.Context, repoID string, limit int) (*domain.RepositoryHealth, error) {
	if limit <= 0 {
		limit = 20
	}

	health := &domain.RepositoryHealth{}
	var total int64
	var points []domain.HealthPoint

	for _, j := range f.jobs {
		if j.RepoID != repoID || j.DryRun || j.FinishedAt == nil {
			continue
		}

		points = append(points, domain.HealthPoint{
			FinishedAt: *j.FinishedAt,
			State:      j.State,
			DurationMs: j.DurationMs,
			Failed:     j.Failed,
		})
		total += j.DurationMs

		if j.State == domain.SyncError {
			health.Failures++
		} else {
			health.Successes++
		}
	}

	if len(points) > limit {
		points = points[:limit]
	}
	if len(points) == 0 {
		return health, nil
	}

	health.Timeline = points
	health.LastDurationMs = points[len(points)-1].DurationMs
	health.AverageDurationMs = total / int64(len(points))

	for i := len(points) - 1; i >= 0; i-- {
		if points[i].State != domain.SyncError {
			break
		}
		health.FailureStreak++
	}

	return health, nil
}

// fakeDeployer records each deploy request so a test can assert exactly what was
// applied and what a dry run left untouched. It also models the config version a
// device holds, which is what drift detection compares against.
type fakeDeployer struct {
	err         error
	rollbackErr error
	versions    map[string]int
	calls       []deploy.DeployRequest
	rollbacks   []string
}

func (d *fakeDeployer) Rollback(_ context.Context, _ *deploy.Caller, _, deviceID string) (*deployconfig.Record, error) {
	d.rollbacks = append(d.rollbacks, deviceID)
	if d.rollbackErr != nil {
		return nil, d.rollbackErr
	}

	return &deployconfig.Record{DeviceID: deviceID}, nil
}

func (d *fakeDeployer) CurrentVersion(_ context.Context, _, deviceID string) (int, bool, error) {
	v, ok := d.versions[deviceID]

	return v, ok, nil
}

func (d *fakeDeployer) Deploy(_ context.Context, req deploy.DeployRequest) (*deploy.DeployResult, error) {
	d.calls = append(d.calls, req)
	if d.err != nil {
		return nil, d.err
	}

	return &deploy.DeployResult{Version: 7, DeviceID: req.DeviceID}, nil
}

// writeTree materializes a repository layout under a temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	return root
}

func testRepo() *domain.Repository {
	return &domain.Repository{
		ID:              "repo-1",
		OrgID:           "org-1",
		ConnectionID:    "conn-1",
		Name:            "fleet",
		Path:            "/",
		Branch:          "main",
		Enabled:         true,
		SyncTarget:      domain.SyncTargetDevice,
		IntervalSeconds: 300,
		SyncState:       domain.SyncPending,
	}
}

func newSyncServiceForTest(t *testing.T, dir string, resources *fakeResources, jobs *fakeJobs, dep Deployer) *SyncService {
	t.Helper()

	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{ID: "conn-1", OrgID: "org-1", Type: domain.ConnectionGitHub}
	conns.tokens["conn-1"] = "token"

	return NewSyncService(
		&fakeFetcher{dir: dir},
		resources,
		conns,
		newFakeRepositoryStore(),
		dep,
		jobs,
		nil,
	)
}

// TestSync_DryRunWritesNothing is the contract's guarantee: a preview never
// changes a device. The deploy service is asked nothing and no resource row is
// written, but the job still records what the run would have done.
func TestSync_DryRunWritesNothing(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
	})

	resources := newFakeResources()
	jobs := newFakeJobs()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, jobs, dep)

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(dep.calls) != 0 {
		t.Fatalf("dry run deployed %d manifests, want 0", len(dep.calls))
	}
	if resources.upserts != 0 {
		t.Fatalf("dry run wrote %d resources, want 0", resources.upserts)
	}
	if !res.Job.DryRun {
		t.Fatal("job is not marked as a dry run")
	}
	if len(jobs.results) != 1 || jobs.results[0].Outcome != domain.ResultApplied {
		t.Fatalf("preview results = %+v, want one applied", jobs.results)
	}
	if jobs.results[0].Message != "would apply" {
		t.Fatalf("preview message = %q, want %q", jobs.results[0].Message, "would apply")
	}
}

// TestSync_AppliesNewManifest is the real run: it deploys through the service and
// records ownership with the content hash, which is what makes drift detectable.
func TestSync_AppliesNewManifest(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
	})

	resources := newFakeResources()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)
	// A confirming reader, so the device is treated as running the signed version.
	s.SetDeviceConfirmation(fakeConfirmation{confirmsAll: true})

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(dep.calls) != 1 {
		t.Fatalf("deployed %d manifests, want 1", len(dep.calls))
	}
	if dep.calls[0].DeviceID != "device-a" || dep.calls[0].OrgID != "org-1" {
		t.Fatalf("deploy target = %s/%s, want org-1/device-a", dep.calls[0].OrgID, dep.calls[0].DeviceID)
	}

	owned, err := resources.GetByDevice(context.Background(), "device-a")
	if err != nil {
		t.Fatalf("resource not recorded: %v", err)
	}
	if owned.ContentHash == "" || owned.ConfigVersion != 7 || owned.State != domain.ResourceApplied {
		t.Fatalf("resource = %+v, want applied with a hash and version 7", owned)
	}
	if res.Job.State != domain.SyncSuccess || res.Job.Applied != 1 {
		t.Fatalf("job = %+v, want success with 1 applied", res.Job)
	}
}

// TestSync_UnchangedManifestIsNotReapplied pins idempotence: re-running with no
// repo change must perform no writes.
func TestSync_UnchangedManifestIsNotReapplied(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
	})

	resources := newFakeResources()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if len(dep.calls) != 1 {
		t.Fatalf("first sync deployed %d, want 1", len(dep.calls))
	}

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}

	if len(dep.calls) != 1 {
		t.Fatalf("second sync re-deployed an unchanged manifest (calls = %d)", len(dep.calls))
	}
	if res.Job.Applied != 0 || res.Job.Skipped != 1 {
		t.Fatalf("job = %+v, want 0 applied and 1 skipped", res.Job)
	}
}

// TestSync_ChangedManifestIsUpdated proves a hash change triggers a re-apply.
func TestSync_ChangedManifestIsUpdated(t *testing.T) {
	resources := newFakeResources()
	dep := &fakeDeployer{}

	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	changed := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest + "\n[[app]]\nname = \"api\"\ncmd = \"run\"\n",
	})
	s.fetcher = &fakeFetcher{dir: changed}

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}

	if len(dep.calls) != 2 {
		t.Fatalf("deployed %d, want 2 (a change must re-apply)", len(dep.calls))
	}
	if res.Job.Applied != 1 {
		t.Fatalf("job applied = %d, want 1", res.Job.Applied)
	}
}

// TestSync_DeletedManifestIsFlaggedNotReverted is the contract's deletion policy:
// a file gone from the repo moves its resource to missing and nothing reverts.
func TestSync_DeletedManifestIsFlaggedNotReverted(t *testing.T) {
	resources := newFakeResources()
	dep := &fakeDeployer{}

	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), dep)

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	s.fetcher = &fakeFetcher{dir: writeTree(t, map[string]string{"README.md": "empty"})}

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}

	if len(dep.calls) != 1 {
		t.Fatalf("a deletion deployed %d, want 1 (nothing reverts)", len(dep.calls))
	}
	if len(resources.states) != 1 || resources.states[0] != domain.ResourceMissing {
		t.Fatalf("states = %v, want one missing", resources.states)
	}
	if res.Job.State != domain.SyncWarning {
		t.Fatalf("job state = %s, want warning", res.Job.State)
	}
}

// TestSync_ConflictNamesTheOwner proves the one-owner rule surfaces before the
// write, naming the repository that already holds the device.
func TestSync_ConflictNamesTheOwner(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	resources.seed(&domain.Resource{ID: "r-other", RepoID: "repo-other", DeviceID: "device-a", Path: "devices/device-a/vyzorix.toml"})

	jobs := newFakeJobs()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, jobs, dep)

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(dep.calls) != 0 {
		t.Fatalf("a conflicting device was deployed to (%d calls)", len(dep.calls))
	}
	if res.Job.State != domain.SyncError || res.Job.Failed != 1 {
		t.Fatalf("job = %+v, want error with 1 failure", res.Job)
	}
	if len(jobs.results) != 1 || !strings.Contains(jobs.results[0].Message, "repo-other") {
		t.Fatalf("conflict message does not name the owner: %+v", jobs.results)
	}
}

// TestSync_OneBadManifestDoesNotBlockTheRest is the per-resource error rule.
func TestSync_OneBadManifestDoesNotBlockTheRest(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
		"devices/device-b/vyzorix.toml": validManifest,
		"not-a-device/vyzorix.toml":     validManifest,
	})

	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), dep)

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(dep.calls) != 2 {
		t.Fatalf("deployed %d, want 2 (the misplaced file must not block the others)", len(dep.calls))
	}
	if res.Job.Failed != 1 {
		t.Fatalf("job failed = %d, want 1", res.Job.Failed)
	}
}

// TestSync_FailedApplyIsNotCountedAsApplied pins the counter against the plan:
// a deploy that is refused must report failed, not applied. Counting the plan
// would tell a user their config landed when it did not.
func TestSync_FailedApplyIsNotCountedAsApplied(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	jobs := newFakeJobs()
	dep := &fakeDeployer{err: errors.New("deploy denied: not_in_org")}
	s := newSyncServiceForTest(t, root, resources, jobs, dep)

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if res.Job.Applied != 0 {
		t.Fatalf("job applied = %d, want 0 (the deploy was refused)", res.Job.Applied)
	}
	if res.Job.Failed != 1 || res.Job.State != domain.SyncError {
		t.Fatalf("job = %+v, want 1 failure and an error state", res.Job)
	}
	if resources.upserts != 0 {
		t.Fatalf("a refused deploy recorded %d resources, want 0", resources.upserts)
	}
}

// TestSync_DisabledRepositoryRefuses pins that a disabled repository does no work.
func TestSync_DisabledRepositoryRefuses(t *testing.T) {
	repo := testRepo()
	repo.Enabled = false

	s := newSyncServiceForTest(t, t.TempDir(), newFakeResources(), newFakeJobs(), &fakeDeployer{})

	if _, err := s.Sync(context.Background(), repo, SyncOptions{}); !errors.Is(err, ErrSyncDisabled) {
		t.Fatalf("got %v, want ErrSyncDisabled", err)
	}
}

// TestSync_FetchFailureRecordsError proves a broken remote fails the job and the
// repository, with the reason kept.
func TestSync_FetchFailureRecordsError(t *testing.T) {
	s := newSyncServiceForTest(t, t.TempDir(), newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.fetcher = &fakeFetcher{err: ErrFetchFailed}

	repo := testRepo()
	if _, err := s.Sync(context.Background(), repo, SyncOptions{}); !errors.Is(err, ErrFetchFailed) {
		t.Fatalf("got %v, want ErrFetchFailed", err)
	}
	if repo.SyncState != domain.SyncError || repo.LastError == "" {
		t.Fatalf("repository = %+v, want error with a reason", repo)
	}
}

package provisioning

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

var errUnparseable = errors.New("vyzorix.toml is not valid TOML")

// TestDryRunDoesNotStampRepository is the guard for a preview resetting the timer.
//
// finishJob stamped the repository on every run, including a dry run. last_sync_at is
// what ListDueForSync compares against, so previewing a sync pushed the next real sync
// out by a whole interval — the wizard's step 4 would silently delay the thing it was
// previewing.
func TestDryRunDoesNotStampRepository(t *testing.T) {
	repos := newStampingRepositoryStore()
	resources := newFakeResources()
	svc := NewSyncService(nil, resources, nil, repos, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(1000, 0) }

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}

	job := &domain.Job{ID: "job-1", RepoID: repo.ID, State: domain.SyncWorking,
		StartedAt: time.Unix(900, 0), DryRun: true}
	if _, err := svc.finishJob(context.Background(), repo, job, &tally{}, false); err != nil {
		t.Fatalf("finishJob: %v", err)
	}

	if repos.stamps != 0 {
		t.Fatalf("a dry run stamped the repository %d time(s); it must not touch sync state", repos.stamps)
	}

	// A real run does stamp, so the scheduler still sees the sync.
	realJob := &domain.Job{ID: "job-2", RepoID: repo.ID, State: domain.SyncWorking,
		StartedAt: time.Unix(900, 0)}
	if _, err := svc.finishJob(context.Background(), repo, realJob, &tally{}, false); err != nil {
		t.Fatalf("finishJob (real): %v", err)
	}
	if repos.stamps != 1 {
		t.Fatalf("a real run stamped the repository %d time(s), want 1", repos.stamps)
	}
}

// stampingRepositoryStore counts UpdateSyncStatus calls.
type stampingRepositoryStore struct{ stamps int }

func newStampingRepositoryStore() *stampingRepositoryStore { return &stampingRepositoryStore{} }

func (s *stampingRepositoryStore) GetRepository(context.Context, string, string) (*domain.Repository, error) {
	return nil, domain.ErrRepositoryNotFound
}

func (s *stampingRepositoryStore) ListDueForSync(context.Context, time.Time) ([]*domain.Repository, error) {
	return nil, nil
}

func (s *stampingRepositoryStore) UpdateSyncStatus(context.Context, string, domain.SyncState, string, time.Time) error {
	s.stamps++

	return nil
}

// TestUnappliedManifestMarksResourceError is the guard for a broken manifest reading
// "applied". The contract requires the resource to go to error with the parse message;
// before, an existing row kept its old state, so a manifest broken in git still claimed
// the device matched it.
func TestUnappliedManifestMarksResourceError(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "d1",
		Path: "devices/d1/vyzorix.toml", State: domain.ResourceApplied,
	})

	svc := NewSyncService(nil, resources, nil, nil, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}
	job := &domain.Job{ID: "job-1", RepoID: repo.ID}

	plan := &Plan{Items: []*PlanItem{
		{
			Manifest: &Manifest{Path: "devices/d1/vyzorix.toml", DeviceID: "d1",
				Err: errUnparseable},
			Resource: &domain.Resource{ID: "res-1", RepoID: repo.ID, DeviceID: "d1",
				Path: "devices/d1/vyzorix.toml", State: domain.ResourceApplied},
			Action:  ActionError,
			Message: errUnparseable.Error(),
		},
	}}

	svc.apply(context.Background(), repo, job, plan, SyncOptions{})

	got, err := resources.GetByID(context.Background(), repo.ID, "res-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.State != domain.ResourceError {
		t.Fatalf("state = %q, want error for a manifest that could not be applied", got.State)
	}
}

// TestFinishJobSummarisesPerResourceFailures proves a red run carries a reason. Without
// one the repository showed a red badge and an empty last_error.
func TestFinishJobSummarisesPerResourceFailures(t *testing.T) {
	svc := NewSyncService(nil, newFakeResources(), nil, newStampingRepositoryStore(),
		&fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(1000, 0) }

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}
	job := &domain.Job{ID: "job-1", RepoID: repo.ID, State: domain.SyncWorking, StartedAt: time.Unix(900, 0)}

	result, err := svc.finishJob(context.Background(), repo, job, &tally{failed: 3}, false)
	if err != nil {
		t.Fatalf("finishJob: %v", err)
	}
	if result.Job.State != domain.SyncError {
		t.Fatalf("state = %q, want error", result.Job.State)
	}
	if result.Job.Error == "" {
		t.Fatal("a failed run recorded no reason; the badge would be red with no explanation")
	}
}

// TestSafeJoinRejectsEscapingPath is the guard for a push-back path traversal. A
// resource path with ".." would let an edit read or overwrite a file outside the clone.
func TestSafeJoinRejectsEscapingPath(t *testing.T) {
	root := t.TempDir()

	bad := []string{"../secret", "devices/../../secret", "../../etc/passwd"}
	for _, rel := range bad {
		if _, err := safeJoin(root, rel); err == nil {
			t.Errorf("safeJoin accepted an escaping path: %q", rel)
		}
	}

	good := []string{"devices/d1/vyzorix.toml", "a/b/c.toml"}
	for _, rel := range good {
		if _, err := safeJoin(root, rel); err != nil {
			t.Errorf("safeJoin rejected a contained path %q: %v", rel, err)
		}
	}
}

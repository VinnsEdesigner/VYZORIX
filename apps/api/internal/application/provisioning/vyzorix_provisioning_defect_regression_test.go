package provisioning

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestValidRepositoryURLRejectsUserinfo is the guard for the credential leak.
//
// A URL like https://user:token@host/repo would store the token in plaintext, return it
// from every repository read, and echo it in git's error output — undoing the fetcher's
// deliberate effort to keep the token out of the URL. The credential belongs in the
// connection, where it is encrypted and never returned.
func TestValidRepositoryURLRejectsUserinfo(t *testing.T) {
	bad := []string{
		"https://user:ghp_secret@github.com/acme/fleet.git",
		"https://ghp_secret@github.com/acme/fleet.git",
		"http://user:pass@git.acme.internal/acme/fleet.git",
	}
	for _, raw := range bad {
		if validRepositoryURL(raw) {
			t.Errorf("accepted a URL carrying userinfo: %q", raw)
		}
	}

	good := []string{
		"https://github.com/acme/fleet.git",
		"git@github.com:acme/fleet.git",
		"ssh://git@github.com/acme/fleet.git",
		"https://git.acme.internal/acme/fleet",
	}
	for _, raw := range good {
		if !validRepositoryURL(raw) {
			t.Errorf("rejected a valid URL: %q", raw)
		}
	}
}

// TestCreateRejectsDuplicateRepositoryName proves a taken name is a validation error,
// not a 500. The schema's UNIQUE(org_id, name) used to surface raw and reach the
// handler's default branch, telling the user the server had broken.
func TestCreateRejectsDuplicateRepositoryName(t *testing.T) {
	svc, _, _ := newRepoServiceForTest(t, nil)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "org-1", validRepoInput()); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	store := svc.store.(*fakeRepositoryStore)
	store.duplicateErr = errors.New("UNIQUE constraint failed: provisioning_repositories.org_id, provisioning_repositories.name")

	if _, err := svc.Create(ctx, "org-1", validRepoInput()); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("got %v, want ErrInvalidRepository", err)
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("message does not explain the conflict: %v", err)
	}
}

// TestDriftedStateIsPersisted is the guard for the drift state never being written.
//
// ResourceDrifted existed in the domain and was counted in stats, but nothing ever
// assigned it: a sync that detected drift and was interrupted before re-applying left
// the row reading "applied", which is a false claim about the device.
func TestDriftedStateIsPersisted(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "d1",
		Path: "devices/d1/vyzorix.toml", ConfigVersion: 3, State: domain.ResourceApplied,
	})

	svc := NewSyncService(nil, resources, nil, nil, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true, Branch: "main"}
	job := &domain.Job{ID: "job-1", RepoID: repo.ID}

	item := &PlanItem{
		Manifest: &Manifest{Path: "devices/d1/vyzorix.toml", DeviceID: "d1"},
		Resource: &domain.Resource{ID: "res-1", RepoID: repo.ID, DeviceID: "d1",
			Path: "devices/d1/vyzorix.toml", ConfigVersion: 3},
		Action:  ActionUpdate,
		Drifted: true,
		Message: "drifted: the device's config was changed after this repository applied it",
	}

	svc.applyOne(context.Background(), repo, job, item, &tally{})

	if len(resources.states) == 0 {
		t.Fatal("the drifted state was never written before the re-apply")
	}
	if resources.states[0] != domain.ResourceDrifted {
		t.Fatalf("first state write = %q, want drifted", resources.states[0])
	}

	// The re-apply succeeded, so the row ends up applied — but only after the drift was
	// recorded, which is what makes an interrupted run honest rather than a false
	// "applied".
	got, err := resources.GetByID(context.Background(), repo.ID, "res-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.State != domain.ResourceApplied {
		t.Fatalf("final state = %q, want applied after a successful re-apply", got.State)
	}
}

// TestPlanMarksDriftedItem pins that the plan carries the drift as a field rather than
// only in the message, so the apply path can act on it without parsing prose.
func TestPlanMarksDriftedItem(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "d1",
		Path: "devices/d1/vyzorix.toml", ContentHash: "same", ConfigVersion: 3,
		State: domain.ResourceApplied,
	})

	svc := NewSyncService(nil, resources, nil, nil,
		&fakeDeployer{versions: map[string]int{"d1": 9}}, newFakeJobs(), nil)

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}
	manifest := &Manifest{Path: "devices/d1/vyzorix.toml", DeviceID: "d1", ContentHash: "same"}

	plan, err := svc.Plan(context.Background(), repo, []*Manifest{manifest})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(plan.Items))
	}

	item := plan.Items[0]
	if item.Action != ActionUpdate {
		t.Fatalf("action = %q, want update for a drifted device", item.Action)
	}
	if !item.Drifted {
		t.Fatal("the plan item does not record that the device drifted")
	}
}

// TestPlanTreatsRenameAsUpdate is the guard for a manifest rename being misreported.
//
// When a repository renames devices/x/vyzorix.toml to devices/x/vyzorix.v the device is
// still owned by this repository, just at a different path. The plan must call that an
// update of the existing row. It used to be a create, which hit the one-owner index and
// failed with "device is owned by another repository" — naming the same repository.
func TestPlanTreatsRenameAsUpdate(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "d1",
		Path: "devices/d1/vyzorix.toml", ContentHash: "old", ConfigVersion: 1,
		State: domain.ResourceApplied,
	})

	svc := NewSyncService(nil, resources, nil, nil, &fakeDeployer{}, newFakeJobs(), nil)

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}
	manifest := &Manifest{Path: "devices/d1/vyzorix.v", DeviceID: "d1", ContentHash: "new"}

	plan, err := svc.Plan(context.Background(), repo, []*Manifest{manifest})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(plan.Items))
	}

	item := plan.Items[0]
	if item.Action != ActionUpdate {
		t.Fatalf("action = %q, want update for a renamed manifest", item.Action)
	}
	if item.Resource == nil || item.Resource.ID != "res-1" {
		t.Fatalf("the rename did not reuse the existing row: %+v", item.Resource)
	}
}

// TestUnchangedRenameRepointsThePath is the guard for a stale path after a rename.
//
// A rename from vyzorix.toml to vyzorix.v keeps the same device and the same content, so
// the plan calls it unchanged. But the row records the path, and leaving it on the old one
// means a later delete of the new path reports "missing" against a file that is gone under
// the old name. The unchanged path must re-point the row.
func TestUnchangedRenameRepointsThePath(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "d1",
		Path: "devices/d1/vyzorix.toml", ContentHash: "same", ConfigVersion: 3,
		State: domain.ResourceApplied,
	})

	svc := NewSyncService(nil, resources, nil, nil, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}
	job := &domain.Job{ID: "job-1", RepoID: repo.ID}

	// The file was renamed; the content hash is unchanged.
	item := &PlanItem{
		Manifest: &Manifest{Path: "devices/d1/vyzorix.v", DeviceID: "d1", ContentHash: "same"},
		Resource: &domain.Resource{ID: "res-1", RepoID: repo.ID, DeviceID: "d1",
			Path: "devices/d1/vyzorix.toml", ContentHash: "same", ConfigVersion: 3},
		Action: ActionUnchanged,
	}

	svc.apply(context.Background(), repo, job, &Plan{Items: []*PlanItem{item}}, SyncOptions{})

	got, err := resources.GetByID(context.Background(), repo.ID, "res-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Path != "devices/d1/vyzorix.v" {
		t.Fatalf("path = %q, want the renamed path", got.Path)
	}
}

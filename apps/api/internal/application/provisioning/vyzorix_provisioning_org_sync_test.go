package provisioning

import (
	"context"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// recordingOrgApplier captures the bytes it was asked to apply.
type recordingOrgApplier struct {
	err   error
	orgs  []string
	blobs [][]byte
}

func (r *recordingOrgApplier) Apply(_ context.Context, orgID string, data []byte) (*ApplyResult, error) {
	if r.err != nil {
		return nil, r.err
	}

	r.orgs = append(r.orgs, orgID)
	r.blobs = append(r.blobs, data)

	return &ApplyResult{Detail: "applied"}, nil
}

// TestSyncAppliesOrgFiles proves a full sync writes an org file through its applier, and
// records a job result for it. Without this an orgs/alerts.toml would be fetched and then
// ignored.
func TestSyncAppliesOrgFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/org-1/alerts.toml": "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n",
	})

	applier := &recordingOrgApplier{}
	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, root, newFakeResources(), jobs, &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(applier.blobs) != 1 {
		t.Fatalf("the applier ran %d time(s), want 1", len(applier.blobs))
	}
	if applier.orgs[0] != "org-1" {
		t.Fatalf("applied to org %q, want org-1 (the repository's org)", applier.orgs[0])
	}

	var found bool
	for _, r := range jobs.results {
		if r.Path == "orgs/org-1/alerts.toml" && r.Outcome == domain.ResultApplied {
			found = true
		}
	}
	if !found {
		t.Fatalf("no job result for the org file: %+v", jobs.results)
	}
}

// TestSyncDryRunDoesNotApplyOrgFiles proves a preview writes no org file. A preview must
// change nothing at all, org files included.
func TestSyncDryRunDoesNotApplyOrgFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/org-1/alerts.toml": "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n",
	})

	applier := &recordingOrgApplier{}
	svc := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("Sync dry run: %v", err)
	}

	if len(applier.blobs) != 0 {
		t.Fatalf("a dry run applied %d org file(s)", len(applier.blobs))
	}
}

// TestSyncReportsAnUnconfiguredKind proves a file for a kind with no applier fails that
// file, visibly. Silence would leave a user wondering why their groups.toml does nothing.
func TestSyncReportsAnUnconfiguredKind(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/org-1/groups.toml": "[[group]]\nname = \"warehouse\"\n",
	})

	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, root, newFakeResources(), jobs, &fakeDeployer{})

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	var found bool
	for _, r := range jobs.results {
		if r.Path == "orgs/org-1/groups.toml" && r.Outcome == domain.ResultFailed {
			found = true
		}
	}
	if !found {
		t.Fatalf("an unconfigured kind was not reported: %+v", jobs.results)
	}
}

// TestSyncScopesOrgFiles proves the repository's include/exclude filter applies to org
// files too, so one repository can be limited to devices or to one org's declarations.
func TestSyncScopesOrgFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/org-1/alerts.toml": "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n",
	})

	applier := &recordingOrgApplier{}
	svc := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)

	repo := testRepo()
	repo.Exclude = []string{"orgs/*"}

	if _, err := svc.Sync(context.Background(), repo, SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(applier.blobs) != 0 {
		t.Fatalf("an excluded org file was applied %d time(s)", len(applier.blobs))
	}
}

// TestSyncResourceDoesNotApplyOrgFiles proves a single-device sync does not write an
// organisation's alerts as a side effect.
func TestSyncResourceDoesNotApplyOrgFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
		"orgs/org-1/alerts.toml":        "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n",
	})

	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-a", RepoID: "repo-1", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	applier := &recordingOrgApplier{}
	svc := newSyncServiceForTest(t, root, resources, newFakeJobs(), &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)

	if _, err := svc.SyncResource(context.Background(), testRepo(), "res-a", SyncOptions{}); err != nil {
		t.Fatalf("SyncResource: %v", err)
	}

	if len(applier.blobs) != 0 {
		t.Fatalf("a single-device sync wrote %d org file(s)", len(applier.blobs))
	}
}

// stubOrgNames resolves an org id to a fixed name.
type stubOrgNames struct{ name string }

func (s stubOrgNames) OrgName(context.Context, string) (string, error) { return s.name, nil }

// TestSyncRefusesAnOrgFileForAnotherOrganisation is the guard for a silent wrong-org write.
// The directory names the organisation, so a file in orgs/wrong-org/ must fail rather than
// write to whichever organisation the repository belongs to.
func TestSyncRefusesAnOrgFileForAnotherOrganisation(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/wrong-org/alerts.toml": "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n",
	})

	applier := &recordingOrgApplier{}
	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, root, newFakeResources(), jobs, &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)
	svc.SetOrgNameLookup(stubOrgNames{name: "Right Org"})

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(applier.blobs) != 0 {
		t.Fatal("an org file for another organisation was applied")
	}

	var found bool
	for _, r := range jobs.results {
		if r.Path == "orgs/wrong-org/alerts.toml" && r.Outcome == domain.ResultFailed {
			found = true
		}
	}
	if !found {
		t.Fatalf("the mismatch was not reported: %+v", jobs.results)
	}
}

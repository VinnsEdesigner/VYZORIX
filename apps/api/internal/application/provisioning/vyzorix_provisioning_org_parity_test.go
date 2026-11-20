package provisioning

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

const alertsOrgFile = "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n"

// TestDiscoverReportsOrgFiles is the guard for a preview that hid org files. Discovery is
// the wizard's preview; if it lists only device manifests, the user approves a plan that
// does not mention the alerts a real sync will rewrite.
func TestDiscoverReportsOrgFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
		"orgs/org-1/alerts.toml":        alertsOrgFile,
	})

	svc := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, &recordingOrgApplier{})
	svc.SetOrgNameLookup(stubOrgNames{name: "org-1"})

	discovery, err := svc.Discover(context.Background(), testRepo())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(discovery.OrgFiles) != 1 {
		t.Fatalf("org files reported = %d, want 1", len(discovery.OrgFiles))
	}
	if discovery.OrgFiles[0].Path != "orgs/org-1/alerts.toml" {
		t.Fatalf("path = %q", discovery.OrgFiles[0].Path)
	}
	if discovery.OrgFiles[0].Kind != KindAlerts {
		t.Fatalf("kind = %q, want alerts", discovery.OrgFiles[0].Kind)
	}
}

// TestDiscoverReportsARejectedOrgFile proves a preview surfaces a refusal rather than
// silently omitting the file.
func TestDiscoverReportsARejectedOrgFile(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/wrong-org/groups.toml": "[[group]]\nname = \"g\"\n",
	})

	svc := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})

	discovery, err := svc.Discover(context.Background(), testRepo())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(discovery.OrgFiles) != 1 {
		t.Fatalf("org files reported = %d, want 1", len(discovery.OrgFiles))
	}
	if discovery.OrgFiles[0].Action != ActionError {
		t.Fatalf("action = %q, want error (no applier for groups)", discovery.OrgFiles[0].Action)
	}
	if !strings.Contains(discovery.OrgFiles[0].Message, "no applier") {
		t.Fatalf("message = %q", discovery.OrgFiles[0].Message)
	}
}

// TestDryRunReportsOrgFiles proves a dry run records the org files too, so its job result
// matches what a real run would do.
func TestDryRunReportsOrgFiles(t *testing.T) {
	root := writeTree(t, map[string]string{"orgs/org-1/alerts.toml": alertsOrgFile})

	applier := &recordingOrgApplier{}
	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, root, newFakeResources(), jobs, &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)
	svc.SetOrgNameLookup(stubOrgNames{name: "org-1"})

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("Sync dry run: %v", err)
	}

	if len(applier.blobs) != 0 {
		t.Fatal("a dry run applied an org file")
	}

	var found bool
	for _, r := range jobs.results {
		if r.Path == "orgs/org-1/alerts.toml" && r.Outcome == domain.ResultApplied {
			found = true
		}
	}
	if !found {
		t.Fatalf("the dry run did not report the org file: %+v", jobs.results)
	}
}

// TestSyncRecordsAnOrgResourceRow proves an applied org file becomes a resource row with a
// kind and no device, so it gets state, a hash and ownership like a device manifest.
func TestSyncRecordsAnOrgResourceRow(t *testing.T) {
	root := writeTree(t, map[string]string{"orgs/org-1/alerts.toml": alertsOrgFile})

	resources := newFakeResources()
	svc := newSyncServiceForTest(t, root, resources, newFakeJobs(), &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, &recordingOrgApplier{})
	svc.SetOrgNameLookup(stubOrgNames{name: "org-1"})

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	rows, err := resources.ListByRepo(context.Background(), "repo-1")
	if err != nil {
		t.Fatalf("ListByRepo: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}

	row := rows[0]
	if row.Kind != KindAlerts {
		t.Fatalf("kind = %q, want alerts", row.Kind)
	}
	if row.DeviceID != "" {
		t.Fatalf("an org resource carries a device: %q", row.DeviceID)
	}
	if row.Path != "orgs/org-1/alerts.toml" {
		t.Fatalf("path = %q", row.Path)
	}
	if row.ContentHash == "" {
		t.Fatal("no content hash, so drift is undetectable")
	}
	if row.State != domain.ResourceApplied {
		t.Fatalf("state = %q, want applied", row.State)
	}
}

// TestSyncRefusesTwoFilesOfOneKind is the guard for the one-file-per-kind rule. Two
// alerts.toml files for one org would apply last-writer-wins.
func TestSyncRefusesTwoFilesOfOneKind(t *testing.T) {
	root := writeTree(t, map[string]string{
		"orgs/org-1/alerts.toml":        alertsOrgFile,
		"orgs/org-1/nested/alerts.toml": alertsOrgFile,
	})

	applier := &recordingOrgApplier{}
	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, root, newFakeResources(), jobs, &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, applier)
	svc.SetOrgNameLookup(stubOrgNames{name: "org-1"})

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(applier.blobs) != 0 {
		t.Fatalf("both duplicate files were applied (%d times)", len(applier.blobs))
	}

	var flagged int
	for _, r := range jobs.results {
		if r.Outcome == domain.ResultFailed && strings.Contains(r.Message, "keep exactly one") {
			flagged++
		}
	}
	if flagged != 2 {
		t.Fatalf("flagged %d files, want both: %+v", flagged, jobs.results)
	}
}

// TestOrgResourceIsNotReportedMissingByTheDevicePlan is the guard for a bug live testing
// found: the device plan iterated every owned row, so an applied org resource was reported
// "missing" on the next sync — as if a device manifest had vanished.
func TestOrgResourceIsNotReportedMissingByTheDevicePlan(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "org-row", RepoID: "repo-1", Kind: KindAlerts,
		Path: "orgs/org-1/alerts.toml", State: domain.ResourceApplied, ContentHash: "h",
	})

	svc := newSyncServiceForTest(t, writeTree(t, map[string]string{}), resources, newFakeJobs(), &fakeDeployer{})

	plan, err := svc.Plan(context.Background(), testRepo(), nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	for _, item := range plan.Items {
		if item.Resource != nil && item.Resource.ID == "org-row" {
			t.Fatalf("the device plan included an org resource as %q", item.Action)
		}
	}
}

// TestDeviceResourceCarriesItsKind proves an applied device manifest is stamped with the
// device kind, which the per-kind ownership index depends on.
func TestDeviceResourceCarriesItsKind(t *testing.T) {
	resources := newFakeResources()
	svc := newSyncServiceForTest(t,
		writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest}),
		resources, newFakeJobs(), &fakeDeployer{})
	svc.now = func() time.Time { return time.Unix(0, 0) }

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, err := resources.GetByDevice(context.Background(), "device-a")
	if err != nil {
		t.Fatalf("GetByDevice: %v", err)
	}
	if got.Kind != domain.KindDevice {
		t.Fatalf("kind = %q, want device", got.Kind)
	}
}

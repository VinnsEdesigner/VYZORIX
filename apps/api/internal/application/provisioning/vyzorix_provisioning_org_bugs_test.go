package provisioning

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/alert"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestDeletedOrgFileIsFlaggedMissing is the guard for a row reading "applied" forever.
//
// An org file deleted from the repository left its row applied, so the console claimed the
// alerts matched the repository when the file was gone. A device manifest already got this
// treatment; the org half was missing.
func TestDeletedOrgFileIsFlaggedMissing(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "org-row", RepoID: "repo-1", Kind: KindAlerts,
		Path: "orgs/org-1/alerts.toml", State: domain.ResourceApplied, ContentHash: "h",
	})

	jobs := newFakeJobs()
	svc := newSyncServiceForTest(t, writeTree(t, map[string]string{}), resources, jobs, &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, &recordingOrgApplier{})

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, err := resources.GetByID(context.Background(), "repo-1", "org-row")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.State != domain.ResourceMissing {
		t.Fatalf("state = %q, want missing for a deleted org file", got.State)
	}
}

// TestSyncResourceRejectsAnOrgResource proves the device endpoint refuses an org id instead
// of returning a job that applied nothing.
func TestSyncResourceRejectsAnOrgResource(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "org-row", RepoID: "repo-1", Kind: KindAlerts,
		Path: "orgs/org-1/alerts.toml", State: domain.ResourceApplied,
	})

	svc := newSyncServiceForTest(t, writeTree(t, map[string]string{}), resources, newFakeJobs(), &fakeDeployer{})

	if _, err := svc.SyncResource(context.Background(), testRepo(), "org-row", SyncOptions{}); !errors.Is(err, ErrNotADeviceResource) {
		t.Fatalf("got %v, want ErrNotADeviceResource", err)
	}
}

// TestAlertApplierDetectsDrift is the guard for §14.4: an object changed outside provisioning
// must be reported as drift. It is the console-edit case — git did not move, but the stored
// rule no longer matches the file.
func TestAlertApplierDetectsDrift(t *testing.T) {
	store := newMemAlerts()
	store.rules["existing"] = &alert.Rule{
		ID: "existing", OrgID: "org-1", Name: "device-offline",
		// The console raised the threshold; the file still says 0.
		Metric: alert.MetricDeviceOfflineCount, Condition: alert.ConditionGt, Threshold: 99,
	}

	a := NewAlertApplier(store)
	a.now = func() time.Time { return time.Unix(0, 0) }

	res, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Drifted != 1 {
		t.Fatalf("drifted = %d, want 1 for a console-edited rule", res.Drifted)
	}
	if !strings.Contains(res.Detail, "had drifted") {
		t.Fatalf("detail does not mention drift: %q", res.Detail)
	}

	// The rewrite must restore the declared value.
	if store.rules["existing"].Threshold != 0 {
		t.Fatalf("threshold = %v, want the file's 0", store.rules["existing"].Threshold)
	}
}

// TestAlertApplierNoDriftWhenMatching proves an unchanged rule is not reported as drift, so
// a plain re-sync stays quiet.
func TestAlertApplierNoDriftWhenMatching(t *testing.T) {
	store := newMemAlerts()
	a := NewAlertApplier(store)
	a.now = func() time.Time { return time.Unix(0, 0) }

	if _, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule)); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	res, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule))
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if res.Drifted != 0 {
		t.Fatalf("drifted = %d on an unchanged rule", res.Drifted)
	}
}

// TestOrgResourceIsMarkedDriftedWhenObjectsChange proves the sync records the drifted state,
// but only when the file itself did not change.
func TestOrgResourceIsMarkedDriftedWhenObjectsChange(t *testing.T) {
	root := writeTree(t, map[string]string{"orgs/org-1/alerts.toml": alertFileOneRule})

	resources := newFakeResources()
	store := newMemAlerts()
	jobs := newFakeJobs()

	svc := newSyncServiceForTest(t, root, resources, jobs, &fakeDeployer{})
	svc.SetOrgApplier(KindAlerts, NewAlertApplier(store))
	svc.SetOrgNameLookup(stubOrgNames{name: "org-1"})
	svc.now = func() time.Time { return time.Unix(0, 0) }

	// First sync applies the file.
	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}

	// Someone edits the rule in the console; the file is untouched.
	for _, r := range store.rules {
		r.Threshold = 42
	}

	if _, err := svc.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("second Sync: %v", err)
	}

	rows, err := resources.ListByRepo(context.Background(), "repo-1")
	if err != nil {
		t.Fatalf("ListByRepo: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].State != domain.ResourceDrifted {
		t.Fatalf("state = %q, want drifted after a console edit", rows[0].State)
	}
}

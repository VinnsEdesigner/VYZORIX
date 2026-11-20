package provisioning

import (
	"context"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestPlanExcludesOutOfScopeFiles proves the scope filter drops a discovered file before
// it is planned. An excluded file must not appear as a resource at all — not as an
// apply, and not as an "unchanged" row the user has to read past.
func TestPlanExcludesOutOfScopeFiles(t *testing.T) {
	svc := NewSyncService(nil, newFakeResources(), nil, nil, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	repo := &domain.Repository{
		ID: "repo-1", OrgID: "org-1", Enabled: true,
		Exclude: []string{"devices/staging/*"},
	}

	manifests := []*Manifest{
		{Path: "devices/staging/vyzorix.toml", DeviceID: "staging", ContentHash: "a"},
		{Path: "devices/prod/vyzorix.toml", DeviceID: "prod", ContentHash: "b"},
	}

	plan, err := svc.Plan(context.Background(), repo, manifests)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if len(plan.Items) != 1 {
		t.Fatalf("got %d items, want 1 (the excluded file must not be planned)", len(plan.Items))
	}
	if plan.Items[0].Manifest.DeviceID != "prod" {
		t.Fatalf("planned the wrong device: %s", plan.Items[0].Manifest.DeviceID)
	}
}

// TestPlanMarksOutOfScopeResourceIgnored proves a resource that falls out of scope becomes
// "ignored", not "missing". Missing invites a revert; ignored says this repository no
// longer claims the file, so the device must be left alone.
func TestPlanMarksOutOfScopeResourceIgnored(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "staging",
		Path: "devices/staging/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 1,
	})

	svc := NewSyncService(nil, resources, nil, nil, &fakeDeployer{}, newFakeJobs(), nil)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	// The repository now excludes the path that produced the resource, and the file is no
	// longer discovered under it either.
	repo := &domain.Repository{
		ID: "repo-1", OrgID: "org-1", Enabled: true,
		Exclude: []string{"devices/staging/*"},
	}

	plan, err := svc.Plan(context.Background(), repo, nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(plan.Items))
	}
	if plan.Items[0].Action != ActionIgnored {
		t.Fatalf("action = %q, want ignored (not missing)", plan.Items[0].Action)
	}
}

// TestApplyIgnoredChangesStateOnly proves an ignored resource has its state changed and
// nothing else: no deploy, no revert, and a job result the user can read.
func TestApplyIgnoredChangesStateOnly(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", DeviceID: "d1",
		Path: "devices/d1/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 4,
	})

	deployer := &fakeDeployer{}
	jobs := newFakeJobs()
	svc := NewSyncService(nil, resources, nil, nil, deployer, jobs, nil)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1", Enabled: true}
	job := &domain.Job{ID: "job-1", RepoID: repo.ID}

	plan := &Plan{Items: []*PlanItem{{
		Resource: &domain.Resource{ID: "res-1", RepoID: repo.ID, DeviceID: "d1",
			Path: "devices/d1/vyzorix.toml"},
		Action:  ActionIgnored,
		Message: "outside scope",
	}}}

	svc.apply(context.Background(), repo, job, plan, SyncOptions{})

	if len(deployer.calls) != 0 {
		t.Fatalf("an ignored resource deployed %d times", len(deployer.calls))
	}
	if len(deployer.rollbacks) != 0 {
		t.Fatalf("an ignored resource was reverted %d times", len(deployer.rollbacks))
	}

	got, err := resources.GetByID(context.Background(), repo.ID, "res-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.State != domain.ResourceIgnored {
		t.Fatalf("state = %q, want ignored", got.State)
	}

	if len(jobs.results) != 1 || jobs.results[0].Outcome != domain.ResultIgnored {
		t.Fatalf("job results = %+v, want one ignored row", jobs.results)
	}
}

// TestIgnoredDoesNotColourTheRunAsWarning proves an ignored resource leaves the job green.
// It is not a failure and not a warning: nothing was applied and nothing is wrong.
func TestIgnoredDoesNotColourTheRunAsWarning(t *testing.T) {
	var tallyObj tally
	tallyObj.record(domain.ResultIgnored)

	if tallyObj.warned != 0 {
		t.Fatal("ignored counted as a warning")
	}
	if tallyObj.skipped != 1 {
		t.Fatalf("ignored counted as %d skipped, want 1", tallyObj.skipped)
	}
	if tallyObj.failed != 0 {
		t.Fatal("ignored counted as a failure")
	}
}

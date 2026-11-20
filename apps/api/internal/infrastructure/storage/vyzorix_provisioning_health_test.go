package storage

import (
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestProvisioningJobDurationRecorded proves a finished job stores its duration, so
// the health view aggregates rather than recomputing it per row.
func TestProvisioningJobDurationRecorded(t *testing.T) {
	store, ctx := newResourceFixture(t)

	started := time.Now().Add(-2 * time.Second)
	job := &provisioning.Job{
		ID: "job-1", RepoID: "repo-1", State: provisioning.SyncWorking, StartedAt: started,
	}
	if err := store.StartJob(ctx, job); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	finished := started.Add(1500 * time.Millisecond)
	job.State = provisioning.SyncSuccess
	job.FinishedAt = &finished
	job.DurationMs = finished.Sub(started).Milliseconds()

	if err := store.FinishJob(ctx, job); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}

	got, err := store.GetJob(ctx, "repo-1", "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.DurationMs != 1500 {
		t.Fatalf("duration = %d ms, want 1500", got.DurationMs)
	}
}

// TestProvisioningHealthAggregates proves the health figures the contract's v2.6
// asks for: average and last duration, the failure streak, and a timeline ordered
// oldest first.
func TestProvisioningHealthAggregates(t *testing.T) {
	store, ctx := newResourceFixture(t)
	base := time.Now().Add(-time.Hour)

	// Oldest first: success, success, failure, failure — so the streak is 2.
	runs := []struct {
		state    provisioning.SyncState
		duration int64
	}{
		{provisioning.SyncSuccess, 1000},
		{provisioning.SyncSuccess, 2000},
		{provisioning.SyncError, 3000},
		{provisioning.SyncError, 4000},
	}

	for i, r := range runs {
		started := base.Add(time.Duration(i) * time.Minute)
		finished := started.Add(time.Duration(r.duration) * time.Millisecond)
		job := &provisioning.Job{
			ID:         "job-" + string(rune('a'+i)),
			RepoID:     "repo-1",
			State:      r.state,
			StartedAt:  started,
			FinishedAt: &finished,
			DurationMs: r.duration,
		}
		if err := store.StartJob(ctx, job); err != nil {
			t.Fatalf("StartJob: %v", err)
		}
		if err := store.FinishJob(ctx, job); err != nil {
			t.Fatalf("FinishJob: %v", err)
		}
	}

	health, err := store.Health(ctx, "repo-1", 20)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}

	if health.Successes != 2 || health.Failures != 2 {
		t.Fatalf("successes/failures = %d/%d, want 2/2", health.Successes, health.Failures)
	}
	if health.FailureStreak != 2 {
		t.Fatalf("failure streak = %d, want 2", health.FailureStreak)
	}
	if health.LastDurationMs != 4000 {
		t.Fatalf("last duration = %d, want 4000 (the newest run)", health.LastDurationMs)
	}
	if health.AverageDurationMs != 2500 {
		t.Fatalf("average duration = %d, want 2500", health.AverageDurationMs)
	}
	if len(health.Timeline) != 4 {
		t.Fatalf("timeline = %d points, want 4", len(health.Timeline))
	}
	if health.Timeline[0].DurationMs != 1000 || health.Timeline[3].DurationMs != 4000 {
		t.Fatalf("timeline is not oldest-first: %+v", health.Timeline)
	}
}

// TestProvisioningHealthExcludesDryAndUnfinished proves the aggregate counts only
// real, completed runs: a dry run applies nothing and a working job has no outcome.
func TestProvisioningHealthExcludesDryAndUnfinished(t *testing.T) {
	store, ctx := newResourceFixture(t)

	now := time.Now()
	finished := now

	// A real run.
	real := &provisioning.Job{ID: "j-real", RepoID: "repo-1", State: provisioning.SyncWorking, StartedAt: now.Add(-time.Minute)}
	if err := store.StartJob(ctx, real); err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	real.State = provisioning.SyncSuccess
	real.FinishedAt = &finished
	real.DurationMs = 500
	if err := store.FinishJob(ctx, real); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}

	// A dry run with a much longer duration: it must not skew the average.
	dry := &provisioning.Job{ID: "j-dry", RepoID: "repo-1", State: provisioning.SyncWorking, DryRun: true, StartedAt: now.Add(-time.Minute)}
	if err := store.StartJob(ctx, dry); err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	dry.State = provisioning.SyncSuccess
	dry.FinishedAt = &finished
	dry.DurationMs = 99_000
	if err := store.FinishJob(ctx, dry); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}

	// A job still working: no outcome, no duration.
	if err := store.StartJob(ctx, &provisioning.Job{
		ID: "j-working", RepoID: "repo-1", State: provisioning.SyncWorking, StartedAt: now,
	}); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	health, err := store.Health(ctx, "repo-1", 20)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}

	if len(health.Timeline) != 1 {
		t.Fatalf("timeline = %d points, want only the one real completed run", len(health.Timeline))
	}
	if health.AverageDurationMs != 500 {
		t.Fatalf("average = %d, want 500 (the dry run must not count)", health.AverageDurationMs)
	}
}

// TestProvisioningHealthEmpty proves a repository with no history reports zeroes
// rather than failing or reporting a bogus instant average.
func TestProvisioningHealthEmpty(t *testing.T) {
	store, ctx := newResourceFixture(t)

	health, err := store.Health(ctx, "repo-1", 20)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.AverageDurationMs != 0 || health.FailureStreak != 0 || len(health.Timeline) != 0 {
		t.Fatalf("empty health = %+v, want all zeroes", health)
	}
}

// TestProvisioningHealthScopedToRepo proves one repository's runs never appear in
// another's health.
func TestProvisioningHealthScopedToRepo(t *testing.T) {
	store, ctx := newResourceFixture(t)

	now := time.Now()
	job := &provisioning.Job{ID: "j-other", RepoID: "repo-2", State: provisioning.SyncWorking, StartedAt: now}
	if err := store.StartJob(ctx, job); err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	job.State = provisioning.SyncSuccess
	job.FinishedAt = &now
	job.DurationMs = 100
	if err := store.FinishJob(ctx, job); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}

	health, err := store.Health(ctx, "repo-1", 20)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if len(health.Timeline) != 0 {
		t.Fatalf("repo-1 saw %d points from repo-2's runs", len(health.Timeline))
	}
}

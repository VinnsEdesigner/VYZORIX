package provisioning

import (
	"context"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// fakeEvents records the events a service emits.
type fakeEvents struct {
	err      error
	recorded []*domain.ProvisioningEvent
}

func (f *fakeEvents) Record(_ context.Context, e *domain.ProvisioningEvent) error {
	if f.err != nil {
		return f.err
	}
	f.recorded = append(f.recorded, e)

	return nil
}

func (f *fakeEvents) ListForOrg(context.Context, string, int) ([]*domain.ProvisioningEvent, error) {
	return f.recorded, nil
}

func (f *fakeEvents) ListForResource(context.Context, string, string, string, int) ([]*domain.ProvisioningEvent, error) {
	return f.recorded, nil
}

// TestSyncRecordsTheTriggeringActor is the requirement: the audit row names whoever
// asked for the sync, so a scheduled run is not mistaken for a person's.
func TestSyncRecordsTheTriggeringActor(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	events := &fakeEvents{}
	s := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.SetAuditor(NewAuditor(events, nil, nil))

	user := domain.Actor{Kind: domain.ActorUser, ID: "u1", Label: "a@b.c"}
	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{Actor: user}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(events.recorded) != 1 {
		t.Fatalf("events = %d, want 1", len(events.recorded))
	}
	got := events.recorded[0]
	if got.Actor.Kind != domain.ActorUser || got.Actor.ID != "u1" {
		t.Fatalf("actor = %+v, want the calling user", got.Actor)
	}
	if got.Type != domain.EventSyncApplied {
		t.Fatalf("type = %s, want sync.applied", got.Type)
	}
	if got.OrgID != "org-1" || got.ResourceID != "repo-1" {
		t.Fatalf("event = %+v, want org-1/repo-1", got)
	}
}

// TestSyncWithoutActorRecordsSystem proves a sync never writes an unattributed event:
// a caller that supplies nothing is recorded as the system, not as blank.
func TestSyncWithoutActorRecordsSystem(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	events := &fakeEvents{}
	s := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.SetAuditor(NewAuditor(events, nil, nil))

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(events.recorded) != 1 {
		t.Fatalf("events = %d, want 1", len(events.recorded))
	}
	if events.recorded[0].Actor.Kind != domain.ActorSystem {
		t.Fatalf("actor = %+v, want the system actor, never blank", events.recorded[0].Actor)
	}
}

// TestSchedulerRecordsSchedulerActor proves the interval worker attributes its runs to
// the scheduler, which is what lets a reader tell an unattended run from a manual one.
func TestSchedulerRecordsSchedulerActor(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	repo := testRepo()
	events := &fakeEvents{}

	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{ID: "conn-1", OrgID: "org-1", Type: domain.ConnectionGitHub}
	conns.tokens["conn-1"] = "token"

	s := NewSyncService(&fakeFetcher{dir: root}, newFakeResources(), conns,
		dueStore{repo: repo}, &fakeDeployer{}, newFakeJobs(), nil)
	s.SetAuditor(NewAuditor(events, nil, nil))

	NewScheduler(dueStore{repo: repo}, s, nil, 0).tickOnce(context.Background())

	if len(events.recorded) != 1 {
		t.Fatalf("events = %d, want 1", len(events.recorded))
	}
	if events.recorded[0].Actor.Kind != domain.ActorScheduler {
		t.Fatalf("actor = %+v, want the scheduler", events.recorded[0].Actor)
	}
}

// TestDryRunRecordsTriggerNotApply proves a dry run is audited as a trigger: it
// changes nothing, so recording it as an apply would misstate what happened.
func TestDryRunRecordsTriggerNotApply(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	events := &fakeEvents{}
	s := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.SetAuditor(NewAuditor(events, nil, nil))

	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(events.recorded) != 1 {
		t.Fatalf("events = %d, want 1", len(events.recorded))
	}
	if events.recorded[0].Type != domain.EventSyncTriggered {
		t.Fatalf("type = %s, want sync.triggered for a dry run", events.recorded[0].Type)
	}
}

// TestFailedSyncIsStillAttributed proves a failure records an error result with the
// actor, rather than leaving the failure unattributed.
func TestFailedSyncIsStillAttributed(t *testing.T) {
	events := &fakeEvents{}
	s := newSyncServiceForTest(t, t.TempDir(), newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.SetAuditor(NewAuditor(events, nil, nil))
	s.fetcher = &fakeFetcher{err: ErrFetchFailed}

	user := domain.Actor{Kind: domain.ActorUser, ID: "u1", Label: "a@b.c"}
	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{Actor: user}); err == nil {
		t.Fatal("a failing fetch must return an error")
	}

	if len(events.recorded) != 1 {
		t.Fatalf("events = %d, want 1", len(events.recorded))
	}
	got := events.recorded[0]
	if got.Result != "error" {
		t.Fatalf("result = %q, want error", got.Result)
	}
	if got.Actor.ID != "u1" {
		t.Fatalf("actor = %+v, want the user even on failure", got.Actor)
	}
}

// TestEventRecordingFailureDoesNotFailTheSync proves a broken audit trail cannot turn
// a successful sync into an error: the sync already happened.
func TestEventRecordingFailureDoesNotFailTheSync(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	events := &fakeEvents{err: context.DeadlineExceeded}
	s := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.SetAuditor(NewAuditor(events, nil, nil))

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("Sync returned an error because the audit write failed: %v", err)
	}
	if res.Job.State != domain.SyncSuccess {
		t.Fatalf("state = %s, want success", res.Job.State)
	}
}

// TestResolveActorDefaultsToSystem pins the fallback that keeps every job attributed.
func TestResolveActorDefaultsToSystem(t *testing.T) {
	cases := []struct {
		name string
		in   domain.Actor
		want domain.ActorKind
	}{
		{"empty", domain.Actor{}, domain.ActorSystem},
		{"kind but no id", domain.Actor{Kind: domain.ActorUser}, domain.ActorSystem},
		{"invalid kind", domain.Actor{Kind: "robot", ID: "x"}, domain.ActorSystem},
		{"valid", domain.Actor{Kind: domain.ActorUser, ID: "u1"}, domain.ActorUser},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveActor(tc.in); got.Kind != tc.want {
				t.Fatalf("kind = %s, want %s", got.Kind, tc.want)
			}
		})
	}

	_ = time.Now
}

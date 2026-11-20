package storage

import (
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestProvisioningEventRecordsActor proves the audit row keeps who caused the action
// and how it was authenticated, which is the whole point of the table.
func TestProvisioningEventRecordsActor(t *testing.T) {
	store, ctx := newResourceFixture(t)

	now := time.Now()
	event := &provisioning.ProvisioningEvent{
		ID:           "ev-1",
		OrgID:        "org-1",
		Actor:        provisioning.Actor{Kind: provisioning.ActorUser, ID: "u1", Label: "a@b.c"},
		Type:         provisioning.EventSyncTriggered,
		ResourceType: "repository",
		ResourceID:   "repo-1",
		Result:       "success",
		Detail:       "applied=2 skipped=0 failed=0",
		CreatedAt:    now,
	}

	if err := store.Record(ctx, event); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := store.ListForOrg(ctx, "org-1", 10)
	if err != nil {
		t.Fatalf("ListForOrg: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events = %d, want 1", len(got))
	}

	e := got[0]
	if e.Actor.Kind != provisioning.ActorUser || e.Actor.ID != "u1" || e.Actor.Label != "a@b.c" {
		t.Fatalf("actor = %+v, want the user recorded verbatim", e.Actor)
	}
	if e.Type != provisioning.EventSyncTriggered || e.ResourceID != "repo-1" {
		t.Fatalf("event = %+v, want the sync trigger for repo-1", e)
	}
}

// TestProvisioningEventDistinguishesSchedulerFromUser is the requirement: a run the
// scheduler started and one a person pressed must be distinguishable afterwards.
func TestProvisioningEventDistinguishesSchedulerFromUser(t *testing.T) {
	store, ctx := newResourceFixture(t)
	now := time.Now()

	actors := []provisioning.Actor{
		provisioning.SchedulerActor(),
		{Kind: provisioning.ActorUser, ID: "u1", Label: "a@b.c"},
		{Kind: provisioning.ActorServiceAccount, ID: "tok-1", Label: "ci"},
	}

	for i, actor := range actors {
		if err := store.Record(ctx, &provisioning.ProvisioningEvent{
			ID:           "ev-" + string(rune('a'+i)),
			OrgID:        "org-1",
			Actor:        actor,
			Type:         provisioning.EventSyncApplied,
			ResourceType: "repository",
			ResourceID:   "repo-1",
			Result:       "success",
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	got, err := store.ListForOrg(ctx, "org-1", 10)
	if err != nil {
		t.Fatalf("ListForOrg: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("events = %d, want 3", len(got))
	}

	// Newest first: service account, user, scheduler.
	kinds := []provisioning.ActorKind{got[0].Actor.Kind, got[1].Actor.Kind, got[2].Actor.Kind}
	want := []provisioning.ActorKind{
		provisioning.ActorServiceAccount, provisioning.ActorUser, provisioning.ActorScheduler,
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("actor kinds = %v, want %v (newest first)", kinds, want)
		}
	}
}

// TestProvisioningEventRefusesUnknownActor proves an unattributed change cannot be
// written: that is the failure this table exists to prevent.
func TestProvisioningEventRefusesUnknownActor(t *testing.T) {
	store, ctx := newResourceFixture(t)

	err := store.Record(ctx, &provisioning.ProvisioningEvent{
		ID: "ev-bad", OrgID: "org-1", ResourceType: "repository",
		Type: provisioning.EventSyncApplied, Result: "success",
	})
	if err == nil {
		t.Fatal("an event with no actor was accepted")
	}

	events, listErr := store.ListForOrg(ctx, "org-1", 10)
	if listErr != nil {
		t.Fatalf("ListForOrg: %v", listErr)
	}
	if len(events) != 0 {
		t.Fatalf("an unattributed event was stored: %+v", events)
	}
}

// TestProvisioningEventsScopedToOrg proves one organisation never sees another's
// audit trail.
func TestProvisioningEventsScopedToOrg(t *testing.T) {
	store, ctx := newResourceFixture(t)

	if err := store.Record(ctx, &provisioning.ProvisioningEvent{
		ID: "ev-1", OrgID: "org-1", Actor: provisioning.SchedulerActor(),
		Type: provisioning.EventSyncApplied, ResourceType: "repository",
		ResourceID: "repo-1", Result: "success", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, err := store.ListForOrg(ctx, "org-other", 10)
	if err != nil {
		t.Fatalf("ListForOrg: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("cross-org leak: %d events", len(events))
	}
}

// TestProvisioningEventsByResource proves a repository's history can be read on its
// own, which is what the repository detail view asks for.
func TestProvisioningEventsByResource(t *testing.T) {
	store, ctx := newResourceFixture(t)
	now := time.Now()

	for _, spec := range []struct{ id, repoID string }{
		{"ev-1", "repo-1"}, {"ev-2", "repo-2"},
	} {
		if err := store.Record(ctx, &provisioning.ProvisioningEvent{
			ID: spec.id, OrgID: "org-1", Actor: provisioning.SchedulerActor(),
			Type: provisioning.EventSyncApplied, ResourceType: "repository",
			ResourceID: spec.repoID, Result: "success", CreatedAt: now,
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	events, err := store.ListForResource(ctx, "org-1", "repository", "repo-1", 10)
	if err != nil {
		t.Fatalf("ListForResource: %v", err)
	}
	if len(events) != 1 || events[0].ResourceID != "repo-1" {
		t.Fatalf("events = %+v, want only repo-1's", events)
	}
}

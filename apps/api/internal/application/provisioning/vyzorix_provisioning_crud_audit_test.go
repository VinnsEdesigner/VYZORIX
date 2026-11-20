package provisioning

import (
	"context"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestConnectionCrudIsAudited proves the credential lifecycle records who did it.
// A rotated token is security-relevant, so it must be attributable.
func TestConnectionCrudIsAudited(t *testing.T) {
	events := &fakeEvents{}

	conns := newFakeConnectionStore()
	svc := NewConnectionService(conns, nil, nil)
	svc.SetAuditor(NewAuditor(events, nil, nil))

	user := domain.Actor{Kind: domain.ActorUser, ID: "u1", Label: "a@b.c"}
	ctx := context.Background()

	created, err := svc.Create(ctx, "org-1", ConnectionInput{
		Actor: user, Name: "gh", Type: domain.ConnectionGitHub, Host: "github.com", Token: "tok",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(ctx, "org-1", created.ID, ConnectionInput{
		Actor: user, Token: "rotated",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := svc.Delete(ctx, "org-1", created.ID, user); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(events.recorded) != 3 {
		t.Fatalf("events = %d, want create/update/delete", len(events.recorded))
	}

	wantTypes := []domain.EventType{
		domain.EventConnectionCreated,
		domain.EventConnectionUpdated,
		domain.EventConnectionDeleted,
	}
	for i, want := range wantTypes {
		if events.recorded[i].Type != want {
			t.Fatalf("event %d = %s, want %s", i, events.recorded[i].Type, want)
		}
		if events.recorded[i].Actor.ID != "u1" {
			t.Fatalf("event %d actor = %+v, want the user", i, events.recorded[i].Actor)
		}
		if events.recorded[i].ResourceType != "connection" {
			t.Fatalf("event %d resource = %q, want connection", i, events.recorded[i].ResourceType)
		}
	}
}

// TestConnectionAuditNeverRecordsTheToken is the security guard: the audit row says a
// credential was supplied, never what it was.
func TestConnectionAuditNeverRecordsTheToken(t *testing.T) {
	events := &fakeEvents{}

	conns := newFakeConnectionStore()
	svc := NewConnectionService(conns, nil, nil)
	svc.SetAuditor(NewAuditor(events, nil, nil))

	const secret = "ghp_SUPER_SECRET_TOKEN_VALUE"

	if _, err := svc.Create(context.Background(), "org-1", ConnectionInput{
		Actor: domain.SystemActor(), Name: "gh", Type: domain.ConnectionGitHub, Token: secret,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, e := range events.recorded {
		if containsSubstring(e.Detail, secret) {
			t.Fatalf("the token leaked into an audit detail: %q", e.Detail)
		}
	}
}

// containsSubstring reports whether sub appears in s.
func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}

// TestRepositoryCrudIsAudited proves the repository lifecycle is attributable too.
func TestRepositoryCrudIsAudited(t *testing.T) {
	events := &fakeEvents{}

	repos := newFakeRepositoryStore()
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	conns.tokens["conn-1"] = "token"

	svc := NewRepositoryService(repos, conns, nil, nil)
	svc.SetAuditor(NewAuditor(events, nil, nil))

	actor := domain.Actor{Kind: domain.ActorServiceAccount, ID: "tok-1", Label: "ci"}
	ctx := context.Background()

	repo, err := svc.Create(ctx, "org-1", RepositoryInput{
		Actor: actor, Name: "fleet", URL: "https://github.com/acme/fleet.git", ConnectionID: "conn-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(ctx, "org-1", repo.ID, RepositoryInput{Actor: actor, Title: "Fleet"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := svc.Delete(ctx, "org-1", repo.ID, true, actor); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(events.recorded) != 3 {
		t.Fatalf("events = %d, want create/update/delete", len(events.recorded))
	}

	wantTypes := []domain.EventType{
		domain.EventRepositoryCreated,
		domain.EventRepositoryUpdated,
		domain.EventRepositoryDeleted,
	}
	for i, want := range wantTypes {
		if events.recorded[i].Type != want {
			t.Fatalf("event %d = %s, want %s", i, events.recorded[i].Type, want)
		}
		if events.recorded[i].Actor.Kind != domain.ActorServiceAccount {
			t.Fatalf("event %d actor = %+v, want the service account", i, events.recorded[i].Actor)
		}
	}
}

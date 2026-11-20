package provisioning

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// TestUpdateConnectionTypePropagatesToRepositories is the guard for provider-type drift.
//
// A repository stores a copy of its connection's type and uses it to pick the provider
// API and git username. Changing the connection from github to gitlab left every
// repository on the old type, so it authenticated with the wrong username and asked the
// wrong provider's API — a failure that looked like a broken credential.
func TestUpdateConnectionTypePropagatesToRepositories(t *testing.T) {
	store := newFakeConnectionStore()
	store.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	store.tokens["conn-1"] = "tok"

	svc := NewConnectionService(store, nil, nil)

	typ := domain.ConnectionGitLab
	if _, err := svc.Update(context.Background(), "org-1", "conn-1", ConnectionInput{
		Type:  typ,
		Token: redaction.NewSecret("newtok"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := store.typeUpdates["conn-1"]; got != string(domain.ConnectionGitLab) {
		t.Fatalf("repositories were re-stamped with %q, want gitlab", got)
	}
}

// TestUpdateConnectionWithoutTypeChangeDoesNotPropagate proves the propagation only runs
// when the type actually changed, so a rename does not issue a needless table-wide update.
func TestUpdateConnectionWithoutTypeChangeDoesNotPropagate(t *testing.T) {
	store := newFakeConnectionStore()
	store.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	store.tokens["conn-1"] = "tok"

	svc := NewConnectionService(store, nil, nil)

	if _, err := svc.Update(context.Background(), "org-1", "conn-1", ConnectionInput{Name: "renamed"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(store.typeUpdates) != 0 {
		t.Fatalf("a rename re-stamped repositories: %v", store.typeUpdates)
	}
}

// TestGeneratedBranchIsUniqueAndRefSafe is the guard for a lost commit.
//
// The branch name used only a second-resolution timestamp, so two edits in the same
// second produced the same branch and the second "checkout -B" reset the branch under
// the first commit. The slug also had to be git-ref-safe, since a repository name is
// free text and git rejects "~^:?*[\" and spaces in a ref.
func TestGeneratedBranchIsUniqueAndRefSafe(t *testing.T) {
	at := time.Unix(1700000000, 0)

	first := generatedBranch("My Fleet: prod*", at)
	second := generatedBranch("My Fleet: prod*", at)

	if first == second {
		t.Fatalf("two edits at the same instant produced the same branch: %q", first)
	}

	for _, name := range []string{first, second} {
		if strings.ContainsAny(name, " ~^:?*[\\") {
			t.Fatalf("branch %q contains a character git rejects in a ref", name)
		}
		if !strings.HasPrefix(name, "vyzorix/edit-") {
			t.Fatalf("branch %q does not carry the expected prefix", name)
		}
	}
}

// TestRefSlugNeverReturnsEmpty proves a name made entirely of rejected characters still
// produces a usable ref segment rather than "vyzorix/edit--<ts>".
func TestRefSlugNeverReturnsEmpty(t *testing.T) {
	for _, raw := range []string{"", "!!!", "   ", "///"} {
		if got := refSlug(raw); got == "" {
			t.Fatalf("refSlug(%q) returned empty", raw)
		}
	}
}

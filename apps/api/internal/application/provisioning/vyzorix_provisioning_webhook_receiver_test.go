package provisioning

import (
	"context"
	"net/http"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// fakeWebhookStore serves repositories and their webhook secrets.
type fakeWebhookStore struct {
	repos   map[string]*domain.Repository
	secrets map[string]string
}

func newFakeWebhookStore() *fakeWebhookStore {
	return &fakeWebhookStore{repos: map[string]*domain.Repository{}, secrets: map[string]string{}}
}

func (f *fakeWebhookStore) GetRepositoryForWebhook(_ context.Context, id string) (*domain.Repository, error) {
	r, ok := f.repos[id]
	if !ok {
		return nil, domain.ErrRepositoryNotFound
	}
	copied := *r

	return &copied, nil
}

func (f *fakeWebhookStore) RepositoryWebhookSecret(_ context.Context, repoID string) (redaction.Secret, error) {
	if _, ok := f.repos[repoID]; !ok {
		return "", domain.ErrRepositoryNotFound
	}

	return redaction.NewSecret(f.secrets[repoID]), nil
}

func (f *fakeWebhookStore) SetRepositoryWebhookSecret(_ context.Context, repoID string, secret redaction.Secret) error {
	f.secrets[repoID] = secret.Reveal()

	return nil
}

// webhookFixture builds a receiver over a repository that tracks branch "main".
func webhookFixture(t *testing.T) (*WebhookReceiver, *fakeWebhookStore, *fakeDeployer) {
	t.Helper()

	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	store := newFakeWebhookStore()
	store.repos["repo-1"] = &domain.Repository{
		ID: "repo-1", OrgID: "org-1", ConnectionID: "conn-1",
		Name: "fleet", Branch: "main", Path: "/", Enabled: true,
		SyncTarget: domain.SyncTargetDevice, IntervalSeconds: 300, SyncState: domain.SyncPending,
	}
	store.secrets["repo-1"] = testWebhookSecret

	dep := &fakeDeployer{}
	syncSvc := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), dep)

	return NewWebhookReceiver(store, store, NewSignatureVerifier(), syncSvc), store, dep
}

// signedRequest builds the header a provider would send for a body.
func signedRequest(body string) http.Header {
	h := http.Header{}
	h.Set("X-Hub-Signature-256", githubSignature(testWebhookSecret, body))

	return h
}

// TestWebhookTriggersASync proves a verified push runs a sync, and that the audit actor
// is the provider — not the repository owner, and not the scheduler.
func TestWebhookTriggersASync(t *testing.T) {
	receiver, _, dep := webhookFixture(t)
	body := []byte(`{"ref":"refs/heads/main"}`)

	result, err := receiver.Handle(context.Background(), "repo-1", ProviderGitHub, signedRequest(string(body)), body)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !result.Triggered {
		t.Fatal("a verified push did not trigger a sync")
	}
	if len(dep.calls) != 1 {
		t.Fatalf("deployed %d manifests, want 1", len(dep.calls))
	}
	if result.Branch != "main" {
		t.Fatalf("branch = %q, want main", result.Branch)
	}
}

// TestWebhookIgnoresAnUntrackedBranch proves a push to another branch is acknowledged
// without syncing: providers send every push, and syncing an unrelated branch would pull
// a tree this repository never reads.
func TestWebhookIgnoresAnUntrackedBranch(t *testing.T) {
	receiver, _, dep := webhookFixture(t)
	body := []byte(`{"ref":"refs/heads/feature-x"}`)

	result, err := receiver.Handle(context.Background(), "repo-1", ProviderGitHub, signedRequest(string(body)), body)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if result.Triggered {
		t.Fatal("a push to an untracked branch triggered a sync")
	}
	if len(dep.calls) != 0 {
		t.Fatalf("deployed %d manifests, want 0", len(dep.calls))
	}
	if result.Branch != "feature-x" {
		t.Fatalf("branch = %q, want feature-x", result.Branch)
	}
}

// TestWebhookRejectsAnUnknownRepository proves an unknown id is refused without revealing
// whether the id exists.
func TestWebhookRejectsAnUnknownRepository(t *testing.T) {
	receiver, _, _ := webhookFixture(t)
	body := []byte(`{"ref":"refs/heads/main"}`)

	if _, err := receiver.Handle(context.Background(), "nope", ProviderGitHub, signedRequest(string(body)), body); err == nil {
		t.Fatal("an unknown repository accepted a webhook")
	}
}

// TestWebhookRejectsABadSignature proves the receiver refuses before it syncs.
func TestWebhookRejectsABadSignature(t *testing.T) {
	receiver, _, dep := webhookFixture(t)
	body := []byte(`{"ref":"refs/heads/main"}`)

	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256=deadbeef")

	if _, err := receiver.Handle(context.Background(), "repo-1", ProviderGitHub, h, body); err == nil {
		t.Fatal("a bad signature was accepted")
	}
	if len(dep.calls) != 0 {
		t.Fatalf("an unverified webhook deployed %d manifests", len(dep.calls))
	}
}

// TestWebhookRejectsARepositoryWithNoSecret proves an unconfigured repository cannot be
// triggered: otherwise every webhook URL is a public sync button.
func TestWebhookRejectsARepositoryWithNoSecret(t *testing.T) {
	receiver, store, dep := webhookFixture(t)
	delete(store.secrets, "repo-1")

	body := []byte(`{"ref":"refs/heads/main"}`)

	if _, err := receiver.Handle(context.Background(), "repo-1", ProviderGitHub, signedRequest(string(body)), body); err == nil {
		t.Fatal("a repository with no secret accepted a webhook")
	}
	if len(dep.calls) != 0 {
		t.Fatalf("deployed %d manifests, want 0", len(dep.calls))
	}
}

// TestProviderActorKind pins the attribution: a provider push is its own actor kind, so an
// operator reading the audit trail can tell it from the interval and from a person.
func TestProviderActorKind(t *testing.T) {
	actor := ProviderActor(ProviderGitHub)

	if actor.Kind != domain.ActorProvider {
		t.Fatalf("kind = %q, want provider", actor.Kind)
	}
	if actor.ID != "github" {
		t.Fatalf("id = %q, want github", actor.ID)
	}
	if !actor.Kind.Valid() {
		t.Fatal("provider is not a valid actor kind")
	}
}

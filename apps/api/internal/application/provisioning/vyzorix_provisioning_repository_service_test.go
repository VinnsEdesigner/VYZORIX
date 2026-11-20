package provisioning

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// TestParseRepositoryPath pins the URL forms a self-hoster actually pastes. A
// wrong parse sends the provider API to the wrong project, which reads as "branch
// not found" for a repository that is fine.
func TestParseRepositoryPath(t *testing.T) {
	cases := []struct {
		raw         string
		owner, repo string
		ok          bool
	}{
		{"https://github.com/acme/fleet.git", "acme", "fleet", true},
		{"https://github.com/acme/fleet", "acme", "fleet", true},
		{"https://github.com/acme/fleet/", "acme", "fleet", true},
		{"git@github.com:acme/fleet.git", "acme", "fleet", true},
		{"ssh://git@github.com/acme/fleet.git", "acme", "fleet", true},
		{"https://gitlab.com/group/sub/project.git", "sub", "project", true},
		{"https://github.com/onlyowner", "", "", false},
		{"", "", "", false},
		{"not a url", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			owner, repo, ok := parseRepositoryPath(tc.raw)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if owner != tc.owner || repo != tc.repo {
				t.Fatalf("got %s/%s, want %s/%s", owner, repo, tc.owner, tc.repo)
			}
		})
	}
}

// fakeRepositoryStore is an in-memory RepositoryStore.
type fakeRepositoryStore struct {
	repos          map[string]*domain.Repository
	webhookSecrets map[string]string
	// duplicateErr, when set, is returned by the next SaveRepository. It models the
	// UNIQUE(org_id, name) violation the real schema raises.
	duplicateErr error
}

func newFakeRepositoryStore() *fakeRepositoryStore {
	return &fakeRepositoryStore{repos: map[string]*domain.Repository{}}
}

func (f *fakeRepositoryStore) SaveRepository(_ context.Context, r *domain.Repository) error {
	if f.duplicateErr != nil {
		return f.duplicateErr
	}

	copied := *r
	f.repos[r.ID] = &copied

	return nil
}

func (f *fakeRepositoryStore) GetRepository(_ context.Context, orgID, id string) (*domain.Repository, error) {
	r, ok := f.repos[id]
	if !ok || r.OrgID != orgID {
		return nil, domain.ErrRepositoryNotFound
	}
	copied := *r

	return &copied, nil
}

func (f *fakeRepositoryStore) ListRepositories(_ context.Context, orgID string) ([]*domain.Repository, error) {
	var out []*domain.Repository
	for _, r := range f.repos {
		if r.OrgID == orgID {
			copied := *r
			out = append(out, &copied)
		}
	}

	return out, nil
}

func (f *fakeRepositoryStore) DeleteRepository(_ context.Context, orgID, id string, _ bool) error {
	r, ok := f.repos[id]
	if !ok || r.OrgID != orgID {
		return domain.ErrRepositoryNotFound
	}
	delete(f.repos, id)

	return nil
}

func (f *fakeRepositoryStore) SetRepositoryWebhookSecret(_ context.Context, repoID string, secret redaction.Secret) error {
	if f.webhookSecrets == nil {
		f.webhookSecrets = map[string]string{}
	}
	f.webhookSecrets[repoID] = secret.Reveal()

	return nil
}

func (f *fakeRepositoryStore) UpdateSyncStatus(_ context.Context, repoID string, state domain.SyncState, lastError string, lastSyncAt time.Time) error {
	r, ok := f.repos[repoID]
	if !ok {
		return domain.ErrRepositoryNotFound
	}
	r.SyncState = state
	r.LastError = lastError
	r.LastSyncAt = &lastSyncAt

	return nil
}

func (f *fakeRepositoryStore) ListDueForSync(_ context.Context, _ time.Time) ([]*domain.Repository, error) {
	return nil, nil
}

// fakeVerifier records whether it was asked, and can be made to fail.
type fakeVerifier struct {
	err   error
	calls int
}

func (v *fakeVerifier) Verify(context.Context, string, string, domain.ConnectionType, redaction.Secret) error {
	v.calls++

	return v.err
}

func newRepoServiceForTest(t *testing.T, verifier RemoteVerifier) (*RepositoryService, *fakeRepositoryStore, *fakeConnectionStore) {
	t.Helper()

	repos := newFakeRepositoryStore()
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh",
		Type: domain.ConnectionGitHub, Status: domain.ConnectionReady,
	}
	conns.tokens["conn-1"] = "token"

	return NewRepositoryService(repos, conns, verifier, nil), repos, conns
}

func validRepoInput() RepositoryInput {
	return RepositoryInput{
		Name:         "fleet",
		URL:          "https://github.com/acme/fleet.git",
		Branch:       "main",
		ConnectionID: "conn-1",
	}
}

func TestRepositoryService_CreateRejectsInvalidInput(t *testing.T) {
	svc, _, _ := newRepoServiceForTest(t, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		in   RepositoryInput
	}{
		{"no name", RepositoryInput{URL: "https://x/y.git", ConnectionID: "conn-1"}},
		{"no url", RepositoryInput{Name: "r", ConnectionID: "conn-1"}},
		{"no connection", RepositoryInput{Name: "r", URL: "https://x/y.git"}},
		{"plain path", RepositoryInput{Name: "r", URL: "/local/dir", ConnectionID: "conn-1"}},
		{"interval too short", RepositoryInput{Name: "r", URL: "https://x/y.git", ConnectionID: "conn-1", IntervalSeconds: 5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, "org-1", tc.in); !errors.Is(err, ErrInvalidRepository) {
				t.Fatalf("got %v, want ErrInvalidRepository", err)
			}
		})
	}
}

// TestRepositoryService_UnreachableRemoteIsNotStored is the contract's rule: the
// remote is verified BEFORE the row is written, so a typo is a form error rather
// than a permanently red repository.
func TestRepositoryService_UnreachableRemoteIsNotStored(t *testing.T) {
	verifier := &fakeVerifier{err: ErrRemoteUnreachable}
	svc, repos, _ := newRepoServiceForTest(t, verifier)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "org-1", validRepoInput()); !errors.Is(err, ErrRemoteUnreachable) {
		t.Fatalf("got %v, want ErrRemoteUnreachable", err)
	}
	if len(repos.repos) != 0 {
		t.Fatal("an unverifiable repository must not be stored")
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier called %d times, want 1", verifier.calls)
	}
}

func TestRepositoryService_CreateAppliesDefaults(t *testing.T) {
	svc, _, _ := newRepoServiceForTest(t, &fakeVerifier{})

	repo, err := svc.Create(context.Background(), "org-1", validRepoInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.Path != "/" {
		t.Errorf("path = %q, want /", repo.Path)
	}
	if repo.SyncTarget != domain.SyncTargetDevice {
		t.Errorf("sync target = %q, want device", repo.SyncTarget)
	}
	if repo.IntervalSeconds != DefaultSyncIntervalSeconds {
		t.Errorf("interval = %d, want %d", repo.IntervalSeconds, DefaultSyncIntervalSeconds)
	}
	if repo.SyncState != domain.SyncPending {
		t.Errorf("sync state = %q, want pending", repo.SyncState)
	}
}

// TestRepositoryService_RenameDoesNotReverify pins that editing a display title
// does not re-probe the provider: a transient network blip must not block a
// cosmetic edit.
func TestRepositoryService_RenameDoesNotReverify(t *testing.T) {
	verifier := &fakeVerifier{}
	svc, _, _ := newRepoServiceForTest(t, verifier)
	ctx := context.Background()

	repo, err := svc.Create(ctx, "org-1", validRepoInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(ctx, "org-1", repo.ID, RepositoryInput{Title: "Fleet configs"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if verifier.calls != 1 {
		t.Fatalf("a title edit re-verified the remote (%d calls)", verifier.calls)
	}
}

// TestRepositoryService_BranchChangeReverifies pins the opposite: changing the
// branch is exactly the edit that can break a repository, so it is checked.
func TestRepositoryService_BranchChangeReverifies(t *testing.T) {
	verifier := &fakeVerifier{}
	svc, _, _ := newRepoServiceForTest(t, verifier)
	ctx := context.Background()

	repo, err := svc.Create(ctx, "org-1", validRepoInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(ctx, "org-1", repo.ID, RepositoryInput{Branch: "release"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if verifier.calls != 2 {
		t.Fatalf("a branch change must re-verify (calls = %d, want 2)", verifier.calls)
	}
}

// TestRepositoryService_DisableMarksSyncDisabled pins that turning sync off moves
// the state to disabled, so the UI does not show a stale "pending".
func TestRepositoryService_DisableMarksSyncDisabled(t *testing.T) {
	svc, _, _ := newRepoServiceForTest(t, &fakeVerifier{})
	ctx := context.Background()

	repo, err := svc.Create(ctx, "org-1", validRepoInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	off := false
	updated, err := svc.Update(ctx, "org-1", repo.ID, RepositoryInput{Enabled: &off})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.SyncState != domain.SyncDisabled {
		t.Fatalf("sync state = %q, want disabled", updated.SyncState)
	}

	on := true
	reEnabled, err := svc.Update(ctx, "org-1", repo.ID, RepositoryInput{Enabled: &on})
	if err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	if reEnabled.SyncState != domain.SyncPending {
		t.Fatalf("sync state = %q, want pending after re-enable", reEnabled.SyncState)
	}
}

func TestRepositoryService_ScopedToOrg(t *testing.T) {
	svc, _, _ := newRepoServiceForTest(t, &fakeVerifier{})
	ctx := context.Background()

	repo, err := svc.Create(ctx, "org-1", validRepoInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Get(ctx, "org-2", repo.ID); !errors.Is(err, domain.ErrRepositoryNotFound) {
		t.Fatalf("got %v, want ErrRepositoryNotFound for a foreign org", err)
	}
}

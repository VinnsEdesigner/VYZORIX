package provisioning

import (
	"context"
	"errors"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// fakeConnectionStore is an in-memory ConnectionStore. It exists so the service's
// rules can be tested without a database; the storage layer has its own tests.
type fakeConnectionStore struct {
	connections map[string]*domain.Connection
	tokens      map[string]string
	typeUpdates map[string]string
	inUse       int
}

func newFakeConnectionStore() *fakeConnectionStore {
	return &fakeConnectionStore{
		connections: map[string]*domain.Connection{},
		tokens:      map[string]string{},
	}
}

func (f *fakeConnectionStore) SaveConnection(_ context.Context, c *domain.Connection) error {
	copied := *c
	f.connections[c.ID] = &copied

	return nil
}

func (f *fakeConnectionStore) SaveConnectionWithToken(_ context.Context, c *domain.Connection, token redaction.Secret) error {
	copied := *c
	f.connections[c.ID] = &copied
	if !token.IsZero() {
		f.tokens[c.ID] = token.Reveal()
	}

	return nil
}

func (f *fakeConnectionStore) GetConnection(_ context.Context, orgID, id string) (*domain.Connection, error) {
	c, ok := f.connections[id]
	if !ok || c.OrgID != orgID {
		return nil, domain.ErrConnectionNotFound
	}

	copied := *c

	return &copied, nil
}

func (f *fakeConnectionStore) ListConnections(_ context.Context, orgID string) ([]*domain.Connection, error) {
	var out []*domain.Connection
	for _, c := range f.connections {
		if c.OrgID == orgID {
			copied := *c
			out = append(out, &copied)
		}
	}

	return out, nil
}

func (f *fakeConnectionStore) DeleteConnection(_ context.Context, orgID, id string) error {
	c, ok := f.connections[id]
	if !ok || c.OrgID != orgID {
		return domain.ErrConnectionNotFound
	}
	delete(f.connections, id)

	return nil
}

func (f *fakeConnectionStore) CountRepositoriesUsing(context.Context, string, string) (int, error) {
	return f.inUse, nil
}

func (f *fakeConnectionStore) ConnectionToken(_ context.Context, orgID, id string) (redaction.Secret, error) {
	// Mirror the storage's org scoping, so a test cannot accidentally rely on an
	// unscoped decrypt.
	if c, ok := f.connections[id]; ok && orgID != "" && c.OrgID != orgID {
		return "", domain.ErrConnectionNotFound
	}

	return redaction.NewSecret(f.tokens[id]), nil
}

// UpdateRepositoriesTypeForConnection records the propagation call so a test can assert
// a provider-type change reaches the repositories that use the connection.
func (f *fakeConnectionStore) UpdateRepositoriesTypeForConnection(_ context.Context, connectionID, typ string) error {
	if f.typeUpdates == nil {
		f.typeUpdates = map[string]string{}
	}
	f.typeUpdates[connectionID] = typ

	return nil
}

// fakeChecker reports a fixed result.
type fakeChecker struct {
	err   error
	calls int
}

func (c *fakeChecker) Check(context.Context, string, domain.ConnectionType, redaction.Secret) error {
	c.calls++

	return c.err
}

func TestConnectionService_CreateRejectsInvalidInput(t *testing.T) {
	svc := NewConnectionService(newFakeConnectionStore(), nil, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		in   ConnectionInput
	}{
		{"no token", ConnectionInput{Name: "x", Type: domain.ConnectionGitHub}},
		{"unknown provider", ConnectionInput{Name: "x", Type: "svn", Token: "t"}},
		{"bad host", ConnectionInput{Name: "x", Type: domain.ConnectionGitHub, Token: "t", Host: "not a host"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, "org-1", tc.in); !errors.Is(err, ErrInvalidConnection) {
				t.Fatalf("got %v, want ErrInvalidConnection", err)
			}
		})
	}
}

func TestConnectionService_CreateNormalisesHost(t *testing.T) {
	svc := NewConnectionService(newFakeConnectionStore(), nil, nil)

	c, err := svc.Create(context.Background(), "org-1", ConnectionInput{
		Name:  "gh",
		Type:  domain.ConnectionGitHub,
		Host:  "https://github.com/",
		Token: "t",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Host != "github.com" {
		t.Fatalf("host = %q, want github.com", c.Host)
	}
}

// TestConnectionService_FailedCheckLeavesVisibleRow pins the choice to store
// before checking: a bad token must produce a row the user can see and fix, not a
// silent nothing.
func TestConnectionService_FailedCheckLeavesVisibleRow(t *testing.T) {
	store := newFakeConnectionStore()
	checker := &fakeChecker{err: ErrConnectionUnreachable}
	svc := NewConnectionService(store, checker, nil)
	ctx := context.Background()

	c, err := svc.Create(ctx, "org-1", ConnectionInput{
		Name: "gh", Type: domain.ConnectionGitHub, Token: "bad",
	})
	if err != nil {
		t.Fatalf("Create must not fail on a failed check: %v", err)
	}
	if c.Status != domain.ConnectionError {
		t.Fatalf("status = %q, want error", c.Status)
	}
	if c.LastError == "" {
		t.Fatal("a failed check must record why")
	}
	if _, ok := store.connections[c.ID]; !ok {
		t.Fatal("the connection must be stored even when the check fails")
	}
}

func TestConnectionService_DeleteRefusesWhileInUse(t *testing.T) {
	store := newFakeConnectionStore()
	store.inUse = 2
	svc := NewConnectionService(store, nil, nil)
	ctx := context.Background()

	c, err := svc.Create(ctx, "org-1", ConnectionInput{
		Name: "gh", Type: domain.ConnectionGitHub, Token: "t",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(ctx, "org-1", c.ID, domain.SystemActor()); !errors.Is(err, domain.ErrConnectionInUse) {
		t.Fatalf("got %v, want ErrConnectionInUse", err)
	}
}

// TestConnectionService_RenameDoesNotRecheck pins that a rename does not probe the
// host again: the credential did not change, so a transient network blip must not
// turn a healthy connection red.
func TestConnectionService_RenameDoesNotRecheck(t *testing.T) {
	store := newFakeConnectionStore()
	checker := &fakeChecker{}
	svc := NewConnectionService(store, checker, nil)
	ctx := context.Background()

	c, err := svc.Create(ctx, "org-1", ConnectionInput{
		Name: "gh", Type: domain.ConnectionGitHub, Token: "t",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if checker.calls != 1 {
		t.Fatalf("create made %d checks, want 1", checker.calls)
	}

	if _, err := svc.Update(ctx, "org-1", c.ID, ConnectionInput{Name: "renamed"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if checker.calls != 1 {
		t.Fatalf("a rename re-checked the host (%d calls)", checker.calls)
	}
}

// TestConnectionService_TokenRotationRechecks pins the opposite: a new token is
// checked, because a rotated credential is exactly the thing that might be wrong.
func TestConnectionService_TokenRotationRechecks(t *testing.T) {
	store := newFakeConnectionStore()
	checker := &fakeChecker{}
	svc := NewConnectionService(store, checker, nil)
	ctx := context.Background()

	c, err := svc.Create(ctx, "org-1", ConnectionInput{
		Name: "gh", Type: domain.ConnectionGitHub, Token: "t",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(ctx, "org-1", c.ID, ConnectionInput{Token: "new"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if checker.calls != 2 {
		t.Fatalf("a token rotation must re-check (calls = %d, want 2)", checker.calls)
	}
	if store.tokens[c.ID] != "new" {
		t.Fatalf("token = %q, want the rotated value", store.tokens[c.ID])
	}
}

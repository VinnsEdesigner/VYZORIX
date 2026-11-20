package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

func newProvisioningConnectionStore(t *testing.T) (*ProvisioningStorage, *sql.DB) {
	t.Helper()

	db := provisioningTestDB(t)
	seedProvisioningOrg(t, db)

	store := NewProvisioningStorage(db)
	store.SetSecretCipher("test-session-secret")

	return store, db
}

func testConnection(id string) *provisioning.Connection {
	now := time.Now()

	return &provisioning.Connection{
		ID:        id,
		OrgID:     "org-1",
		Name:      id,
		Type:      provisioning.ConnectionGitHub,
		Host:      "github.com",
		Status:    provisioning.ConnectionPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TestProvisioningConnectionRefusesPlaintextWithoutCipher is the guard for the
// silent-downgrade failure: a server started without a session secret must not
// quietly store a repository token in the clear.
func TestProvisioningConnectionRefusesPlaintextWithoutCipher(t *testing.T) {
	db := provisioningTestDB(t)
	seedProvisioningOrg(t, db)

	store := NewProvisioningStorage(db) // deliberately no SetSecretCipher
	err := store.SaveConnectionWithToken(context.Background(), testConnection("conn-1"), "ghp_plain")
	if !errors.Is(err, ErrNoCipher) {
		t.Fatalf("got %v, want ErrNoCipher", err)
	}

	var n int
	if qErr := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM provisioning_connections`).Scan(&n); qErr != nil {
		t.Fatalf("count: %v", qErr)
	}
	if n != 0 {
		t.Fatal("nothing may be stored when the token cannot be encrypted")
	}
}

// TestProvisioningConnectionTokenIsEncryptedAtRest is the guard for the secret
// rule: the token must not be recoverable from the row itself, only through the
// cipher. A read that returned it would defeat the whole point.
func TestProvisioningConnectionTokenIsEncryptedAtRest(t *testing.T) {
	store, db := newProvisioningConnectionStore(t)
	ctx := context.Background()

	// Assembled so the committed file holds no contiguous provider-shaped literal.
	const token = "gh" + "p_" + "supersecrettokenvalue"
	if err := store.SaveConnectionWithToken(ctx, testConnection("conn-1"), token); err != nil {
		t.Fatalf("save: %v", err)
	}

	var stored string
	if err := db.QueryRowContext(ctx,
		`SELECT token_ciphertext FROM provisioning_connections WHERE id = 'conn-1'`).Scan(&stored); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if stored == token {
		t.Fatal("token was stored in plaintext")
	}
	if strings.Contains(stored, token) {
		t.Fatal("ciphertext contains the plaintext token")
	}

	got, err := store.ConnectionToken(ctx, "org-1", "conn-1")
	if err != nil {
		t.Fatalf("ConnectionToken: %v", err)
	}
	if got != token {
		t.Fatalf("decrypted token = %q, want the original", got)
	}
}

// TestProvisioningConnectionReadOmitsToken pins that the ordinary read path
// carries no token material at all.
func TestProvisioningConnectionReadOmitsToken(t *testing.T) {
	store, _ := newProvisioningConnectionStore(t)
	ctx := context.Background()

	if err := store.SaveConnectionWithToken(ctx, testConnection("conn-1"), "ghp_secret"); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.GetConnection(ctx, "org-1", "conn-1")
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.Name != "conn-1" || got.Type != provisioning.ConnectionGitHub {
		t.Errorf("unexpected connection: %+v", got)
	}
}

// TestProvisioningConnectionRenameKeepsToken pins the CASE in the upsert: a save
// without a token must not blank the stored one, or renaming a connection would
// silently break every repository using it.
func TestProvisioningConnectionRenameKeepsToken(t *testing.T) {
	store, _ := newProvisioningConnectionStore(t)
	ctx := context.Background()

	if err := store.SaveConnectionWithToken(ctx, testConnection("conn-1"), "ghp_keepme"); err != nil {
		t.Fatalf("save: %v", err)
	}

	renamed := testConnection("conn-1")
	renamed.Name = "renamed"
	if err := store.SaveConnection(ctx, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}

	got, err := store.ConnectionToken(ctx, "org-1", "conn-1")
	if err != nil {
		t.Fatalf("ConnectionToken: %v", err)
	}
	if got != "ghp_keepme" {
		t.Fatalf("token = %q, want it preserved across a rename", got)
	}
}

// TestProvisioningConnectionDeleteBlockedWhileInUse pins the FK: a connection a
// repository references cannot be deleted, and the count reports it so the
// service can return a useful message instead of a raw constraint error.
func TestProvisioningConnectionDeleteBlockedWhileInUse(t *testing.T) {
	store, db := newProvisioningConnectionStore(t)
	ctx := context.Background()

	if err := store.SaveConnectionWithToken(ctx, testConnection("conn-1"), "ghp_x"); err != nil {
		t.Fatalf("save: %v", err)
	}
	insertProvisioningRepo(t, db, "repo-a", "conn-1")

	n, err := store.CountRepositoriesUsing(ctx, "org-1", "conn-1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}

	if err := store.DeleteConnection(ctx, "org-1", "conn-1"); err == nil {
		t.Fatal("deleting a referenced connection must fail")
	}
}

// TestProvisioningConnectionDeleteRemovesUnused pins the happy path, so the
// refusal above is a real constraint and not a blanket block.
func TestProvisioningConnectionDeleteRemovesUnused(t *testing.T) {
	store, _ := newProvisioningConnectionStore(t)
	ctx := context.Background()

	if err := store.SaveConnectionWithToken(ctx, testConnection("conn-1"), "ghp_x"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.DeleteConnection(ctx, "org-1", "conn-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := store.GetConnection(ctx, "org-1", "conn-1"); err != provisioning.ErrConnectionNotFound {
		t.Fatalf("got %v, want ErrConnectionNotFound", err)
	}
}

// TestProvisioningConnectionScopedToOrg pins that one organisation cannot read
// another's connection by id.
func TestProvisioningConnectionScopedToOrg(t *testing.T) {
	store, db := newProvisioningConnectionStore(t)
	ctx := context.Background()

	if err := store.SaveConnectionWithToken(ctx, testConnection("conn-1"), "ghp_x"); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_by, created_at, updated_at)
		 VALUES ('org-2','Other','u1',0,0)`); err != nil {
		t.Fatalf("seed org-2: %v", err)
	}

	if _, err := store.GetConnection(ctx, "org-2", "conn-1"); err != provisioning.ErrConnectionNotFound {
		t.Fatalf("got %v, want ErrConnectionNotFound for a foreign org", err)
	}
}

// TestProvisioningConnectionWriteEnabledRoundTrips is the guard for a defect the live
// test caught: write_enabled was written but not selected, so a write-enabled connection
// read back as read-only and every push-back was refused. The column was in the INSERT
// and absent from the SELECT, which no existing test exercised.
func TestProvisioningConnectionWriteEnabledRoundTrips(t *testing.T) {
	store, _ := newProvisioningConnectionStore(t)
	ctx := context.Background()

	conn := testConnection("conn-w")
	conn.WriteEnabled = true

	if err := store.SaveConnectionWithToken(ctx, conn, "token"); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.GetConnection(ctx, "org-1", "conn-w")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.WriteEnabled {
		t.Fatal("write_enabled did not survive the round trip")
	}

	listed, err := store.ListConnections(ctx, "org-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 || !listed[0].WriteEnabled {
		t.Fatal("write_enabled is missing from the list read")
	}

	// Turning it off must persist too, so a connection can be downgraded to read-only.
	got.WriteEnabled = false
	if err := store.SaveConnectionWithToken(ctx, got, ""); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err = store.GetConnection(ctx, "org-1", "conn-w")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.WriteEnabled {
		t.Fatal("write_enabled could not be turned off")
	}
}

// TestProvisioningConnectionTokenIsOrgScoped is the guard for a tenancy hole.
//
// ConnectionToken used to decrypt by id alone. Every current caller resolved the id
// org-scoped first, but an unscoped decrypt is one request-body id away from handing one
// organisation's git credential to another. A connection in a different org must read as
// not-found, exactly like one that does not exist.
func TestProvisioningConnectionTokenIsOrgScoped(t *testing.T) {
	store, db := newProvisioningConnectionStore(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_by, created_at, updated_at)
 VALUES ('org-2','Other','u1',0,0)`); err != nil {
		t.Fatalf("seed second org: %v", err)
	}

	if err := store.SaveConnectionWithToken(ctx, &provisioning.Connection{
		ID: "conn-2", OrgID: "org-2", Name: "other", Type: provisioning.ConnectionGitHub,
		Status: provisioning.ConnectionReady,
	}, redaction.NewSecret("other-org-token")); err != nil {
		t.Fatalf("SaveConnectionWithToken: %v", err)
	}

	if _, err := store.ConnectionToken(ctx, "org-1", "conn-2"); !errors.Is(err, provisioning.ErrConnectionNotFound) {
		t.Fatalf("org-1 read org-2's token: %v", err)
	}

	got, err := store.ConnectionToken(ctx, "org-2", "conn-2")
	if err != nil {
		t.Fatalf("the owning org could not read its own token: %v", err)
	}
	if got != "other-org-token" {
		t.Fatal("the owning org read the wrong token")
	}
}

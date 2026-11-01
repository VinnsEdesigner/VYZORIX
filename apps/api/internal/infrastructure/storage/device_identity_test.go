package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// newTestDeviceDB opens an in-memory SQLite database with the real devices
// schema, including the columns migrations 74 and 75 add. The repository is
// exercised end to end rather than mocked, because the bug this guards against —
// a column written but never selected — is invisible to a mock.
func newTestDeviceDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	stmts := []string{
		`CREATE TABLE devices (
			id TEXT PRIMARY KEY,
			firebase_install_id TEXT,
			fcm_token TEXT,
			app_version TEXT,
			device_class TEXT,
			command_secret_hash TEXT,
			online INTEGER NOT NULL DEFAULT 0,
			registered_at INTEGER NOT NULL DEFAULT 0,
			last_seen INTEGER NOT NULL DEFAULT 0,
			operator_id TEXT,
			organization_id TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			device_name TEXT,
			manufacturer TEXT,
			model TEXT,
			os_version TEXT,
			security_patch TEXT,
			deregistered_at INTEGER,
			deletion_scheduled_at INTEGER,
			fcm_token_refreshed_at INTEGER,
			tags TEXT,
			tier INTEGER NOT NULL DEFAULT 1,
			public_key TEXT,
			pairing_code TEXT,
			pairing_expires_at INTEGER)`,
	}

	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	return db
}

// TestDeviceRoundTripKeepsIdentityAndTier is the guard on the write-only bug:
// tier, public_key, pairing_code and pairing_expires_at must survive a write and
// a read. Each was previously written by INSERT/UPDATE without being selected.
func TestDeviceRoundTripKeepsIdentityAndTier(t *testing.T) {
	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	expires := time.Now().Add(10 * time.Minute).UnixMilli()
	d := device.NewDevice("dev-1", "fid-1")
	d.PublicKey = "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n"
	d.PairingCode = "K7QM3X2P"
	d.PairingExpiresAt = &expires
	d.Tier = device.TierRooted

	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByID(ctx, "dev-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if got.PublicKey != d.PublicKey {
		t.Errorf("public key did not round trip: %q", got.PublicKey)
	}
	if got.PairingCode != d.PairingCode {
		t.Errorf("pairing code did not round trip: %q", got.PairingCode)
	}
	if got.PairingExpiresAt == nil || *got.PairingExpiresAt != expires {
		t.Errorf("pairing expiry did not round trip: %v", got.PairingExpiresAt)
	}
	if got.Tier != device.TierRooted {
		t.Errorf("tier did not round trip: got %d, want %d", got.Tier, device.TierRooted)
	}
}

// TestFindByPairingCodeOnlyMatchesLiveCodes proves an expired code is not
// returned, so an operator cannot claim a device whose notification went stale.
func TestFindByPairingCodeOnlyMatchesLiveCodes(t *testing.T) {
	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	live := time.Now().Add(time.Minute).UnixMilli()
	d := device.NewDevice("dev-live", "")
	d.PairingCode = "LIVE1234"
	d.PairingExpiresAt = &live
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("create: %v", err)
	}

	expired := time.Now().Add(-time.Minute).UnixMilli()
	old := device.NewDevice("dev-old", "")
	old.PairingCode = "OLD12345"
	old.PairingExpiresAt = &expired
	if err := repo.Create(ctx, old); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := repo.FindByPairingCode(ctx, "LIVE1234"); err != nil {
		t.Errorf("a live code must resolve: %v", err)
	}
	if _, err := repo.FindByPairingCode(ctx, "OLD12345"); err == nil {
		t.Error("an expired code must not resolve")
	}
	if _, err := repo.FindByPairingCode(ctx, "NOPE0000"); err == nil {
		t.Error("an unknown code must not resolve")
	}
}

// TestClearPairingCodeConsumesIt proves a claimed code cannot be claimed twice.
func TestClearPairingCodeConsumesIt(t *testing.T) {
	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	expires := time.Now().Add(time.Minute).UnixMilli()
	d := device.NewDevice("dev-1", "")
	d.PairingCode = "ONCE1234"
	d.PairingExpiresAt = &expires
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.ClearPairingCode(ctx, "dev-1"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	if _, err := repo.FindByPairingCode(ctx, "ONCE1234"); err == nil {
		t.Fatal("a consumed code must not resolve")
	}

	got, err := repo.FindByID(ctx, "dev-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.PairingCode != "" || got.PairingExpiresAt != nil {
		t.Errorf("clearing must leave no code: %q %v", got.PairingCode, got.PairingExpiresAt)
	}
}

// TestUpdateKeepsIdentityAndTier proves the UPDATE statement carries the new
// columns too, since INSERT and UPDATE are separate statements that can drift.
func TestUpdateKeepsIdentityAndTier(t *testing.T) {
	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	d := device.NewDevice("dev-1", "")
	d.Tier = device.TierUnrooted
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("create: %v", err)
	}

	d.Tier = device.TierAdvanced
	d.PublicKey = "key-after-update"
	if err := repo.Update(ctx, d); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindByID(ctx, "dev-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Tier != device.TierAdvanced {
		t.Errorf("tier not updated: got %d", got.Tier)
	}
	if got.PublicKey != "key-after-update" {
		t.Errorf("public key not updated: %q", got.PublicKey)
	}
}

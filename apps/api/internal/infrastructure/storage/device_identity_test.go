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
			build_id TEXT,
			api_level INTEGER NOT NULL DEFAULT 0,
			supported_abis TEXT,
			ram_bytes INTEGER NOT NULL DEFAULT 0,
			storage_bytes INTEGER NOT NULL DEFAULT 0,
			deregistered_at INTEGER,
			deletion_scheduled_at INTEGER,
			fcm_token_refreshed_at INTEGER,
			tags TEXT,
			tier INTEGER NOT NULL DEFAULT 1,
			public_key TEXT,
			secret_public_key TEXT,
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

// TestDeviceReadHandlesAnIsoTimestamp is the guard on the read path that made every
// enroll fail with a 500. devices.created_at/updated_at are declared TIMESTAMP and
// the driver stores whatever is written; a row written with a time.Time comes back
// as an ISO8601 string, and scanning that straight into time.Time fails at runtime
// with "unsupported Scan, storing driver.Value type string into type *time.Time".
//
// The app writes Unix millis, but a row can hold either, so the read must handle
// both. This writes the string form directly and asserts the read succeeds.
func TestDeviceReadHandlesAnIsoTimestamp(t *testing.T) {
	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	if _, err := db.Exec(
		`INSERT INTO devices (id, firebase_install_id, fcm_token, app_version, device_class,
		                      public_key, online, registered_at, last_seen, created_at, updated_at)
		 VALUES ('iso-1', '', '', '', '', '', 0, ?, ?, ?, ?)`,
		time.Now().UnixMilli(), time.Now().UnixMilli(),
		"2026-11-04 01:08:22.455800251+00:00", "2026-11-04 01:08:22.455800311+00:00",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := repo.FindByID(ctx, "iso-1")
	if err != nil {
		t.Fatalf("a device row with an ISO8601 timestamp must be readable: %v", err)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at must be parsed, not left zero")
	}
}

// TestDeviceReadHandlesNullAppVersionAndDeviceClass guards the deploy read path:
// app_version and device_class are nullable, and a device that has not yet
// reported them is the normal state. Scanning NULL into a plain string fails the
// whole read, which surfaced as a 500 on every deploy to a fresh device.
func TestDeviceReadHandlesNullAppVersionAndDeviceClass(t *testing.T) {
	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	if _, err := db.Exec(
		`INSERT INTO devices (id, firebase_install_id, organization_id, online,
                                      registered_at, last_seen, created_at, updated_at)
                 VALUES ('fresh-1', 'fid-1', 'org-1', 0, ?, ?, ?, ?)`,
		time.Now().UnixMilli(), time.Now().UnixMilli(),
		time.Now().UnixMilli(), time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := repo.FindByID(ctx, "fresh-1")
	if err != nil {
		t.Fatalf("a device with NULL app_version/device_class must be readable: %v", err)
	}
	if got.AppVersion != "" || got.DeviceClass != "" {
		t.Errorf("null fields must read as empty, got app_version=%q device_class=%q",
			got.AppVersion, got.DeviceClass)
	}

	list, _, err := repo.List(ctx, "org-1", 10, 0)
	if err != nil {
		t.Fatalf("List must tolerate NULL app_version/device_class: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 device, got %d", len(list))
	}
}

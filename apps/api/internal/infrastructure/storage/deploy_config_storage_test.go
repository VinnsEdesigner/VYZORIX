package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
)

// newDeployConfigDB opens an in-memory SQLite database with the real schema,
// created by the migration itself rather than a hand-copied DDL. A copy would
// drift from the migration and pass while the real table failed.
//
// The devices table is created too: the org-scoped read joins against it, so a
// fixture without devices would fail every read rather than test it.
func newDeployConfigDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := migrateCreateDeviceDeployConfig(tx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		organization_id TEXT
	)`); err != nil {
		t.Fatalf("create devices: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return db
}

// seedDeployDevice records a device in an organisation, so an org-scoped read has a
// row to join against.
func seedDeployDevice(t *testing.T, db *sql.DB, deviceID, orgID string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO devices (id, organization_id) VALUES (?, ?)`, deviceID, orgID); err != nil {
		t.Fatalf("seed device %s: %v", deviceID, err)
	}
}

// deployFixture is the common setup: a store, a device in org-1, and a context.
func deployFixture(t *testing.T) (*DeployConfigRepository, *sql.DB, context.Context) {
	t.Helper()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")

	return NewDeployConfigRepository(db), db, context.Background()
}

func record(deviceID string, version int, document string) *deployconfig.Record {
	return &deployconfig.Record{
		DeviceID:  deviceID,
		Kind:      deployconfig.KindCurrent,
		Version:   version,
		Document:  document,
		CreatedAt: time.UnixMilli(int64(version) * 1000),
	}
}

// TestSaveCurrentRoundTrip covers the write and read of the current config.
func TestSaveCurrentRoundTrip(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 1, "doc-v1")); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Current(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if got.Version != 1 || got.Document != "doc-v1" {
		t.Fatalf("round trip changed the record: %+v", got)
	}
}

// TestMissingConfigIsNotFound proves the sentinel, so a caller can tell "none
// stored" from a real error.
func TestMissingConfigIsNotFound(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)

	if _, err := repo.Current(context.Background(), "org-1", "absent"); !errors.Is(err, deployconfig.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestSaveCurrentDemotesPrevious is the retention rule: the previous current
// becomes the last-good.
func TestSaveCurrentDemotesPrevious(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 1, "doc-v1")); err != nil {
		t.Fatalf("save v1: %v", err)
	}

	if err := repo.SaveCurrent(ctx, record("d_1", 2, "doc-v2")); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	lastGood, err := repo.LastGood(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("last good: %v", err)
	}

	if lastGood.Version != 1 || lastGood.Document != "doc-v1" {
		t.Fatalf("the previous current must become the last-good, got %+v", lastGood)
	}

	current, err := repo.Current(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if current.Version != 2 {
		t.Fatalf("current must be the newest, got %d", current.Version)
	}
}

// TestRetentionStaysAtTwo proves a third save does not leave an older last-good
// behind: retention is a count, not a history.
func TestRetentionStaysAtTwo(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	for v := 1; v <= 3; v++ {
		if err := repo.SaveCurrent(ctx, record("d_1", v, fmt.Sprintf("doc-v%d", v))); err != nil {
			t.Fatalf("save v%d: %v", v, err)
		}
	}

	lastGood, err := repo.LastGood(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("last good: %v", err)
	}

	if lastGood.Version != 2 {
		t.Fatalf("after three saves the last-good must be v2, got v%d", lastGood.Version)
	}

	var count int

	if err := repo.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_deploy_configs WHERE device_id = ?`, "d_1").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != deployconfig.Retention {
		t.Fatalf("retention must be %d rows, got %d", deployconfig.Retention, count)
	}
}

// TestMarkAppliedRecordsTheVersion covers the held-vs-target read the console
// needs.
func TestMarkAppliedRecordsTheVersion(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 7, "doc-v7")); err != nil {
		t.Fatalf("save: %v", err)
	}

	at := time.UnixMilli(1_700_000_000_000)

	if err := repo.MarkApplied(ctx, "d_1", 7, at); err != nil {
		t.Fatalf("mark applied: %v", err)
	}

	got, err := repo.Current(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if !got.AppliedAt.Equal(at) {
		t.Fatalf("applied_at not recorded: got %v want %v", got.AppliedAt, at)
	}
}

// TestMarkAppliedIgnoresAStaleVersion proves a device reporting an older version
// does not mark the newer config as applied.
func TestMarkAppliedIgnoresAStaleVersion(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 7, "doc-v7")); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := repo.MarkApplied(ctx, "d_1", 6, time.UnixMilli(1)); err != nil {
		t.Fatalf("mark applied: %v", err)
	}

	got, err := repo.Current(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if !got.AppliedAt.IsZero() {
		t.Fatalf("a stale version must not mark the current applied, got %v", got.AppliedAt)
	}
}

// TestDevicesAreIsolated proves one device's config never leaks into another's.
func TestDevicesAreIsolated(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 1, "doc-a")); err != nil {
		t.Fatalf("save d_1: %v", err)
	}

	if err := repo.SaveCurrent(ctx, record("d_2", 1, "doc-b")); err != nil {
		t.Fatalf("save d_2: %v", err)
	}

	first, err := repo.Current(ctx, "org-1", "d_1")
	if err != nil {
		t.Fatalf("current d_1: %v", err)
	}

	if first.Document != "doc-a" {
		t.Fatalf("d_1 must not read d_2's config, got %q", first.Document)
	}
}

// TestSaveCurrentRejectsTheWrongKind proves the store cannot be used to write a
// last-good directly, which would bypass the demotion.
func TestSaveCurrentRejectsTheWrongKind(t *testing.T) {
	t.Parallel()

	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	repo := NewDeployConfigRepository(db)

	rec := record("d_1", 1, "doc")
	rec.Kind = deployconfig.KindLastGood

	if err := repo.SaveCurrent(context.Background(), rec); err == nil {
		t.Fatal("SaveCurrent must refuse a last-good record")
	}
}

// TestCurrentIsOrgScoped is the guard for the cross-tenant read.
//
// Current used to look a device up by id alone, so a caller holding another tenant's
// device id could read that device's signed config — which names its apps and their
// secret keys. The read now joins against devices on organization_id, so a device in
// another organisation reads as ErrNotFound, exactly like one that does not exist.
func TestCurrentIsOrgScoped(t *testing.T) {
	db := newDeployConfigDB(t)
	seedDeployDevice(t, db, "d_1", "org-1")
	seedDeployDevice(t, db, "d_other", "org-2")

	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_other", 1, "other-tenant-doc")); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The owning org reads it.
	if _, err := repo.Current(ctx, "org-2", "d_other"); err != nil {
		t.Fatalf("the owning org could not read its own config: %v", err)
	}

	// Another org must not, and must not be able to tell it exists.
	if _, err := repo.Current(ctx, "org-1", "d_other"); !errors.Is(err, deployconfig.ErrNotFound) {
		t.Fatalf("org-1 read org-2's config: %v", err)
	}
	if _, err := repo.LastGood(ctx, "org-1", "d_other"); !errors.Is(err, deployconfig.ErrNotFound) {
		t.Fatalf("org-1 read org-2's last-good config: %v", err)
	}

	// The device's own read is not org-scoped: the device signed the request, so it is
	// the authenticated principal and can only reach its own row.
	if _, err := repo.CurrentForDevice(ctx, "d_other"); err != nil {
		t.Fatalf("the device could not read its own config: %v", err)
	}
}

// TestCurrentForDeviceStillReadsOwnRow proves the device-own read was not broken by the
// org scoping, and that it does not require a devices row to exist — the device path
// authenticates by signature, not by a join.
func TestCurrentForDeviceStillReadsOwnRow(t *testing.T) {
	db := newDeployConfigDB(t)
	repo := NewDeployConfigRepository(db)
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_unjoined", 3, "doc")); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.CurrentForDevice(ctx, "d_unjoined")
	if err != nil {
		t.Fatalf("CurrentForDevice: %v", err)
	}
	if got.Version != 3 {
		t.Fatalf("version = %d, want 3", got.Version)
	}
}

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

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return db
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 1, "doc-v1")); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Current(ctx, "d_1")
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))

	if _, err := repo.Current(context.Background(), "absent"); !errors.Is(err, deployconfig.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestSaveCurrentDemotesPrevious is the retention rule: the previous current
// becomes the last-good.
func TestSaveCurrentDemotesPrevious(t *testing.T) {
	t.Parallel()

	repo := NewDeployConfigRepository(newDeployConfigDB(t))
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 1, "doc-v1")); err != nil {
		t.Fatalf("save v1: %v", err)
	}

	if err := repo.SaveCurrent(ctx, record("d_1", 2, "doc-v2")); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	lastGood, err := repo.LastGood(ctx, "d_1")
	if err != nil {
		t.Fatalf("last good: %v", err)
	}

	if lastGood.Version != 1 || lastGood.Document != "doc-v1" {
		t.Fatalf("the previous current must become the last-good, got %+v", lastGood)
	}

	current, err := repo.Current(ctx, "d_1")
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))
	ctx := context.Background()

	for v := 1; v <= 3; v++ {
		if err := repo.SaveCurrent(ctx, record("d_1", v, fmt.Sprintf("doc-v%d", v))); err != nil {
			t.Fatalf("save v%d: %v", v, err)
		}
	}

	lastGood, err := repo.LastGood(ctx, "d_1")
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 7, "doc-v7")); err != nil {
		t.Fatalf("save: %v", err)
	}

	at := time.UnixMilli(1_700_000_000_000)

	if err := repo.MarkApplied(ctx, "d_1", 7, at); err != nil {
		t.Fatalf("mark applied: %v", err)
	}

	got, err := repo.Current(ctx, "d_1")
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 7, "doc-v7")); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := repo.MarkApplied(ctx, "d_1", 6, time.UnixMilli(1)); err != nil {
		t.Fatalf("mark applied: %v", err)
	}

	got, err := repo.Current(ctx, "d_1")
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))
	ctx := context.Background()

	if err := repo.SaveCurrent(ctx, record("d_1", 1, "doc-a")); err != nil {
		t.Fatalf("save d_1: %v", err)
	}

	if err := repo.SaveCurrent(ctx, record("d_2", 1, "doc-b")); err != nil {
		t.Fatalf("save d_2: %v", err)
	}

	first, err := repo.Current(ctx, "d_1")
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

	repo := NewDeployConfigRepository(newDeployConfigDB(t))

	rec := record("d_1", 1, "doc")
	rec.Kind = deployconfig.KindLastGood

	if err := repo.SaveCurrent(context.Background(), rec); err == nil {
		t.Fatal("SaveCurrent must refuse a last-good record")
	}
}

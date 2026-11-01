package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestMigration75AddsDeviceIdentityColumns runs the real migration registry, not
// a hand-written schema. The device identity tests build their own table, so
// without this a migration that never applied would go unnoticed: the repository
// would work in tests and fail on a real database.
func TestMigration75AddsDeviceIdentityColumns(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := runMigrations(db); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	want := map[string]bool{
		"tier":               false,
		"public_key":         false,
		"pairing_code":       false,
		"pairing_expires_at": false,
	}

	rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(devices)`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid          int
			name, ctype  string
			notNull, pk  int
			defaultValue sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}

	for col, found := range want {
		if !found {
			t.Errorf("migration 75 did not add column %q", col)
		}
	}
}

// TestMigrationsAreIdempotent proves a second run is a no-op, since the
// migration adds columns with ALTER TABLE and a re-run must not fail.
func TestMigrationsAreIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := runMigrations(db); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := runMigrations(db); err != nil {
		t.Fatalf("second run: %v", err)
	}
}

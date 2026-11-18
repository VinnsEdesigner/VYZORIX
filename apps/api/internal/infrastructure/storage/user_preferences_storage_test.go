package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestSavePreferencesRawInsertsTimestamps is the regression guard for a
// pre-existing defect: the INSERT named eight columns but the table's
// created_at and updated_at are NOT NULL with no default, so the very first save
// for a user failed with "NOT NULL constraint failed". Every other INSERT in the
// same file supplied both; only this one did not.
//
// The bug was invisible to unit tests that only exercised the update path,
// because ON CONFLICT only triggers once a row exists — and no row could ever be
// created. It surfaced only by calling the real endpoint.
func TestSavePreferencesRawInsertsTimestamps(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, m := range migrations {
		applyTx(t, db, m)
	}

	ctx := context.Background()
	repo := NewUserRepository(db)

	// user_settings has a FK to users, so the user must exist first.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, password_hash, role, mfa_enabled, email_verified, instance_role, created_at, updated_at)
		 VALUES ('u1','a@b.test','A','x','operator',0,1,'none',1,1)`); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// First save: the INSERT path. This is the case that used to fail.
	if err := repo.SavePreferencesRaw(ctx, "u1", `{"theme":"light"}`); err != nil {
		t.Fatalf("first save (insert) failed: %v", err)
	}

	got, err := repo.GetPreferencesRaw(ctx, "u1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got != `{"theme":"light"}` {
		t.Errorf("preferences = %q, want %q", got, `{"theme":"light"}`)
	}

	// Second save: the ON CONFLICT path.
	if err := repo.SavePreferencesRaw(ctx, "u1", `{"theme":"tron"}`); err != nil {
		t.Fatalf("second save (update) failed: %v", err)
	}
	got, err = repo.GetPreferencesRaw(ctx, "u1")
	if err != nil {
		t.Fatalf("read back 2: %v", err)
	}
	if got != `{"theme":"tron"}` {
		t.Errorf("preferences after update = %q, want %q", got, `{"theme":"tron"}`)
	}
}

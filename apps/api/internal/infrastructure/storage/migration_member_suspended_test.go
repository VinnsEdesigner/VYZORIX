package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestMigration99AdmitsSuspendedMembership is the regression guard for the
// membership status CHECK. The domain has always had a `suspended` state, but
// the table was created with CHECK (status IN ('active','removed')), so the
// UPDATE that suspends a member failed on any backend enforcing it. The
// migration must widen the constraint and add suspended_at.
func TestMigration99AdmitsSuspendedMembership(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, m := range migrations {
		if m.Version >= 99 {
			break
		}
		applyTx(t, db, m)
	}

	if columnExists(t, db, "organization_members", "suspended_at") {
		t.Fatal("precondition failed: suspended_at should not exist before migration 99")
	}
	if err := insertMemberRow(db, "active"); err != nil {
		t.Fatalf("seed active membership: %v", err)
	}
	if err := suspendSeedRow(db); err == nil {
		t.Fatal("precondition failed: the old CHECK must reject 'suspended'")
	}

	for _, m := range migrations {
		if m.Version == 99 {
			applyTx(t, db, m)
		}
	}

	if !columnExists(t, db, "organization_members", "suspended_at") {
		t.Fatal("migration 99 did not add organization_members.suspended_at")
	}
	if err := suspendSeedRow(db); err != nil {
		t.Fatalf("migration 99 must admit 'suspended', got %v", err)
	}
	if got := memberStatus(db, t); got != "suspended" {
		t.Fatalf("status = %q, want suspended", got)
	}
}

func insertMemberRow(db *sql.DB, status string) error {
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO organization_members (id, organization_id, user_id, role, joined_at, status)
                 VALUES ('m1', 'org-1', 'u1', 'admin', 1700000000000, ?)`, status)

	return err
}

func suspendSeedRow(db *sql.DB) error {
	_, err := db.ExecContext(context.Background(),
		`UPDATE organization_members SET status = 'suspended' WHERE id = 'm1'`)

	return err
}

func memberStatus(db *sql.DB, t *testing.T) string {
	t.Helper()
	var status string
	if err := db.QueryRowContext(context.Background(),
		`SELECT status FROM organization_members WHERE id = 'm1'`).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}

	return status
}

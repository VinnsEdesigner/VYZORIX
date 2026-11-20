package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// seedOrgListingFixture builds a two-tenant database: acme has one active admin
// and one suspended viewer; globex has one active admin. It runs the real
// migration registry, so the query is exercised against the live schema.
func seedOrgListingFixture(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := runMigrations(db); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	stmts := []string{
		`INSERT INTO users (id, email, name, avatar_url, created_at, updated_at, last_login_at)
		 VALUES ('u1', 'ada@acme.io', 'Ada', '/avatars/ada.png', 0, 0, 1700000000000),
		        ('u2', 'bob@acme.io', 'Bob', '', 0, 0, 1700000001000),
		        ('u3', 'cy@globex.io', 'Cy', '', 0, 0, 1700000002000)`,
		`INSERT INTO organizations (id, name, created_by, created_at, updated_at)
		 VALUES ('org-acme', 'Acme', 'u1', 0, 0), ('org-globex', 'Globex', 'u3', 0, 0)`,
		`INSERT INTO organization_members (id, organization_id, user_id, role, joined_at, status)
		 VALUES ('m1', 'org-acme', 'u1', 'admin', 1700000000000, 'active'),
		        ('m2', 'org-acme', 'u2', 'viewer', 1700000000000, 'suspended'),
		        ('m3', 'org-globex', 'u3', 'admin', 1700000000000, 'active')`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	return db
}

// TestListByOrganization_ExcludesOtherTenants is the regression guard for the
// cross-tenant leak: the list was a scan of the users table, so an org
// super_admin saw every tenant's accounts. The listing must be joined on the
// caller's membership, so a row outside the org cannot appear.
func TestListByOrganization_ExcludesOtherTenants(t *testing.T) {
	repo := NewUserRepository(seedOrgListingFixture(t))

	rows, total, err := repo.ListByOrganization(context.Background(), "org-acme", 20, 0, "", "created_at", true, "")
	if err != nil {
		t.Fatalf("ListByOrganization: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("acme rows = %d (total %d), want 2; globex must not leak in", len(rows), total)
	}
	for _, row := range rows {
		if row.ID == "u3" {
			t.Fatal("a globex user appeared in acme's listing")
		}
	}
}

// TestListByOrganization_ReportsMembershipFacts pins that the role and status
// reported are the membership's, and that the account's profile facts are joined
// in so the table needs no second request per row.
func TestListByOrganization_ReportsMembershipFacts(t *testing.T) {
	repo := NewUserRepository(seedOrgListingFixture(t))

	rows, _, err := repo.ListByOrganization(context.Background(), "org-acme", 20, 0, "", "created_at", false, "")
	if err != nil {
		t.Fatalf("ListByOrganization: %v", err)
	}

	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.ID] = true
		switch row.ID {
		case "u1":
			if row.Role != "admin" || row.Status != "active" {
				t.Errorf("u1 role/status = %q/%q, want admin/active", row.Role, row.Status)
			}
			if row.AvatarURL != "/avatars/ada.png" {
				t.Errorf("u1 avatar = %q, want the joined users.avatar_url", row.AvatarURL)
			}
			if row.LastSeenAt == nil {
				t.Error("u1 last seen is nil, want the joined users.last_login_at")
			}
		case "u2":
			if row.Status != "suspended" {
				t.Errorf("u2 status = %q, want suspended (the membership's state)", row.Status)
			}
			if row.MemberID != "m2" {
				t.Errorf("u2 member id = %q, want m2", row.MemberID)
			}
		}
	}
	if !seen["u1"] || !seen["u2"] {
		t.Fatalf("expected both acme members, got %v", seen)
	}
}

// TestListByOrganization_FiltersByStatus pins the two tabs: "active" narrows to
// active memberships, and an empty filter keeps suspended members visible so they
// can be reinstated.
func TestListByOrganization_FiltersByStatus(t *testing.T) {
	repo := NewUserRepository(seedOrgListingFixture(t))

	active, _, err := repo.ListByOrganization(context.Background(), "org-acme", 20, 0, "", "created_at", false, "active")
	if err != nil {
		t.Fatalf("ListByOrganization active: %v", err)
	}
	if len(active) != 1 || active[0].ID != "u1" {
		t.Fatalf("active filter returned %d rows, want only u1", len(active))
	}

	every, _, err := repo.ListByOrganization(context.Background(), "org-acme", 20, 0, "", "created_at", false, "")
	if err != nil {
		t.Fatalf("ListByOrganization all: %v", err)
	}
	if len(every) != 2 {
		t.Fatalf("unfiltered listing returned %d rows, want 2 (suspended included)", len(every))
	}
}

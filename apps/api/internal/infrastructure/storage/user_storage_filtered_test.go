package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

func newUserRepo(t *testing.T) *UserRepository {
	t.Helper()

	s, err := Open(DefaultConfig(filepath.Join(t.TempDir(), "users.db")))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return NewUserRepository(s.DB())
}

func seedUser(t *testing.T, r *UserRepository, id, email, name string) *user.User {
	t.Helper()
	now := time.Now()
	u := &user.User{ID: id, Email: email, Name: name, CreatedAt: now, UpdatedAt: now}
	if err := r.Create(context.Background(), u); err != nil {
		t.Fatalf("Create %s: %v", id, err)
	}
	return u
}

// TestUserRepo_ListFiltered covers the admin list query: search narrows by email
// and name, sort selects the column and direction, and the total reflects the
// filtered set rather than the whole table.
func TestUserRepo_ListFiltered(t *testing.T) {
	r := newUserRepo(t)
	ctx := context.Background()

	seedUser(t, r, "u1", "ada@acme.io", "Ada Lovelace")
	seedUser(t, r, "u2", "grace@globex.io", "Grace Hopper")
	seedUser(t, r, "u3", "ada2@acme.io", "Ada Byron")

	// Unfiltered: everything, newest first.
	all, total, err := r.ListFiltered(ctx, 10, 0, "", "created_at", true)
	if err != nil {
		t.Fatalf("ListFiltered all: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("expected 3 users, got %d (total %d)", len(all), total)
	}

	// Search matches email and name, and total follows the filter.
	byEmail, total, err := r.ListFiltered(ctx, 10, 0, "acme.io", "", true)
	if err != nil {
		t.Fatalf("ListFiltered search: %v", err)
	}
	if total != 2 || len(byEmail) != 2 {
		t.Fatalf("email search: got %d (total %d), want 2", len(byEmail), total)
	}

	byName, _, err := r.ListFiltered(ctx, 10, 0, "hopper", "", true)
	if err != nil {
		t.Fatalf("ListFiltered name search: %v", err)
	}
	if len(byName) != 1 || byName[0].ID != "u2" {
		t.Fatalf("name search: got %v, want [u2]", byName)
	}

	// Sort by email ascending.
	asc, _, err := r.ListFiltered(ctx, 10, 0, "", "email", false)
	if err != nil {
		t.Fatalf("ListFiltered sort: %v", err)
	}
	if asc[0].Email != "ada2@acme.io" {
		t.Errorf("email asc first = %q, want ada2@acme.io", asc[0].Email)
	}

	// An unknown sort column falls back rather than erroring.
	if _, _, err := r.ListFiltered(ctx, 10, 0, "", "'; DROP TABLE users;--", true); err != nil {
		t.Errorf("unknown sort column should fall back, got %v", err)
	}

	// Pagination.
	page2, total, err := r.ListFiltered(ctx, 2, 2, "", "email", false)
	if err != nil {
		t.Fatalf("ListFiltered page2: %v", err)
	}
	if total != 3 || len(page2) != 1 {
		t.Errorf("page2: got %d rows (total %d), want 1 of 3", len(page2), total)
	}
}

// TestUserRepo_SetDisabled verifies the disable timestamp round-trips through
// FindByID and that clearing it re-enables the account.
func TestUserRepo_SetDisabled(t *testing.T) {
	r := newUserRepo(t)
	ctx := context.Background()

	seedUser(t, r, "u1", "ada@acme.io", "Ada Lovelace")

	loaded, err := r.FindByID(ctx, "u1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.Disabled {
		t.Fatal("a fresh user must not be disabled")
	}

	now := time.Now()
	if err := r.SetDisabled(ctx, "u1", &now); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	loaded, _ = r.FindByID(ctx, "u1")
	if !loaded.Disabled {
		t.Error("expected the user to be disabled")
	}

	if err := r.SetDisabled(ctx, "u1", nil); err != nil {
		t.Fatalf("SetDisabled(nil): %v", err)
	}
	loaded, _ = r.FindByID(ctx, "u1")
	if loaded.Disabled {
		t.Error("expected the user to be re-enabled")
	}

	// Unknown id is reported, not silently ignored.
	if err := r.SetDisabled(ctx, "nope", &now); err != user.ErrNotFound {
		t.Errorf("SetDisabled on missing user: %v, want ErrNotFound", err)
	}
}

// TestUserRepo_CreatePersistsAvatar is the regression guard for a defect where
// the INSERT column list and the value list had drifted apart (the value list
// carried avatar_url but the column list did not), so every user insert failed
// with "NOT NULL constraint failed: users.mfa_enabled" — the values were bound
// one position off. Migration 085 rebuilt the table and dropped avatar_url from
// the column list, leaving the two lists out of step.
func TestUserRepo_CreatePersistsAvatar(t *testing.T) {
	r := newUserRepo(t)
	ctx := context.Background()

	now := time.Now()
	u := &user.User{
		ID:        "u1",
		Email:     "Ada@Acme.io",
		Name:      "Ada Lovelace",
		AvatarURL: "/avatars/ada.png",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := r.FindByID(ctx, "u1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.AvatarURL != "/avatars/ada.png" {
		t.Errorf("avatar_url = %q, want /avatars/ada.png", loaded.AvatarURL)
	}
	if loaded.Email != "ada@acme.io" {
		t.Errorf("email = %q, want lower-cased ada@acme.io", loaded.Email)
	}
}

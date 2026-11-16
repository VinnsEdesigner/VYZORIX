package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
	_ "github.com/mattn/go-sqlite3"
)

func setupPrefsTestDB(t *testing.T) (*storage.UserRepository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE user_settings (user_id TEXT PRIMARY KEY, risk_warn INTEGER DEFAULT 70, risk_crit INTEGER DEFAULT 85, thermal_warn INTEGER DEFAULT 45, thermal_crit INTEGER DEFAULT 50, buffer_warn INTEGER DEFAULT 30, buffer_crit INTEGER DEFAULT 15, preferences TEXT DEFAULT '{}')`)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	repo := storage.NewUserRepository(db)
	return repo, db
}

func TestGetPreferences_NoData_ReturnsDefaults(t *testing.T) {
	repo, db := setupPrefsTestDB(t)
	defer db.Close()
	svc := NewClientSettingsService(repo, nil)
	ctx := context.Background()

	prefs, err := svc.GetPreferences(ctx, "op-nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for missing operator, got: %v", err)
	}
	// The theme is deliberately NOT defaulted here: an absent theme means
	// "inherit", so the cascade can fall through to the org or system default.
	// Defaulting it would make every user look like they chose dark.
	if _, exists := prefs["theme"]; exists {
		t.Errorf("theme must be absent when unset, got %v", prefs["theme"])
	}
	if prefs["language"] != "en" {
		t.Errorf("expected default language 'en', got %v", prefs["language"])
	}
}

// fakeOrgTheme is a stub OrgThemeSource for the cascade tests.
type fakeOrgTheme struct {
	theme string
	err   error
}

func (f fakeOrgTheme) OrgTheme(_ context.Context, _ string) (string, error) {
	return f.theme, f.err
}

func TestResolveTheme_Cascade(t *testing.T) {
	tests := []struct {
		name    string
		userSet string // "" = leave unset
		orgID   string
		org     OrgThemeSource
		want    string
	}{
		{name: "user wins over org", userSet: "light", orgID: "org-1", org: fakeOrgTheme{theme: "tron"}, want: "light"},
		{name: "org used when user unset", orgID: "org-1", org: fakeOrgTheme{theme: "tron"}, want: "tron"},
		{name: "system default when both unset", orgID: "org-1", org: fakeOrgTheme{theme: ""}, want: DefaultTheme},
		{name: "no org selected falls to default", want: DefaultTheme},
		{name: "no org source falls to default", orgID: "org-1", want: DefaultTheme},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, db := setupPrefsTestDB(t)
			defer db.Close()
			svc := NewClientSettingsService(repo, tc.org)
			ctx := context.Background()

			if tc.userSet != "" {
				if _, err := svc.UpdatePreferences(ctx, "op-1", map[string]any{"theme": tc.userSet}); err != nil {
					t.Fatalf("seed user theme: %v", err)
				}
			}

			got, err := svc.ResolveTheme(ctx, "op-1", tc.orgID)
			if err != nil {
				t.Fatalf("ResolveTheme: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolveTheme = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveTheme_ClearingUserThemeFallsThroughToOrg(t *testing.T) {
	repo, db := setupPrefsTestDB(t)
	defer db.Close()
	svc := NewClientSettingsService(repo, fakeOrgTheme{theme: "burgundy"})
	ctx := context.Background()

	// Set then clear: an empty string must delete the key, not store "".
	if _, err := svc.UpdatePreferences(ctx, "op-1", map[string]any{"theme": "light"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := svc.UpdatePreferences(ctx, "op-1", map[string]any{"theme": ""}); err != nil {
		t.Fatalf("clear: %v", err)
	}

	got, err := svc.ResolveTheme(ctx, "op-1", "org-1")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	if got != "burgundy" {
		t.Errorf("after clearing the user theme, expected the org default, got %q", got)
	}
}

func TestUpdatePreferences_ThenGet(t *testing.T) {
	repo, db := setupPrefsTestDB(t)
	defer db.Close()
	svc := NewClientSettingsService(repo, nil)
	ctx := context.Background()

	updates := map[string]any{
		"theme":            "light",
		"refresh_interval": 60,
	}
	_, err := svc.UpdatePreferences(ctx, "op-1", updates)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	prefs, err := svc.GetPreferences(ctx, "op-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if prefs["theme"] != "light" {
		t.Errorf("expected theme 'light', got %v", prefs["theme"])
	}
	if prefs["refresh_interval"] != float64(60) {
		t.Errorf("expected refresh_interval float64(60), got %v (%T)", prefs["refresh_interval"], prefs["refresh_interval"])
	}
	if prefs["language"] != "en" {
		t.Errorf("expected default language 'en' to be merged in, got %v", prefs["language"])
	}
}

func TestUpdatePreferences_DeleteCustomKey(t *testing.T) {
	repo, db := setupPrefsTestDB(t)
	defer db.Close()
	svc := NewClientSettingsService(repo, nil)
	ctx := context.Background()

	_, _ = svc.UpdatePreferences(ctx, "op-1", map[string]any{"custom_key": "custom_value"})
	_, err := svc.UpdatePreferences(ctx, "op-1", map[string]any{"custom_key": nil})
	if err != nil {
		t.Fatalf("update with nil: %v", err)
	}

	raw, _ := repo.GetPreferencesRaw(ctx, "op-1")
	if raw == "" {
		t.Fatal("expected raw preferences to be saved")
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := stored["custom_key"]; exists {
		t.Error("custom_key should be absent in stored JSON after nil update")
	}
}

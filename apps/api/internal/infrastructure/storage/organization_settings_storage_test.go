package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// TestOrganizationSettingsRoundTripWeekStart is the guard for the column
// wiring: the repository's SELECT, INSERT, UPDATE and scan each list the columns
// positionally, so a column added to one list and not another (or added in a
// different order) silently reads or writes the wrong field. Only a real
// round-trip through the database catches that.
func TestOrganizationSettingsRoundTripWeekStart(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, m := range migrations {
		applyTx(t, db, m)
	}

	ctx := context.Background()
	repo := NewOrganizationSettingsRepository(db)

	settings := organization.NewOrganizationSettings("org-1")
	settings.WeekStart = "monday"
	settings.Theme = "tron"
	settings.Timezone = "Europe/Berlin"
	settings.DateFormat = "DD/MM/YYYY"

	if err := repo.Create(ctx, settings); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByOrganizationID(ctx, "org-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if got.WeekStart != "monday" {
		t.Errorf("weekStart after create = %q, want %q", got.WeekStart, "monday")
	}
	if got.Theme != "tron" {
		t.Errorf("theme after create = %q, want %q", got.Theme, "tron")
	}

	// The update path must carry week_start too.
	got.WeekStart = "saturday"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	after, err := repo.FindByOrganizationID(ctx, "org-1")
	if err != nil {
		t.Fatalf("find after update: %v", err)
	}
	if after.WeekStart != "saturday" {
		t.Errorf("weekStart after update = %q, want %q", after.WeekStart, "saturday")
	}
	if after.Theme != "tron" {
		t.Errorf("theme after update = %q, want %q", after.Theme, "tron")
	}

	// Clearing must persist as empty, which is what "inherit" means.
	after.WeekStart = ""
	if err := repo.Update(ctx, after); err != nil {
		t.Fatalf("update clear: %v", err)
	}
	cleared, err := repo.FindByOrganizationID(ctx, "org-1")
	if err != nil {
		t.Fatalf("find after clear: %v", err)
	}
	if cleared.WeekStart != "" {
		t.Errorf("weekStart after clear = %q, want empty", cleared.WeekStart)
	}
}

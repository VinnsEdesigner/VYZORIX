package provisioninghandler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// stubStats is a StatsSource with a fixed answer.
type stubStats struct {
	stats *domain.ResourceStats
}

func (s stubStats) StatsByRepo(_ context.Context, _ string) (*domain.ResourceStats, error) {
	return s.stats, nil
}

// TestRepositoryJSONCarriesStats proves the contract's stats[] field reaches the
// wire shape, so a list view can render counts without loading every resource.
func TestRepositoryJSONCarriesStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &domain.Repository{
		ID: "repo-1", Name: "fleet", Branch: "main", Path: "/",
		SyncState: domain.SyncSuccess, Enabled: true,
	}

	stats := domain.ResourceStats{Applied: 4, Missing: 2, Error: 1, Total: 7}

	out := toRepositoryJSON(repo, stats)

	if out.Stats.Total != 7 || out.Stats.Applied != 4 || out.Stats.Missing != 2 || out.Stats.Error != 1 {
		t.Fatalf("stats = %+v, want 4 applied / 2 missing / 1 error / 7 total", out.Stats)
	}

	// The JSON must carry the field even when it is empty: a client reading stats
	// should never have to distinguish "absent" from "zero".
	encoded, err := json.Marshal(toRepositoryJSON(repo, domain.ResourceStats{}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["stats"]; !ok {
		t.Fatalf("stats missing from the wire shape: %s", encoded)
	}
}

// TestStatsForToleratesNilSource proves a missing source degrades to zero counts
// rather than failing the list.
func TestStatsForToleratesNilSource(t *testing.T) {
	h := &RepositoryHandler{}

	got := h.statsFor(context.Background(), "repo-1")
	if got != (domain.ResourceStats{}) {
		t.Fatalf("stats = %+v, want the zero value", got)
	}
}

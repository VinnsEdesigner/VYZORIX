package provisioning

import (
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestSchedulerStatusSeparatesDueFromNext covers a presentation defect: a repository already
// past its interval was also reported as the "next due" time, which rendered as a negative
// "4m ago". A past moment is not a next time — it belongs in DueNow only.
func TestSchedulerStatusSeparatesDueFromNext(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	lastSync := now.Add(-10 * time.Minute)

	repos := []*domain.Repository{
		// Overdue: last synced 10m ago with a 60s interval.
		{Name: "overdue", Enabled: true, IntervalSeconds: 60, LastSyncAt: &lastSync},
		// Due in 5 minutes.
		{Name: "later", Enabled: true, IntervalSeconds: 900, LastSyncAt: &lastSync},
		// Disabled: never due.
		{Name: "off", Enabled: false, IntervalSeconds: 60},
		// Never synced: due immediately.
		{Name: "fresh", Enabled: true, IntervalSeconds: 60},
	}

	state := SchedulerStatus(nil, repos, true, 15*time.Second, now)

	if !state.Running {
		t.Fatal("running should be reported")
	}
	if state.TickSeconds != 15 {
		t.Fatalf("tick = %d, want 15", state.TickSeconds)
	}

	due := map[string]bool{}
	for _, name := range state.DueNow {
		due[name] = true
	}
	if !due["overdue"] || !due["fresh"] {
		t.Fatalf("due now = %v, want overdue and fresh", state.DueNow)
	}
	if due["later"] || due["off"] {
		t.Fatalf("due now = %v, must not include a future or disabled repository", state.DueNow)
	}

	if state.NextDue.IsZero() {
		t.Fatal("next due should be the future repository")
	}
	if !state.NextDue.After(now) {
		t.Fatalf("next due = %v, must be in the future; a past time renders as a negative", state.NextDue)
	}
	if got := state.NextDue.Sub(now); got != 5*time.Minute {
		t.Fatalf("next due in %v, want 5m", got)
	}
}

// A scheduler that is not running must say so rather than reporting a schedule it cannot keep.
func TestSchedulerStatusReportsNotRunning(t *testing.T) {
	state := SchedulerStatus(nil, nil, false, 0, time.Now())

	if state.Running {
		t.Fatal("a nil scheduler must report running=false")
	}
	if state.TickSeconds != int(DefaultSchedulerTick.Seconds()) {
		t.Fatalf("tick = %d, want the default", state.TickSeconds)
	}
}

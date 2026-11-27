package provisioning

import (
	"context"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// SchedulerState is what the console needs to answer "what is the scheduler doing?".
//
// The scheduler runs on its own; without a way to see its state a repository simply changes
// under a user who was never told a timer exists. This is deliberately read-only: the
// schedule itself is per repository (enabled + interval), which is where it belongs, and the
// tick is a server setting an operator cannot change from a browser.
type SchedulerState struct {
	// NextDue is the earliest moment any enabled repository becomes due, zero when none is.
	NextDue time.Time
	// DueNow lists repositories whose interval has elapsed and which the next tick will sync.
	DueNow []string
	// TickSeconds is how often the loop looks for due repositories.
	TickSeconds int
	// Running is false when the server was started with the scheduler disabled.
	Running bool
}

// SchedulerStatus reads the scheduler's state for an organisation.
//
// DueNow is computed with the same rule the scheduler itself uses (enabled, and last sync
// older than the interval, or never synced), so what this reports is what the loop will do
// rather than an approximation of it.
func SchedulerStatus(ctx context.Context, repos []*domain.Repository, running bool, tick time.Duration, now time.Time) SchedulerState {
	state := SchedulerState{Running: running, TickSeconds: int(tick.Seconds())}
	if state.TickSeconds <= 0 {
		state.TickSeconds = int(DefaultSchedulerTick.Seconds())
	}

	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		due := dueAt(repo)
		if !due.After(now) {
			state.DueNow = append(state.DueNow, repo.Name)
			// Already due: it belongs in DueNow, not in "next". Reporting a past moment as
			// the next due time renders as a negative "4m ago", which reads as a bug.
			continue
		}
		if state.NextDue.IsZero() || due.Before(state.NextDue) {
			state.NextDue = due
		}
	}

	return state
}

// dueAt is when a repository becomes due, mirroring ListDueForSync.
func dueAt(repo *domain.Repository) time.Time {
	if repo.LastSyncAt == nil {
		// Never synced: due immediately.
		return time.Time{}
	}

	return repo.LastSyncAt.Add(time.Duration(repo.IntervalSeconds) * time.Second)
}

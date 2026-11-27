package provisioning

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// DefaultSchedulerTick is how often the scheduler looks for due repositories.
//
// It is much shorter than the minimum sync interval (60s) so a repository is
// picked up close to its scheduled time rather than up to a whole interval late.
// The cost of a tick is one indexed query.
const DefaultSchedulerTick = 15 * time.Second

// SchedulerStore is the read the scheduler needs.
type SchedulerStore interface {
	ListDueForSync(ctx context.Context, now time.Time) ([]*domain.Repository, error)
}

// NodeLock is a cross-node lease. The per-repository lock in SyncService is in-process
// only, so on a multi-replica deployment every replica's scheduler would sync the same
// repository at the same time: the deploys would be applied twice and the two runs would
// race each other's job counters. The lease makes exactly one node the scheduler.
//
// It is an interface so the loop can be tested without a database; a nil lock means
// single-node, where the in-process lock is sufficient.
type NodeLock interface {
	Acquire(ctx context.Context, name, holder string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, name, holder string) error
}

// Scheduler runs interval syncs.
//
// Without it a repository only ever syncs when a user presses Sync, so
// interval_seconds would be a stored, validated, ignored number. It is a plain
// loop rather than a job queue because the work is periodic and idempotent: a
// missed tick is caught by the next one, and the per-repository lock in SyncService
// makes an overlapping tick harmless.
type Scheduler struct {
	shutdown chan struct{}
	store    SchedulerStore
	sync     *SyncService
	lock     NodeLock
	log      *slog.Logger
	now      func() time.Time
	tick     time.Duration
}

// schedulerLockName is the lease every node contends for.
const schedulerLockName = "provisioning-scheduler"

// NewScheduler builds a Scheduler. A zero tick falls back to DefaultSchedulerTick.
func NewScheduler(store SchedulerStore, syncSvc *SyncService, log *slog.Logger, tick time.Duration) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	if tick <= 0 {
		tick = DefaultSchedulerTick
	}

	return &Scheduler{
		store:    store,
		sync:     syncSvc,
		log:      log,
		tick:     tick,
		now:      time.Now,
		shutdown: make(chan struct{}),
	}
}

// Tick exposes the loop's cadence, so a status view can report the real value rather than a
// default that may have been overridden.
func (s *Scheduler) Tick() time.Duration { return s.tick }

// Start runs the loop until ctx is cancelled. It returns immediately; the loop
// runs in its own goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

// Stop asks the loop to end. It is idempotent.
func (s *Scheduler) Stop() {
	select {
	case <-s.shutdown:
	default:
		close(s.shutdown)
	}
}

func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.tickOnce(ctx)
		}
	}
}

// SetNodeLock attaches the cross-node lease. It is a setter because the lock service is
// built alongside the database, after this scheduler is constructed.
func (s *Scheduler) SetNodeLock(l NodeLock) { s.lock = l }

// acquireLease claims the scheduler lease, reporting whether this node may run.
//
// The lease TTL is two ticks: long enough that a healthy node keeps renewing it before
// it expires, short enough that a node that dies releases the schedule within ~30s. With
// no lock configured the node is the only one, so it always proceeds.
func (s *Scheduler) acquireLease(ctx context.Context) bool {
	if s.lock == nil {
		return true
	}

	acquired, err := s.lock.Acquire(ctx, schedulerLockName, s.holder(), 2*s.tick)
	if err != nil {
		s.log.Warn("provisioning: could not acquire the scheduler lease", "err", err)

		return false
	}

	return acquired
}

// holder identifies this node in the lease.
func (s *Scheduler) holder() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "node"
	}

	return host + ":" + strconv.Itoa(os.Getpid())
}

// tickOnce syncs every repository whose interval has elapsed.
//
// A failure on one repository is logged and the rest continue: one unreachable
// remote must not stall the fleet. A repository already syncing is skipped rather
// than queued — the contract says a scheduled run is skipped, not stacked.
func (s *Scheduler) tickOnce(ctx context.Context) {
	if !s.acquireLease(ctx) {
		return
	}

	due, err := s.store.ListDueForSync(ctx, s.now())
	if err != nil {
		s.log.Error("provisioning: could not list due repositories", "err", err)

		return
	}

	for _, repo := range due {
		if ctx.Err() != nil {
			return
		}

		// The actor is what distinguishes this run from a person's in the audit
		// trail: nobody pressed anything, so it is recorded as the scheduler.
		if _, err := s.sync.Sync(ctx, repo, SyncOptions{Actor: domain.SchedulerActor()}); err != nil {
			switch {
			case errors.Is(err, domain.ErrSyncInProgress):
				s.log.Debug("provisioning: scheduled sync skipped, one is already running", "repo", repo.ID)
			case errors.Is(err, ErrSyncDisabled):
				s.log.Debug("provisioning: scheduled sync skipped, sync disabled", "repo", repo.ID)
			default:
				s.log.Warn("provisioning: scheduled sync failed", "repo", repo.ID, "err", err)
			}
		}
	}
}

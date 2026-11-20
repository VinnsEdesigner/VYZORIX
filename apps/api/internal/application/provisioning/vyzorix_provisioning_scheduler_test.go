package provisioning

import (
	"context"
	"sync"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestSchedulerOnlyRunsWhileHoldingTheLease is the guard for a multi-replica double sync.
//
// The per-repository lock in SyncService is in-process only, so on more than one replica
// every scheduler ticked and every repository was synced by each node: the deploys were
// applied twice and the two runs raced each other's job counters. The scheduler now needs
// the cross-node lease before it looks at anything.
func TestSchedulerOnlyRunsWhileHoldingTheLease(t *testing.T) {
	store := &countingSchedulerStore{}
	lock := &fakeNodeLock{granted: false}

	s := NewScheduler(store, nil, nil, time.Second)
	s.SetNodeLock(lock)
	s.tickOnce(context.Background())

	if store.calls != 0 {
		t.Fatalf("a node without the lease listed repositories %d time(s); it must do nothing", store.calls)
	}

	lock.granted = true
	s.tickOnce(context.Background())

	if store.calls != 1 {
		t.Fatalf("the lease holder listed repositories %d time(s), want 1", store.calls)
	}
}

// TestSchedulerWithoutLockAlwaysRuns proves a single-node deployment, which has no lock
// service, still schedules.
func TestSchedulerWithoutLockAlwaysRuns(t *testing.T) {
	store := &countingSchedulerStore{}
	s := NewScheduler(store, nil, nil, time.Second)
	s.tickOnce(context.Background())

	if store.calls != 1 {
		t.Fatalf("a lockless scheduler listed repositories %d time(s), want 1", store.calls)
	}
}

// TestSchedulerLeaseIsRenewedPerTick proves the lease TTL is renewed each tick, so a
// healthy node keeps the schedule rather than losing it after the first interval.
func TestSchedulerLeaseIsRenewedPerTick(t *testing.T) {
	lock := &fakeNodeLock{granted: true}
	s := NewScheduler(&countingSchedulerStore{}, nil, nil, time.Second)
	s.SetNodeLock(lock)

	s.tickOnce(context.Background())
	s.tickOnce(context.Background())

	if lock.acquires != 2 {
		t.Fatalf("acquired the lease %d time(s) over two ticks, want 2", lock.acquires)
	}
}

type countingSchedulerStore struct {
	mu    sync.Mutex
	calls int
}

func (c *countingSchedulerStore) ListDueForSync(context.Context, time.Time) ([]*domain.Repository, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls++

	return nil, nil
}

type fakeNodeLock struct {
	granted  bool
	acquires int
}

func (f *fakeNodeLock) Acquire(context.Context, string, string, time.Duration) (bool, error) {
	f.acquires++

	return f.granted, nil
}

func (f *fakeNodeLock) Release(context.Context, string, string) error { return nil }

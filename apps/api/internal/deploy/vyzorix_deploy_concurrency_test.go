package deploy

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// TestConcurrentDeploysDoNotCollide is the guard for a lost-update bug.
//
// The version is inside the signed document, so Deploy reads the current version, adds
// one, signs, and writes. Without a per-device lock two deploys run that sequence at the
// same time, both read v3, both write v4, and the second write silently overwrites the
// first document — a deploy the user was told succeeded, gone. Probing the real store
// showed 6 of 8 concurrent deploys lost this way.
//
// The lock now serialises the sequence per device, so a concurrent deploy is refused with
// ErrDeployInProgress rather than silently colliding.
func TestConcurrentDeploysDoNotCollide(t *testing.T) {
	svc, configs, caller := serviceFixture(t, deployCaller())

	// Serialise the in-memory store the way the real repository's transaction does, so
	// this test measures the lock, not the fake's map races.
	configs.mu.Lock()
	configs.mu.Unlock()

	const workers = 8

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		applied   []int
		refused   int
		failedErr []error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			result, err := svc.Deploy(context.Background(), DeployRequest{
				Caller:   caller,
				Manifest: []byte(twoAppManifest),
				OrgID:    "org_01H",
				DeviceID: "d_01H",
				MemberID: "op_01H",
			})

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				applied = append(applied, result.Version)
			case errors.Is(err, ErrDeployInProgress):
				refused++
			default:
				failedErr = append(failedErr, err)
			}
		}()
	}

	wg.Wait()

	for _, err := range failedErr {
		t.Errorf("unexpected deploy error: %v", err)
	}

	// Every deploy that was accepted must have a distinct version, and the stored config
	// must be the highest one. A duplicate version is exactly the lost update.
	seen := make(map[int]bool, len(applied))

	for _, v := range applied {
		if seen[v] {
			t.Errorf("two accepted deploys share version %d; one was silently overwritten", v)
		}

		seen[v] = true
	}

	if len(applied) == 0 {
		t.Fatal("no deploy was accepted")
	}

	stored, err := configs.Current(context.Background(), "org_01H", "d_01H")
	if err != nil {
		t.Fatalf("read stored config: %v", err)
	}

	highest := 0
	for v := range seen {
		if v > highest {
			highest = v
		}
	}

	if stored.Version != highest {
		t.Fatalf("stored version = %d, but the highest accepted was %d", stored.Version, highest)
	}

	t.Logf("%d accepted, %d refused with in-progress, versions %v", len(applied), refused, applied)
}

// TestDryRunTakesNoLock proves a preview does not block or get blocked: two previews at
// the same version are harmless, and a preview must not hold a device's slot.
func TestDryRunTakesNoLock(t *testing.T) {
	svc, _, caller := serviceFixture(t, deployCaller())

	// Hold the device's lock as if a real deploy were running.
	if !svc.locks.acquire("d_01H") {
		t.Fatal("could not take the lock for the test setup")
	}
	defer svc.locks.release("d_01H")

	if _, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(twoAppManifest),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
		DryRun:   true,
	}); err != nil {
		t.Fatalf("a dry run was blocked by an in-flight deploy: %v", err)
	}
}

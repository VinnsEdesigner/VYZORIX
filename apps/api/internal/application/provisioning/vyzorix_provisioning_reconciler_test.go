package provisioning

import (
	"context"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestReconcilerAppliesOnDeviceReport covers the stale-state defect: a resource applied
// while the device was offline stays "pending" forever, because the state was only ever
// stamped at apply time. The device's own report is the evidence that it caught up.
func TestReconcilerAppliesOnDeviceReport(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", Kind: domain.KindDevice, DeviceID: "dev-1",
		Path: "devices/dev-1/vyzorix.toml", ConfigVersion: 4,
		State:     domain.ResourcePending,
		LastError: "the device has not reported this config version yet",
	})

	NewReconciler(resources, nil).DeviceReported(context.Background(), "dev-1", 4)

	got, err := resources.GetByDevice(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("GetByDevice: %v", err)
	}
	if got.State != domain.ResourceApplied {
		t.Fatalf("state = %q, want applied after the device confirmed the version", got.State)
	}
	if got.LastError != "" {
		t.Fatalf("last_error = %q, want it cleared; a stale reason beside an applied badge is a lie", got.LastError)
	}
}

// A device reporting a different version means it runs something else, so an "applied" row
// must stop claiming otherwise.
func TestReconcilerUnappliesOnVersionMismatch(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", Kind: domain.KindDevice, DeviceID: "dev-1",
		Path: "devices/dev-1/vyzorix.toml", ConfigVersion: 4, State: domain.ResourceApplied,
	})

	NewReconciler(resources, nil).DeviceReported(context.Background(), "dev-1", 2)

	got, err := resources.GetByDevice(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("GetByDevice: %v", err)
	}
	if got.State != domain.ResourcePending {
		t.Fatalf("state = %q, want pending: the device reports a version this source did not apply", got.State)
	}
}

// Drift is a claim about cause ("someone changed the device after this source applied it"),
// which a version mismatch does not establish. The reconciler must not guess it.
func TestReconcilerNeverInventsDrift(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", Kind: domain.KindDevice, DeviceID: "dev-1",
		Path: "devices/dev-1/vyzorix.toml", ConfigVersion: 4, State: domain.ResourceApplied,
	})

	NewReconciler(resources, nil).DeviceReported(context.Background(), "dev-1", 9)

	got, _ := resources.GetByDevice(context.Background(), "dev-1")
	if got.State == domain.ResourceDrifted {
		t.Fatal("the reconciler must not set drifted: only the sync's own comparison establishes drift")
	}
}

// A device with no provisioned resource, or a nil reconciler, is a no-op rather than a panic:
// the hook runs on the WebSocket frame path for every connected device.
func TestReconcilerIsSafeForUnknownDevices(t *testing.T) {
	resources := newFakeResources()
	NewReconciler(resources, nil).DeviceReported(context.Background(), "not-provisioned", 1)

	var nilReconciler *Reconciler
	nilReconciler.DeviceReported(context.Background(), "dev-1", 1)

	NewReconciler(nil, nil).DeviceReported(context.Background(), "dev-1", 1)
}

// Re-applying a matching report must not write again: a device reports on every reconnect,
// and a write per report would churn the row (and its updated_at) for no change.
func TestReconcilerIsIdempotent(t *testing.T) {
	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "res-1", RepoID: "repo-1", Kind: domain.KindDevice, DeviceID: "dev-1",
		Path: "devices/dev-1/vyzorix.toml", ConfigVersion: 4, State: domain.ResourceApplied,
	})

	before := len(resources.states)
	NewReconciler(resources, nil).DeviceReported(context.Background(), "dev-1", 4)

	if len(resources.states) != before {
		t.Fatalf("wrote %d state changes for an already-applied resource, want 0",
			len(resources.states)-before)
	}
}

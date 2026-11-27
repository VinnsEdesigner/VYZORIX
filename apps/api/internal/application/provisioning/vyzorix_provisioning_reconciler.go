package provisioning

import (
	"context"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// Reconciler keeps a provisioned resource's state in step with what the device reports.
//
// Without it a state is stamped once, at apply time, and never revisited: a resource applied
// while the device was offline stays "pending" forever, even after the device fetches the
// config and says so. That is the opposite failure from the one the pending state fixed — a
// claim the server can see but never updates.
type Reconciler struct {
	resources domain.ResourceRepository
	log       Logger
}

// Logger is the slice of slog this file uses, so it can be tested without one.
type Logger interface {
	Warn(msg string, args ...any)
}

// NewReconciler builds a Reconciler.
func NewReconciler(resources domain.ResourceRepository, log Logger) *Reconciler {
	return &Reconciler{resources: resources, log: log}
}

// DeviceReported re-evaluates the resource that owns a device after the device reports the
// config version it holds.
//
// Only two transitions are made, both of which the report is direct evidence for:
//   - pending -> applied, when the device reports the version this repository applied.
//   - applied -> pending, when the device reports a different version, which means the
//     config it runs is not the one recorded here (a rollback, or a deploy from elsewhere).
//
// `drifted` is deliberately NOT set here: drift is "someone changed the device after this
// source applied it", which the sync's own comparison decides. Guessing it from a version
// mismatch would claim a cause the report does not establish.
func (r *Reconciler) DeviceReported(ctx context.Context, deviceID string, reportedVersion int) {
	if r == nil || r.resources == nil || deviceID == "" {
		return
	}

	resource, err := r.resources.GetByDevice(ctx, deviceID)
	if err != nil || resource == nil {
		// A device with no provisioned resource is the common case; nothing to reconcile.
		return
	}

	// Only a device resource has a device to report. An org resource carries no device id,
	// so GetByDevice cannot return one, but the guard documents the invariant.
	if resource.Kind != "" && resource.Kind != domain.KindDevice {
		return
	}

	switch {
	case reportedVersion == resource.ConfigVersion && resource.State == domain.ResourcePending:
		// The device caught up. Clear the pending reason: leaving it would show a stale
		// explanation next to an "applied" badge.
		if err := r.resources.SetState(ctx, resource.ID, domain.ResourceApplied, ""); err != nil && r.log != nil {
			r.log.Warn("provisioning: could not mark resource applied after device report",
				"resource", resource.ID, "device", deviceID, "err", err)
		}
	case reportedVersion != resource.ConfigVersion && resource.State == domain.ResourceApplied:
		// The device runs something else. It is not applied, whatever the row said.
		if err := r.resources.SetState(ctx, resource.ID, domain.ResourcePending,
			"the device reports a different config version than this source applied"); err != nil && r.log != nil {
			r.log.Warn("provisioning: could not mark resource pending after device report",
				"resource", resource.ID, "device", deviceID, "err", err)
		}
	}
}

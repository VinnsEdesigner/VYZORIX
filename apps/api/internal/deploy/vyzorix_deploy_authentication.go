package deploy

import (
	"context"
	"errors"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// The §7.1 predicate: who may deploy to a device. The decision is pure and the
// data it needs is injected, so it is testable without a database and cannot
// drift into the handler.

// Denial is why a deploy was refused. Each value maps to one HTTP response, so
// the reason an operator sees is decided here rather than in the handler.
type Denial string

const (
	// DenialUnauthenticated: no caller. 401.
	DenialUnauthenticated Denial = "unauthenticated"
	// DenialNotInOrg: the device does not exist in the caller's org. 404, never
	// 403 — a 403 would confirm the device exists, an existence leak across
	// tenants (§7.1).
	DenialNotInOrg Denial = "not_in_org"
	// DenialNotRegistered: the device is pending or deregistered. 409.
	DenialNotRegistered Denial = "not_registered"
	// DenialNoGrant: the caller is neither a super-admin nor holds device.deploy
	// on the device. 403.
	DenialNoGrant Denial = "no_grant"
)

// DeniedError wraps a Denial so a caller can match it with errors.Is and read
// the reason with errors.As.
type DeniedError struct {
	Denial Denial
	Detail string
}

// Error implements error.
func (e *DeniedError) Error() string {
	if e.Detail == "" {
		return "deploy denied: " + string(e.Denial)
	}

	return "deploy denied: " + string(e.Denial) + ": " + e.Detail
}

// Caller is the authenticated principal a deploy is attributed to. It carries
// the facts the predicate needs and nothing else.
type Caller struct {
	Role          string
	MemberID      string
	Permissions   permission.ScopedPermissions
	Authenticated bool
	IsSuperAdmin  bool
}

// Authorize applies §7.1. It returns nil when the caller may deploy to dev, or
// a *DeniedError naming the reason.
func Authorize(caller *Caller, dev *device.Device) error {
	if caller == nil || !caller.Authenticated {
		return &DeniedError{Denial: DenialUnauthenticated}
	}

	if dev == nil {
		return &DeniedError{Denial: DenialNotInOrg}
	}

	if dev.Lifecycle != device.LifecycleRegistered {
		return &DeniedError{
			Denial: DenialNotRegistered,
			Detail: string(dev.Lifecycle),
		}
	}

	if caller.IsSuperAdmin {
		return nil
	}

	// The scope is the device's own id, so a grant on "devices:*" covers it and
	// one on "devices:D" covers only D (permission.MatchScope).
	if caller.Permissions.Grants(permission.ActionDeviceDeploy, permission.BuildScope(permission.ScopeDevices, dev.ID)) {
		return nil
	}

	return &DeniedError{Denial: DenialNoGrant}
}

// DeviceLookup resolves a device by id within an org. A device in another org
// must read as ErrNotFound, so the caller cannot distinguish "absent" from
// "someone else's" — the existence leak §7.1 forbids.
type DeviceLookup interface {
	FindForOrg(ctx context.Context, deviceID, orgID string) (*device.Device, error)
}

// ErrDeviceNotInOrg is what a DeviceLookup returns when the device is absent or
// belongs to another org.
var ErrDeviceNotInOrg = errors.New("device not found in organization")

// AuthorizeDeploy is the composed check a handler calls: it loads the device
// scoped to the caller's org and applies the predicate. The device lookup is
// scoped by org, so a cross-org device is DenialNotInOrg rather than a grant
// failure.
func AuthorizeDeploy(ctx context.Context, lookup DeviceLookup, caller *Caller, orgID, deviceID string) (*device.Device, error) {
	if caller == nil || !caller.Authenticated {
		return nil, &DeniedError{Denial: DenialUnauthenticated}
	}

	dev, err := lookup.FindForOrg(ctx, deviceID, orgID)
	if err != nil {
		if errors.Is(err, ErrDeviceNotInOrg) {
			return nil, &DeniedError{Denial: DenialNotInOrg}
		}

		return nil, err
	}

	if authErr := Authorize(caller, dev); authErr != nil {
		return nil, authErr
	}

	return dev, nil
}

// CurrentAndLastGood reads a device's retained configs for the console's
// held-vs-target view. It is here rather than in the store so a handler has one
// call, and it treats a missing last-good as normal: a device deployed once has
// no prior version.
func CurrentAndLastGood(ctx context.Context, store deployconfig.Repository, deviceID string) (current, lastGood *deployconfig.Record, err error) {
	current, err = store.Current(ctx, deviceID)
	if err != nil && !errors.Is(err, deployconfig.ErrNotFound) {
		return nil, nil, err
	}

	lastGood, err = store.LastGood(ctx, deviceID)
	if err != nil && !errors.Is(err, deployconfig.ErrNotFound) {
		return nil, nil, err
	}

	return current, lastGood, nil
}

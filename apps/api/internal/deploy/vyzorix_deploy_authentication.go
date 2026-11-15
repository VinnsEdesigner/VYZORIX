package deploy

import (
	"context"
	"errors"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// The predicate: who may deploy to a device. The decision is pure and the
// data it needs is injected, so it is testable without a database and cannot
// drift into the handler.

// Denial is why a deploy was refused. Each value maps to one HTTP response, so
// the reason a user sees is decided here rather than in the handler.
type Denial string

const (
	// DenialUnauthenticated: no caller. 401.
	DenialUnauthenticated Denial = "unauthenticated"
	// DenialNotInOrg: the device does not exist in the caller's org. 404, never
	// 403 — a 403 would confirm the device exists, an existence leak across
	// tenants.
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
	OrgID         string
	Permissions   permission.ScopedPermissions
	Authenticated bool
	IsSuperAdmin  bool
}

// Authorize applies. It returns nil when the caller may deploy to dev, or
// a *DeniedError naming the reason.
func Authorize(caller *Caller, dev *device.Device) error {
	if caller == nil || !caller.Authenticated {
		return &DeniedError{Denial: DenialUnauthenticated}
	}

	if dev == nil {
		return &DeniedError{Denial: DenialNotInOrg}
	}

	// D ∈ O. A device in another org reads as not-in-org, never as no-grant, so
	// a caller cannot distinguish "absent" from "someone else's".
	if caller.OrgID == "" || caller.OrgID != dev.OrganizationID {
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
// "someone else's" — the existence leak forbids.
type DeviceLookup interface {
	FindForOrg(ctx context.Context, deviceID, orgID string) (*device.Device, error)
}

// ErrDeviceNotInOrg is what a DeviceLookup returns when the device is absent or
// belongs to another org.
var ErrDeviceNotInOrg = errors.New("device not found in organization")

// AuthorizeDeploy is the composed check a handler calls: it loads the device
// scoped to the caller's org and applies the predicate. The org comes from the
// caller, so there is one source of truth and the lookup cannot be pointed at a
// different org than the predicate checks.
func AuthorizeDeploy(ctx context.Context, lookup DeviceLookup, caller *Caller, deviceID string) (*device.Device, error) {
	if caller == nil || !caller.Authenticated {
		return nil, &DeniedError{Denial: DenialUnauthenticated}
	}

	if caller.OrgID == "" {
		return nil, &DeniedError{Denial: DenialNotInOrg}
	}

	dev, err := lookup.FindForOrg(ctx, deviceID, caller.OrgID)
	if err != nil {
		// Both the lookup's own sentinel and the device repository's ErrNotFound read
		// as not-in-org, so a device in another tenant is indistinguishable from an
		// absent one whichever layer reported it.
		if errors.Is(err, ErrDeviceNotInOrg) || errors.Is(err, device.ErrNotFound) {
			return nil, &DeniedError{Denial: DenialNotInOrg}
		}

		return nil, err
	}

	if authErr := Authorize(caller, dev); authErr != nil {
		return nil, authErr
	}

	return dev, nil
}

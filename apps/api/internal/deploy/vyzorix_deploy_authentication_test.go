package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

func registeredDevice(id string) *device.Device {
	return &device.Device{ID: id, OrganizationID: "org_1", Lifecycle: device.LifecycleRegistered}
}

func callerWith(grants ...permission.ScopedPermission) *Caller {
	return &Caller{
		Authenticated: true,
		OrgID:         "org_1",
		Permissions:   grants,
	}
}

// denialOf extracts the Denial from an error, failing if it is not an *DeniedError.
func denialOf(t *testing.T, err error) Denial {
	t.Helper()

	if err == nil {
		t.Fatal("expected a denial, got nil")
	}

	var denied *DeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected *DeniedError, got %T: %v", err, err)
	}

	return denied.Denial
}

// TestSuperAdminMayDeploy covers the first clause: a super-admin needs no grant.
func TestSuperAdminMayDeploy(t *testing.T) {
	t.Parallel()

	caller := &Caller{Authenticated: true, IsSuperAdmin: true, OrgID: "org_1"}

	if err := Authorize(caller, registeredDevice("d_1")); err != nil {
		t.Fatalf("a super-admin must be allowed: %v", err)
	}
}

// TestGrantOnTheDeviceAllowsDeploy covers the second clause, scoped exactly.
func TestGrantOnTheDeviceAllowsDeploy(t *testing.T) {
	t.Parallel()

	caller := callerWith(permission.ScopedPermission{
		Action: permission.ActionDeviceDeploy,
		Scope:  "devices:d_1",
	})

	if err := Authorize(caller, registeredDevice("d_1")); err != nil {
		t.Fatalf("a grant on the device must be allowed: %v", err)
	}
}

// TestWildcardGrantCoversTheDevice covers the devices:* scope.
func TestWildcardGrantCoversTheDevice(t *testing.T) {
	t.Parallel()

	caller := callerWith(permission.ScopedPermission{
		Action: permission.ActionDeviceDeploy,
		Scope:  "devices:*",
	})

	if err := Authorize(caller, registeredDevice("d_1")); err != nil {
		t.Fatalf("a devices:* grant must cover the device: %v", err)
	}
}

// TestGrantOnAnotherDeviceIsRefused is the scoping guarantee: a deploy
// credential or grant that reaches one device must not reach another.
func TestGrantOnAnotherDeviceIsRefused(t *testing.T) {
	t.Parallel()

	caller := callerWith(permission.ScopedPermission{
		Action: permission.ActionDeviceDeploy,
		Scope:  "devices:d_2",
	})

	if got := denialOf(t, Authorize(caller, registeredDevice("d_1"))); got != DenialNoGrant {
		t.Fatalf("a grant on another device must be refused, got %q", got)
	}
}

// TestAppsWriteDoesNotImplyDeploy is the FR-18 invariant: the permission to
// observe an app must not let an operator install one.
func TestAppsWriteDoesNotImplyDeploy(t *testing.T) {
	t.Parallel()

	caller := callerWith(
		permission.ScopedPermission{Action: permission.ActionAppsRead, Scope: "devices:*"},
		permission.ScopedPermission{Action: permission.ActionAppsWrite, Scope: "devices:*"},
	)

	if got := denialOf(t, Authorize(caller, registeredDevice("d_1"))); got != DenialNoGrant {
		t.Fatalf("apps.write must not imply deploy, got %q", got)
	}
}

// TestDeviceManageDoesNotImplyDeploy is the action-set invariant: the aggregate
// must not have been widened to include deploy.
func TestDeviceManageDoesNotImplyDeploy(t *testing.T) {
	t.Parallel()

	// device.manage expands to read/write/delete/assign, not deploy.
	expanded := permission.ExpandActionSets(permission.ScopedPermissions{
		{Action: "device.manage", Scope: "devices:*"},
	})

	caller := callerWith(expanded...)

	if got := denialOf(t, Authorize(caller, registeredDevice("d_1"))); got != DenialNoGrant {
		t.Fatalf("device.manage must not imply deploy, got %q", got)
	}
}

// TestAuthorizeRefusesACrossOrgDevice is the D ∈ O conjunct, checked in
// Authorize itself: a device from another org must be refused even when handed
// in directly, so the function cannot be misused by a caller that forgot to
// scope its lookup.
func TestAuthorizeRefusesACrossOrgDevice(t *testing.T) {
	t.Parallel()

	dev := &device.Device{ID: "d_1", OrganizationID: "org_2", Lifecycle: device.LifecycleRegistered}
	caller := callerWith(permission.ScopedPermission{Action: permission.ActionDeviceDeploy, Scope: "devices:*"})

	if got := denialOf(t, Authorize(caller, dev)); got != DenialNotInOrg {
		t.Fatalf("a cross-org device must be DenialNotInOrg, got %q", got)
	}
}

// TestAuthorizeRefusesASuperAdminFromAnotherOrg proves the org binding is not
// bypassed by the super-admin clause.
func TestAuthorizeRefusesASuperAdminFromAnotherOrg(t *testing.T) {
	t.Parallel()

	dev := &device.Device{ID: "d_1", OrganizationID: "org_2", Lifecycle: device.LifecycleRegistered}
	caller := &Caller{Authenticated: true, IsSuperAdmin: true, OrgID: "org_1"}

	if got := denialOf(t, Authorize(caller, dev)); got != DenialNotInOrg {
		t.Fatalf("the org binding must apply to a super-admin too, got %q", got)
	}
}

// TestUnauthenticatedIsRefused proves a grant does not help an anonymous caller.
func TestUnauthenticatedIsRefused(t *testing.T) {
	t.Parallel()

	caller := callerWith(permission.ScopedPermission{Action: permission.ActionDeviceDeploy, Scope: "devices:*"})
	caller.Authenticated = false

	if got := denialOf(t, Authorize(caller, registeredDevice("d_1"))); got != DenialUnauthenticated {
		t.Fatalf("an unauthenticated caller must be refused, got %q", got)
	}
}

// TestNilCallerIsRefused covers the absent-caller path.
func TestNilCallerIsRefused(t *testing.T) {
	t.Parallel()

	if got := denialOf(t, Authorize(nil, registeredDevice("d_1"))); got != DenialUnauthenticated {
		t.Fatalf("a nil caller must be refused, got %q", got)
	}
}

// TestPendingDeviceIsRefused covers the lifecycle gate.
func TestPendingDeviceIsRefused(t *testing.T) {
	t.Parallel()

	dev := &device.Device{ID: "d_1", OrganizationID: "org_1", Lifecycle: device.LifecyclePending}
	caller := callerWith(permission.ScopedPermission{Action: permission.ActionDeviceDeploy, Scope: "devices:*"})

	if got := denialOf(t, Authorize(caller, dev)); got != DenialNotRegistered {
		t.Fatalf("a pending device must be refused, got %q", got)
	}
}

// TestDeregisteredDeviceIsRefused covers the other lifecycle gate.
func TestDeregisteredDeviceIsRefused(t *testing.T) {
	t.Parallel()

	dev := &device.Device{ID: "d_1", OrganizationID: "org_1", Lifecycle: device.LifecycleDeregistered}
	caller := callerWith(permission.ScopedPermission{Action: permission.ActionDeviceDeploy, Scope: "devices:*"})

	if got := denialOf(t, Authorize(caller, dev)); got != DenialNotRegistered {
		t.Fatalf("a deregistered device must be refused, got %q", got)
	}
}

// TestSuperAdminStillBoundByLifecycle proves the bypass is the grant check only.
func TestSuperAdminStillBoundByLifecycle(t *testing.T) {
	t.Parallel()

	dev := &device.Device{ID: "d_1", OrganizationID: "org_1", Lifecycle: device.LifecyclePending}
	caller := &Caller{Authenticated: true, IsSuperAdmin: true, OrgID: "org_1"}

	if got := denialOf(t, Authorize(caller, dev)); got != DenialNotRegistered {
		t.Fatalf("a super-admin must not bypass the lifecycle gate, got %q", got)
	}
}

// fakeLookup is a DeviceLookup backed by a map keyed by device id, scoped by
// org. A device in another org is absent from the map for that org's lookup.
type fakeLookup struct {
	devices map[string]*device.Device
}

func (f *fakeLookup) FindForOrg(_ context.Context, deviceID, orgID string) (*device.Device, error) {
	dev, ok := f.devices[deviceID]
	if !ok || dev.OrganizationID != orgID {
		return nil, ErrDeviceNotInOrg
	}

	return dev, nil
}

// TestCrossOrgDeviceIsNotInOrg proves the 404-not-403 rule: another org's device
// is indistinguishable from an absent one.
func TestCrossOrgDeviceIsNotInOrg(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{devices: map[string]*device.Device{
		"d_other": {ID: "d_other", OrganizationID: "org_2", Lifecycle: device.LifecycleRegistered},
	}}

	caller := callerWith(permission.ScopedPermission{Action: permission.ActionDeviceDeploy, Scope: "devices:*"})

	_, err := AuthorizeDeploy(context.Background(), lookup, caller, "d_other")

	if got := denialOf(t, err); got != DenialNotInOrg {
		t.Fatalf("another org's device must be DenialNotInOrg, got %q", got)
	}
}

// TestAuthorizeDeployAllowsTheHappyPath covers the composed check.
func TestAuthorizeDeployAllowsTheHappyPath(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{devices: map[string]*device.Device{
		"d_1": {ID: "d_1", OrganizationID: "org_1", Lifecycle: device.LifecycleRegistered},
	}}

	caller := callerWith(permission.ScopedPermission{Action: permission.ActionDeviceDeploy, Scope: "devices:d_1"})

	dev, err := AuthorizeDeploy(context.Background(), lookup, caller, "d_1")
	if err != nil {
		t.Fatalf("the happy path must be allowed: %v", err)
	}

	if dev.ID != "d_1" {
		t.Fatalf("expected the looked-up device, got %q", dev.ID)
	}
}

// TestAuthorizeDeployIsUnauthenticatedBeforeLookup proves an anonymous caller
// does not trigger a device lookup, which would be an existence oracle.
func TestAuthorizeDeployIsUnauthenticatedBeforeLookup(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{devices: map[string]*device.Device{}}

	_, err := AuthorizeDeploy(context.Background(), lookup, &Caller{OrgID: "org_1"}, "d_1")

	if got := denialOf(t, err); got != DenialUnauthenticated {
		t.Fatalf("an unauthenticated caller must be refused, got %q", got)
	}
}

// TestDeployIsNotInAnyActionSet pins the FR-18 decision: no aggregate action may
// imply deploy, or the observe-implies-install grant reappears one level up.
func TestDeployIsNotInAnyActionSet(t *testing.T) {
	t.Parallel()

	for set, members := range permission.ActionSets() {
		for _, member := range members {
			if member == permission.ActionDeviceDeploy {
				t.Fatalf("action set %q must not imply %q", set, permission.ActionDeviceDeploy)
			}
		}
	}
}

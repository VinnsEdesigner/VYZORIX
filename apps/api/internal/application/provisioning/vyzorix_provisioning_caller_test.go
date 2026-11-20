package provisioning

import (
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestCallerAuthorizesAnyDeviceInOrg is the guard for the deploy caller.
//
// The caller used to be marked IsSuperAdmin, which is a bypass in deploy.Authorize that
// skips the grant check. That silently inherited every future superadmin-only privilege
// on the deploy path. It now declares the single grant a sync needs, so this test proves
// the grant still authorizes a deploy in the repository's own org — and that it does not
// authorize a device in another org.
func TestCallerAuthorizesAnyDeviceInOrg(t *testing.T) {
	svc := &SyncService{}
	repo := &domain.Repository{ID: "repo-1", OrgID: "org-1"}

	caller := svc.caller(repo)
	if caller.IsSuperAdmin {
		t.Fatal("the provisioning caller must not claim superadmin; that bypasses the grant check")
	}

	inOrg := &device.Device{ID: "dev-1", OrganizationID: "org-1", Lifecycle: device.LifecycleRegistered}
	if err := deploy.Authorize(caller, inOrg); err != nil {
		t.Fatalf("the sync caller cannot deploy to its own org: %v", err)
	}

	otherOrg := &device.Device{ID: "dev-2", OrganizationID: "org-2", Lifecycle: device.LifecycleRegistered}
	if err := deploy.Authorize(caller, otherOrg); err == nil {
		t.Fatal("the sync caller deployed to a device outside its org")
	}
}

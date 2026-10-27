package permission_test

import (
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// TestShellAndUsbAreAdminOnly is the guard on the most dangerous capabilities:
// a shell is unbounded code execution and a USB attach can flash external
// hardware. Neither may be implied by the operator defaults.
func TestShellAndUsbAreAdminOnly(t *testing.T) {
	devicesAll := permission.WildcardScope(permission.ScopeDevices)

	cases := []struct {
		role  string
		shell bool
		usb   bool
	}{
		{"viewer", false, false},
		{"operator", false, false},
		{"admin", true, true},
		{"super_admin", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			perms := permission.DefaultScopesForRole(tc.role)

			if got := perms.Grants(permission.ActionShell, devicesAll); got != tc.shell {
				t.Errorf("shell.execute for %s = %v, want %v", tc.role, got, tc.shell)
			}
			if got := perms.Grants(permission.ActionUsb, devicesAll); got != tc.usb {
				t.Errorf("usb.attach for %s = %v, want %v", tc.role, got, tc.usb)
			}
		})
	}
}

// TestAppPermissionsByRole pins the app lifecycle split: operators may start and
// stop supervised apps, viewers may only see them.
func TestAppPermissionsByRole(t *testing.T) {
	devicesAll := permission.WildcardScope(permission.ScopeDevices)

	viewer := permission.DefaultScopesForRole("viewer")
	if !viewer.Grants(permission.ActionAppsRead, devicesAll) {
		t.Error("a viewer must be able to list apps")
	}
	if viewer.Grants(permission.ActionAppsWrite, devicesAll) {
		t.Error("a viewer must not be able to start or stop apps")
	}

	operator := permission.DefaultScopesForRole("operator")
	if !operator.Grants(permission.ActionAppsWrite, devicesAll) {
		t.Error("an operator must be able to start and stop apps")
	}
}

// TestShellNotImpliedByCommand proves the capabilities are independent: holding
// command.execute must not confer shell access.
func TestShellNotImpliedByCommand(t *testing.T) {
	devicesAll := permission.WildcardScope(permission.ScopeDevices)
	operator := permission.DefaultScopesForRole("operator")

	if !operator.Grants(permission.ActionCommand, devicesAll) {
		t.Fatal("an operator must be able to execute commands")
	}
	if operator.Grants(permission.ActionShell, devicesAll) {
		t.Fatal("command.execute must not imply shell.execute")
	}
}

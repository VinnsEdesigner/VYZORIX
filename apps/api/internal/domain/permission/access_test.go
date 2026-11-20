package permission

import "testing"

// TestActionAccessClassifiesEveryAction guards the table's completeness: an
// action missing from actionAccess falls back to AccessAdmin, which would
// silently deny it. This test fails when a new Action constant is added without
// a classification, forcing a deliberate decision.
func TestActionAccessClassifiesEveryAction(t *testing.T) {
	all := []Action{
		ActionDeviceRead, ActionDeviceWrite, ActionDeviceDelete, ActionDeviceAssign,
		ActionCommand, ActionShell, ActionUsb, ActionDeviceDeploy,
		ActionAppsRead, ActionAppsWrite,
		ActionTelemetryRead, ActionTelemetryWrite, ActionDiagnosticsRead,
		ActionEventsRead, ActionLogsRead, ActionConnectionsRead,
		ActionStatsRead, ActionSearchRead, ActionAlertRead, ActionAlertWrite,
		ActionNotificationsRead, ActionNotificationsWrite,
		ActionServiceAccountsRead, ActionServiceAccountsWrite,
		ActionAnnotationsRead, ActionAnnotationsWrite,
		ActionConfigVersionsRead, ActionConfigVersionsWrite,
		ActionInboxRead, ActionInboxWrite,
		ActionUpdateRead, ActionUpdateWrite, ActionUpdateDelete,
		ActionSettingsRead, ActionSettingsWrite,
		ActionMembersRead, ActionMembersWrite, ActionMembersDelete,
		ActionAuditRead, ActionKeysManage,
		ActionProvisioningRead, ActionProvisioningWrite,
		ActionAdminLockout, ActionAdminImpersonate, ActionAdminUsers, ActionOrgList, ActionOrgCreate,
		ActionAccountRead, ActionAccountWrite, ActionInvitationRead,
	}

	for _, a := range all {
		if _, ok := actionAccess[a]; !ok {
			t.Errorf("action %q has no access classification (would fall back to admin)", a)
		}
	}
}

// TestUnknownActionRequiresAdmin pins the fail-closed default.
func TestUnknownActionRequiresAdmin(t *testing.T) {
	if got := RequiredAccess(Action("totally.unknown")); got != AccessAdmin {
		t.Errorf("unknown action required %v, want admin (fail closed)", got)
	}
}

// TestScopesSatisfy is the core of the coarse-scope model: read sees, write
// changes, admin destroys — and privilege is monotonic.
func TestScopesSatisfy(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		action Action
		want   bool
	}{
		// Read.
		{"read may read devices", []string{"read"}, ActionDeviceRead, true},
		{"read may not write devices", []string{"read"}, ActionDeviceWrite, false},
		{"read may not delete devices", []string{"read"}, ActionDeviceDelete, false},

		// Write.
		{"write may read", []string{"write"}, ActionDeviceRead, true},
		{"write may write", []string{"write"}, ActionDeviceWrite, true},
		{"write may not delete", []string{"write"}, ActionDeviceDelete, false},

		// Admin.
		{"admin may read", []string{"admin"}, ActionDeviceRead, true},
		{"admin may write", []string{"admin"}, ActionDeviceWrite, true},
		{"admin may delete", []string{"admin"}, ActionDeviceDelete, true},

		// The bug this table fixes: verbs other than .read/.write were never
		// grantable by the old suffix check.
		{"write may execute a command", []string{"write"}, ActionCommand, true},
		{"write may not attach usb", []string{"write"}, ActionUsb, false},
		{"admin may attach usb", []string{"admin"}, ActionUsb, true},
		{"admin may open a shell", []string{"admin"}, ActionShell, true},
		{"write may not open a shell", []string{"write"}, ActionShell, false},
		{"admin may manage keys", []string{"admin"}, ActionKeysManage, true},
		{"write may not manage keys", []string{"write"}, ActionKeysManage, false},
		{"admin may assign devices", []string{"admin"}, ActionDeviceAssign, true},
		{"write may not assign devices", []string{"write"}, ActionDeviceAssign, false},

		// Multiple scopes: the strongest decides.
		{"read+admin is admin", []string{"read", "admin"}, ActionDeviceDelete, true},
		{"read+write is write", []string{"read", "write"}, ActionCommand, true},

		// Unknown or empty scopes grant nothing.
		{"unknown scope grants nothing", []string{"superuser"}, ActionDeviceRead, false},
		{"no scopes grants nothing", nil, ActionDeviceRead, false},
		{"empty scope grants nothing", []string{""}, ActionDeviceRead, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScopesSatisfy(tc.scopes, tc.action); got != tc.want {
				t.Errorf("ScopesSatisfy(%v, %s) = %v, want %v", tc.scopes, tc.action, got, tc.want)
			}
		})
	}
}

// TestServiceAccountActionLevels records a deliberate split: listing accounts is
// read-level, but writing them is admin, because minting or deleting a
// credential must never be something automation can do to itself.
func TestServiceAccountActionLevels(t *testing.T) {
	if RequiredAccess(ActionServiceAccountsRead) != AccessRead {
		t.Error("service_accounts.read should be read-level")
	}
	if RequiredAccess(ActionServiceAccountsWrite) != AccessAdmin {
		t.Error("service_accounts.write should require admin")
	}
	if !ScopesSatisfy([]string{"read"}, ActionServiceAccountsRead) {
		t.Error("a read token should be able to list service accounts")
	}
	if ScopesSatisfy([]string{"read"}, ActionServiceAccountsWrite) {
		t.Error("a read token must not mint a service account")
	}
	if ScopesSatisfy([]string{"write"}, ActionServiceAccountsWrite) {
		t.Error("even a write token must not mint a service account")
	}
}

// TestAccessLevelSatisfies pins the ordering and the invalid-input behaviour.
func TestAccessLevelSatisfies(t *testing.T) {
	if !AccessAdmin.Satisfies(AccessWrite) || !AccessAdmin.Satisfies(AccessRead) {
		t.Error("admin must satisfy write and read")
	}
	if !AccessWrite.Satisfies(AccessRead) {
		t.Error("write must satisfy read")
	}
	if AccessRead.Satisfies(AccessWrite) || AccessRead.Satisfies(AccessAdmin) {
		t.Error("read must not satisfy write or admin")
	}
	if (AccessLevel(0)).Satisfies(AccessRead) {
		t.Error("an unset level must satisfy nothing")
	}
}

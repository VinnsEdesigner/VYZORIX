package permission

import "sort"

// AccessLevel is the privilege an action requires, compared against a bearer
// credential's coarse scope.
//
// The table is explicit rather than derived from the action name: a suffix check
// silently made command.execute, shell.execute, usb.attach, keys.manage and
// device.assign un-grantable. An unclassified action is AccessAdmin (fail
// closed).
type AccessLevel int

const (
	// AccessRead observes state; AccessWrite changes it; AccessAdmin is
	// destructive or privileged. Privilege is monotonic.
	AccessRead AccessLevel = iota + 1
	AccessWrite
	AccessAdmin
)

func (a AccessLevel) String() string {
	switch a {
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	case AccessAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// Satisfies reports whether this level permits an action requiring need.
func (a AccessLevel) Satisfies(need AccessLevel) bool {
	if a < AccessRead || need < AccessRead {
		return false
	}
	return a >= need
}

// actionAccess is the single source of truth for what each action requires.
// Every Action constant must appear here; a missing entry is AccessAdmin.
var actionAccess = map[Action]AccessLevel{
	// Devices: observe, then modify, then destroy/reassign.
	ActionDeviceRead:   AccessRead,
	ActionDeviceWrite:  AccessWrite,
	ActionDeviceDelete: AccessAdmin,
	ActionDeviceAssign: AccessAdmin,
	// Arbitrary or irreversible device operations.
	ActionCommand:      AccessWrite,
	ActionShell:        AccessAdmin,
	ActionUsb:          AccessAdmin,
	ActionDeviceDeploy: AccessAdmin,
	// Apps: list vs start/stop.
	ActionAppsRead:  AccessRead,
	ActionAppsWrite: AccessWrite,
	// Observability.
	ActionTelemetryRead:     AccessRead,
	ActionTelemetryWrite:    AccessWrite,
	ActionDiagnosticsRead:   AccessRead,
	ActionEventsRead:        AccessRead,
	ActionLogsRead:          AccessRead,
	ActionConnectionsRead:   AccessRead,
	ActionStatsRead:         AccessRead,
	ActionSearchRead:        AccessRead,
	ActionAlertRead:         AccessRead,
	ActionAlertWrite:        AccessWrite,
	ActionNotificationsRead: AccessRead,
	// Contact points carry delivery credentials; writing them is privileged.
	ActionNotificationsWrite: AccessAdmin,
	// Listing service accounts is ordinary read access: it reveals names and
	// counts, which an operator already sees in the console. Minting, renaming or
	// deleting them is admin, because each of those creates or destroys a
	// credential — automation must never be able to mint more automation.
	ActionServiceAccountsRead:  AccessRead,
	ActionServiceAccountsWrite: AccessAdmin,
	// Annotations and config history.
	ActionAnnotationsRead:     AccessRead,
	ActionAnnotationsWrite:    AccessWrite,
	ActionConfigVersionsRead:  AccessRead,
	ActionConfigVersionsWrite: AccessAdmin,
	// Registration inbox.
	ActionInboxRead:  AccessRead,
	ActionInboxWrite: AccessWrite,
	// Updates.
	ActionUpdateRead:   AccessRead,
	ActionUpdateWrite:  AccessWrite,
	ActionUpdateDelete: AccessAdmin,
	// Organisation administration.
	ActionSettingsRead:      AccessRead,
	ActionSettingsWrite:     AccessAdmin,
	ActionMembersRead:       AccessRead,
	ActionMembersWrite:      AccessWrite,
	ActionMembersDelete:     AccessAdmin,
	ActionAuditRead:         AccessAdmin,
	ActionKeysManage:        AccessAdmin,
	ActionProvisioningRead:  AccessRead,
	ActionProvisioningWrite: AccessAdmin,
	// Instance- and personal-scoped actions are never satisfied by an
	// organisation-scoped credential, so their level only matters for the
	// instance/personal stores. They are admin here so a token can never reach
	// them.
	ActionAdminLockout:     AccessAdmin,
	ActionAdminImpersonate: AccessAdmin,
	ActionAdminUsers:       AccessAdmin,
	ActionOrgList:          AccessAdmin,
	ActionOrgCreate:        AccessAdmin,
	ActionAccountRead:      AccessRead,
	ActionAccountWrite:     AccessWrite,
	ActionInvitationRead:   AccessRead,
}

// actionScope is the resource each action acts on, stated explicitly so a fixed
// role expands to concrete ScopedPermissions without guessing.
var actionScope = map[Action]string{
	ActionDeviceRead: ScopeDevices, ActionDeviceWrite: ScopeDevices,
	ActionDeviceDelete: ScopeDevices, ActionDeviceAssign: ScopeDevices,
	ActionCommand: ScopeDevices, ActionShell: ScopeDevices, ActionUsb: ScopeDevices,
	ActionDeviceDeploy: ScopeDevices,
	ActionAppsRead:     ScopeDevices, ActionAppsWrite: ScopeDevices,
	ActionTelemetryRead: ScopeTelemetry, ActionTelemetryWrite: ScopeTelemetry,
	ActionDiagnosticsRead: ScopeDiagnostics,
	ActionEventsRead:      ScopeDevices, ActionLogsRead: ScopeDevices,
	ActionConnectionsRead: ScopeDevices,
	ActionStatsRead:       ScopeOrg, ActionSearchRead: ScopeOrg,
	ActionAlertRead: ScopeOrg, ActionAlertWrite: ScopeOrg,
	ActionNotificationsRead: ScopeOrg, ActionNotificationsWrite: ScopeOrg,
	ActionServiceAccountsRead: ScopeOrg, ActionServiceAccountsWrite: ScopeOrg,
	ActionAnnotationsRead: ScopeOrg, ActionAnnotationsWrite: ScopeOrg,
	ActionConfigVersionsRead: ScopeOrg, ActionConfigVersionsWrite: ScopeOrg,
	ActionInboxRead: ScopeInbox, ActionInboxWrite: ScopeInbox,
	ActionUpdateRead: ScopeUpdates, ActionUpdateWrite: ScopeUpdates,
	ActionUpdateDelete: ScopeUpdates,
	ActionSettingsRead: ScopeOrg, ActionSettingsWrite: ScopeOrg,
	ActionMembersRead: ScopeOrg, ActionMembersWrite: ScopeOrg,
	ActionMembersDelete: ScopeOrg,
	ActionAuditRead:     ScopeOrg, ActionKeysManage: ScopeOrg,
	ActionProvisioningRead: ScopeOrg, ActionProvisioningWrite: ScopeOrg,
}

// RequiredScope returns the resource scope prefix an action acts on. An unknown
// action returns "" — callers must treat that as unmatchable (fail closed).
func RequiredScope(a Action) string {
	return actionScope[a]
}

// RequiredAccess returns the level an action requires; unknown is AccessAdmin.
func RequiredAccess(a Action) AccessLevel {
	if level, ok := actionAccess[a]; ok {
		return level
	}
	return AccessAdmin
}

// SatisfiedBy reports whether a coarse scope (read/write/admin) permits a.
func SatisfiedBy(scope string, a Action) bool {
	have, ok := parseAccess(scope)
	if !ok {
		return false
	}
	return have.Satisfies(RequiredAccess(a))
}

// parseAccess maps a coarse scope name to its access level.
func parseAccess(scope string) (AccessLevel, bool) {
	switch scope {
	case "read":
		return AccessRead, true
	case "write":
		return AccessWrite, true
	case "admin":
		return AccessAdmin, true
	default:
		return 0, false
	}
}

// HighestAccess returns the strongest level among the given scopes, and whether
// any scope was recognised.
func HighestAccess(scopes []string) (AccessLevel, bool) {
	var highest AccessLevel
	recognised := false
	for _, s := range scopes {
		if level, ok := parseAccess(s); ok {
			recognised = true
			if level > highest {
				highest = level
			}
		}
	}
	return highest, recognised
}

// ScopesSatisfy reports whether any of the scopes permits a; the strongest wins.
func ScopesSatisfy(scopes []string, a Action) bool {
	have, ok := HighestAccess(scopes)
	if !ok {
		return false
	}
	return have.Satisfies(RequiredAccess(a))
}

// OrgScopedActions lists every action whose resource is organisation-scoped.
// It is the set a coarse read/write/admin grant may address.
func OrgScopedActions() []Action {
	out := make([]Action, 0, len(actionScope))
	for action, scope := range actionScope {
		if scope == ScopeOrg {
			out = append(out, action)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

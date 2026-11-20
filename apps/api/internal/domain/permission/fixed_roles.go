package permission

import "sort"

// FixedRoleScope is the breadth a fixed role is granted at.
type FixedRoleScope string

const (
	// ScopeOrgFixed covers the caller's organization only ("organizational" in
	// Grafana's UI). It never reaches beyond the org the credential is pinned to.
	ScopeOrgFixed FixedRoleScope = "organization"
	// ScopeAllFixed covers every resource the action can address. For a
	// single-org credential the reachable set is the same, but the grant is
	// expressed without the org narrowing, so it survives acting in another org.
	ScopeAllFixed FixedRoleScope = "all"
)

// FixedRole is a named bundle of scoped actions, in a Reader and Writer variant.
//
// Field order is fieldalignment-optimal (strings and the slice first, the
// 8-byte enum last), as elsewhere in the domain; the tags, not the order, define
// the meaning.
type FixedRole struct {
	// ID is the stable identifier stored on a token, e.g. "devices:writer".
	ID string
	// Group is the human grouping shown in the picker, e.g. "Devices".
	Group string
	// Actions are the concrete actions the role grants.
	Actions []Action
	// Access is the level this role grants: read for Reader, write for Writer.
	Access AccessLevel
}

// FixedRoleIDs is the catalog of fixed roles. Each group declares its reader
// actions once; the writer variant is the same set plus the write actions, so a
// Writer always supersedes its Reader (Grafana's ConcatPermissions pattern).
var fixedRoleCatalog = []FixedRole{
	{
		ID: "devices:reader", Group: "Devices", Access: AccessRead,
		Actions: []Action{ActionDeviceRead, ActionAppsRead, ActionEventsRead, ActionLogsRead, ActionConnectionsRead},
	},
	{
		ID: "devices:writer", Group: "Devices", Access: AccessWrite,
		Actions: []Action{
			ActionDeviceRead, ActionAppsRead, ActionEventsRead, ActionLogsRead, ActionConnectionsRead,
			ActionDeviceWrite, ActionAppsWrite, ActionCommand, ActionInboxRead, ActionInboxWrite,
		},
	},
	{
		ID: "telemetry:reader", Group: "Telemetry", Access: AccessRead,
		Actions: []Action{ActionTelemetryRead, ActionDiagnosticsRead, ActionStatsRead},
	},
	{
		ID: "telemetry:writer", Group: "Telemetry", Access: AccessWrite,
		Actions: []Action{ActionTelemetryRead, ActionDiagnosticsRead, ActionStatsRead, ActionTelemetryWrite},
	},
	{
		ID: "alerting:reader", Group: "Alerting", Access: AccessRead,
		Actions: []Action{ActionAlertRead},
	},
	{
		ID: "alerting:writer", Group: "Alerting", Access: AccessWrite,
		Actions: []Action{ActionAlertRead, ActionAlertWrite},
	},
	{
		ID: "updates:reader", Group: "Updates", Access: AccessRead,
		Actions: []Action{ActionUpdateRead},
	},
	{
		ID: "updates:writer", Group: "Updates", Access: AccessWrite,
		Actions: []Action{ActionUpdateRead, ActionUpdateWrite},
	},
	{
		ID: "annotations:reader", Group: "Annotations", Access: AccessRead,
		Actions: []Action{ActionAnnotationsRead},
	},
	{
		ID: "annotations:writer", Group: "Annotations", Access: AccessWrite,
		Actions: []Action{ActionAnnotationsRead, ActionAnnotationsWrite},
	},
	{
		ID: "settings:reader", Group: "Settings", Access: AccessRead,
		Actions: []Action{ActionSettingsRead, ActionConfigVersionsRead},
	},
	{
		ID: "settings:writer", Group: "Settings", Access: AccessWrite,
		Actions: []Action{ActionSettingsRead, ActionConfigVersionsRead, ActionSettingsWrite, ActionConfigVersionsWrite},
	},
	{
		ID: "members:reader", Group: "User administration", Access: AccessRead,
		Actions: []Action{ActionMembersRead},
	},
	{
		ID: "members:writer", Group: "User administration", Access: AccessWrite,
		Actions: []Action{ActionMembersRead, ActionMembersWrite},
	},
	{
		ID: "notifications:reader", Group: "Notifications", Access: AccessRead,
		Actions: []Action{ActionNotificationsRead},
	},
	{
		ID: "notifications:writer", Group: "Notifications", Access: AccessWrite,
		Actions: []Action{ActionNotificationsRead, ActionNotificationsWrite},
	},
}

// fixedRolesByID indexes the catalog once.
var fixedRolesByID = func() map[string]FixedRole {
	m := make(map[string]FixedRole, len(fixedRoleCatalog))
	for _, r := range fixedRoleCatalog {
		m[r.ID] = r
	}
	return m
}()

// FixedRoleByID returns a fixed role and whether it exists.
func FixedRoleByID(id string) (FixedRole, bool) {
	r, ok := fixedRolesByID[id]
	return r, ok
}

// FixedRoles returns the catalog, ordered by group then id, for the role picker.
func FixedRoles() []FixedRole {
	out := append([]FixedRole(nil), fixedRoleCatalog...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// FixedRoleGrant is one fixed role at one breadth — the unit a token stores.
type FixedRoleGrant struct {
	Role  string         `json:"role"`
	Scope FixedRoleScope `json:"scope"`
}

// IsValidFixedRoleGrant reports whether the role exists and the scope is known.
func IsValidFixedRoleGrant(g FixedRoleGrant) bool {
	if _, ok := FixedRoleByID(g.Role); !ok {
		return false
	}
	return g.Scope == ScopeOrgFixed || g.Scope == ScopeAllFixed
}

// ScopedPermissions expands a grant into concrete permissions.
//
// Each action is granted on its own resource prefix. Breadth changes which
// tenants the grant reaches, not the resource: scoping a device action to org:*
// would make it unmatchable.
func (g FixedRoleGrant) ScopedPermissions() ScopedPermissions {
	role, ok := FixedRoleByID(g.Role)
	if !ok {
		return nil
	}

	out := make(ScopedPermissions, 0, len(role.Actions))
	for _, action := range role.Actions {
		prefix := RequiredScope(action)
		if prefix == "" {
			continue
		}
		out = append(out, ScopedPermission{Action: action, Scope: WildcardScope(prefix)})
	}
	return out
}

// ValidOrgRole reports whether name is one of the organisation roles a token may
// act as. A token cannot be a super_admin: that role's extra authority is
// instance-scoped, which a bearer credential must never hold.
func ValidOrgRole(name string) bool {
	switch name {
	case "viewer", "operator", "admin":
		return true
	default:
		return false
	}
}

// OrgRoles returns the roles a token may be assigned, for the role picker.
func OrgRoles() []string {
	return []string{"viewer", "operator", "admin"}
}

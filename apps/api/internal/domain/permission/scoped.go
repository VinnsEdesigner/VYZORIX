// Package permission provides scoped, resource-aware authorization. A
// permission is an action granted on a resource scope, where the scope is a
// hierarchical resource path (for example "devices:imei:<id>") that may end in
// a trailing wildcard ("devices:*"). Evaluation matches an operator's action
// and scope against its granted permissions rather than relying on role tier
// alone. This is the scoped-RBAC model the coarse role-bundle replaced.
package permission

import "strings"

// Action is a resource-scoped operation, for example "device.command".
type Action string

const (
	// Device resource actions.
	ActionDeviceRead   Action = "device.read"
	ActionDeviceWrite  Action = "device.write"
	ActionDeviceDelete Action = "device.delete"
	ActionDeviceAssign Action = "device.assign"

	// Command execution on a device.
	ActionCommand Action = "command.execute"

	// Interactive shell access to a device. Deliberately separate from
	// ActionCommand: a shell is arbitrary, stateful code execution, so it is
	// granted on its own and never implied by command.execute.
	ActionShell Action = "shell.execute"

	// USB session access: read, write and flash an external device over OTG.
	// Separate from shell.execute because the two are different capabilities;
	// holding one must not imply the other.
	ActionUsb Action = "usb.attach"

	// Supervised applications on a device: list them, and start/stop them.
	ActionAppsRead  Action = "apps.read"
	ActionAppsWrite Action = "apps.write"

	// Declarative deployment of what runs on a device. Deliberately separate
	// from apps.read/apps.write and in no action set: deploy changes what code
	// runs, so the grant that lets an operator observe an app must not also let
	// them install one. Granted explicitly, or not at all.
	ActionDeviceDeploy Action = "device.deploy"

	// Telemetry: query device telemetry history, latest, stats, export.
	ActionTelemetryRead  Action = "telemetry.read"
	ActionTelemetryWrite Action = "telemetry.write"

	// Diagnostics: inspect device state and view diagnostic timelines.
	ActionDiagnosticsRead Action = "diagnostics.read"

	// Device events and logs: view the event/log timeline for a device.
	ActionEventsRead Action = "events.read"
	ActionLogsRead   Action = "logs.read"

	// Connection status: view device WebSocket/FCM connection state and metrics.
	ActionConnectionsRead Action = "connections.read"

	// Cluster stats: aggregate dashboard/cluster health and utilization metrics.
	ActionStatsRead  Action = "stats.read"
	ActionSearchRead Action = "search.read"

	// Alerting: view and manage org-scoped alert rules.
	ActionAlertRead  Action = "alert.read"
	ActionAlertWrite Action = "alert.write"

	// Notifications: manage contact points and view delivery log.
	ActionNotificationsRead  Action = "notifications.read"
	ActionNotificationsWrite Action = "notifications.write"

	// Service accounts: manage automation identities and their tokens.
	ActionServiceAccountsRead  Action = "service_accounts.read"
	ActionServiceAccountsWrite Action = "service_accounts.write"

	// Annotations: mark fleet timeline events.
	ActionAnnotationsRead  Action = "annotations.read"
	ActionAnnotationsWrite Action = "annotations.write"

	// Config versions: view and restore org config history.
	ActionConfigVersionsRead  Action = "config_versions.read"
	ActionConfigVersionsWrite Action = "config_versions.write"

	// Device inbox: view/approve device registration inbox entries.
	ActionInboxRead  Action = "inbox.read"
	ActionInboxWrite Action = "inbox.write"

	// Update management actions.
	ActionUpdateRead   Action = "update.read"
	ActionUpdateWrite  Action = "update.write"
	ActionUpdateDelete Action = "update.delete"

	// Settings, keys, members, audit, and admin actions.
	ActionSettingsRead     Action = "settings.read"
	ActionSettingsWrite    Action = "settings.write"
	ActionMembersRead      Action = "members.read"
	ActionMembersWrite     Action = "members.write"
	ActionMembersDelete    Action = "members.delete"
	ActionAuditRead        Action = "audit.read"
	ActionKeysManage       Action = "keys.manage"
	ActionAdminLockout     Action = "admin.lockout"
	ActionAdminImpersonate Action = "admin.impersonate"
)

// Well-known resource scope prefixes. A scope is a sequence of `prefix:id`
// segments optionally ending with a trailing wildcard `:*`.
const (
	ScopeDevices     = "devices"
	ScopeOrg         = "org"
	ScopeUpdates     = "updates"
	ScopeTelemetry   = "telemetry"
	ScopeDiagnostics = "diagnostics"
	ScopeInbox       = "inbox"
)

// ScopedPermission is one action granted on one resource scope.
type ScopedPermission struct {
	Action Action
	Scope  string
}

// ScopedPermissions is the set an operator may act with in an organization.
type ScopedPermissions []ScopedPermission

// Grants reports whether the set grants `action` on the resource `scope`. A
// grant matches when the requested scope equals the granted scope or sits under
// the granted scope's trailing wildcard.
func (s ScopedPermissions) Grants(action Action, scope string) bool {
	for _, p := range s {
		if p.Action == action && MatchScope(p.Scope, scope) {
			return true
		}
	}
	return false
}

// MatchScope reports whether a granted scope covers a target scope. A trailing
// ":*" in the grant is a prefix wildcard, so "devices:*" covers every device.
// Wildcards anywhere else are invalid and never match (fail closed).
func MatchScope(grant, target string) bool {
	if grant == "" || target == "" {
		return false
	}
	if !ValidScope(grant) || !ValidScope(target) {
		return false
	}
	if grant == target {
		return true
	}
	// Trailing wildcard: match as a prefix.
	if strings.HasSuffix(grant, ":*") {
		prefix := grant[:len(grant)-1] // Strip the wildcard itself.
		return strings.HasPrefix(target, prefix)
	}
	return false
}

// ValidScope reports whether a scope string is well-formed: non-empty segments,
// and a wildcard only permitted as the final character. Wildcards in any other
// position or a bare empty segment make the scope invalid.
func ValidScope(scope string) bool {
	if scope == "" {
		return false
	}
	star := strings.Index(scope, "*")
	if star >= 0 && star != len(scope)-1 {
		return false // Wildcard only allowed as the trailing character.
	}
	for _, seg := range strings.Split(scope, ":") {
		if seg == "" {
			return false
		}
	}
	return true
}

// BuildScope joins a resource prefix and id into a scope string.
func BuildScope(prefix, id string) string {
	if id == "" {
		return prefix
	}
	return prefix + ":" + id
}

// WildcardScope returns the wildcard form of a resource prefix, e.g. "devices:*".
func WildcardScope(prefix string) string {
	return prefix + ":*"
}

// Capability tags — the second authorization axis.
//
// The scoped engine above answers "on which resource may this operator act?".
// Tags answer a different question: "does this role hold the capability an API
// requires?". The two compose; neither implies the other. A capability tag is
// written "dimension:value" and carries no resource identity — the resource axis
// stays in Scope, so there is exactly one source of truth for it.

// Tag is one capability attribute of the form "dimension:value".
type Tag string

const (
	// Data sensitivity.
	DataPublic   Tag = "data:public"
	DataApp      Tag = "data:app"
	DataHardware Tag = "data:hardware"
	DataPrivate  Tag = "data:private"

	// Operation.
	OpRead  Tag = "op:read"
	OpWrite Tag = "op:write"

	// Android permission class the API needs at runtime.
	ProtectionNormal    Tag = "protection:normal"
	ProtectionDangerous Tag = "protection:dangerous"
	ProtectionSignature Tag = "protection:signature"

	// Who may see the result.
	VisibilityPublic  Tag = "visibility:public"
	VisibilityPrivate Tag = "visibility:private"

	// Hardware touched. Only HwUsb exists because only USB is in the catalog;
	// camera/location/mic are added with their APIs, not before. A tag no
	// requirement names is inert — it would make a role look like it holds a
	// capability that no check consults.
	HwUsb Tag = "hw:usb"
)

// Tier is a device's state capability level, read from the device record and
// never asserted by a caller. An unrooted device can never be handed an API that
// requires a higher tier, because resolution intersects on MinTier.
type Tier int

const (
	// TierUnrooted is a stock device: app-private storage and the UsbManager path.
	TierUnrooted Tier = 1
	// TierRooted adds direct device-node access (/dev/bus/usb) and RW external
	// storage.
	TierRooted Tier = 2
	// TierAdvanced is unrestricted (the highest state): full filesystem and device access.
	TierAdvanced Tier = 3
)

// TagSet is an unordered set of capability tags held by a role, a path, or a
// requirement check.
type TagSet []Tag

// AllTags is every capability tag the system defines. It exists so a test can
// assert each one is actually required by a catalog entry: a tag nothing names
// is inert, and it makes a role look like it holds a capability no check
// consults.
var AllTags = []Tag{
	DataPublic, DataApp, DataHardware, DataPrivate,
	OpRead, OpWrite,
	ProtectionNormal, ProtectionDangerous, ProtectionSignature,
	VisibilityPublic, VisibilityPrivate,
	HwUsb,
}

// Has reports whether the set contains t.
func (s TagSet) Has(t Tag) bool {
	for _, have := range s {
		if have == t {
			return true
		}
	}
	return false
}

// Requirement is a capability requirement expressed as three tag sets. All must
// be held, at least one of Any must be held, and none of Deny may be held. Any
// and Deny are what a flat conjunction cannot express: UsbApi works on tier 1 OR
// tier 2 (Any), and a deny makes a boundary readable without reading the
// complement of a grant.
type Requirement struct {
	All  []Tag
	Any  []Tag
	Deny []Tag
}

// SatisfiedBy reports whether grant holds every tag the requirement demands.
//
// Fail-closed: an empty requirement is satisfied by an empty grant (an API that
// needs nothing needs nothing), but a requirement with tags is never satisfied by
// a grant that lacks them. There is deliberately no "empty grant means
// everything" case — that inversion is what made the earlier sketch fail open.
func (r Requirement) SatisfiedBy(grant TagSet) bool {
	for _, t := range r.All {
		if !grant.Has(t) {
			return false
		}
	}
	if len(r.Any) > 0 {
		found := false
		for _, t := range r.Any {
			if grant.Has(t) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, t := range r.Deny {
		if grant.Has(t) {
			return false
		}
	}
	return true
}

// The filesystem is NOT modelled here.
//
// An earlier revision of this package carried a PathPolicy/PathBindable model
// with fs:* tags, mirroring the device's bind rules. It was removed: the server
// has no filesystem, so it could not enforce any of it, and nothing consumed it.
// It had also drifted — it listed `rootfs`, `overlay` and `android.sock` as an
// "sandbox zone" to protect, when the rootfs is the container's own root and the
// socket is the door exists to open.
//
// Bind rules are a device concern and live in exactly one place:
// `ProotLauncher.binds()` and `ProotLauncher.isSandboxZonePath()` in the Android
// repo, where the closed-world list and the sandbox-zone assertion actually run.
// Duplicating them here would give two sources of truth for one boundary.

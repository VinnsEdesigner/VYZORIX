package permission

// Action sets are aggregate permissions that expand into base actions. An admin
// can grant "device.manage" to a team rather than listing device.read,
// device.write, device.delete, device.assign individually. ExpandActionSets
// expands any aggregate action in a permission set into its members before
// evaluation, so a grant of an action set satisfies a check for any of its
// base actions before evaluation.

// Aggregate action names. These are intentionally distinct from the base
// Action constants so a grant of an aggregate is recognizable as such.
const (
	ActionSetDeviceManage    Action = "device.manage"
	ActionSetUpdateManage    Action = "update.manage"
	ActionSetMembersManage   Action = "members.manage"
	ActionSetSettingsManage  Action = "settings.manage"
	ActionSetOrgAdmin        Action = "org.admin"
	ActionSetCommandManage   Action = "command.manage"
	ActionSetTelemetryManage Action = "telemetry.manage"
	ActionSetObservability   Action = "observability"
	ActionSetInboxManage     Action = "inbox.manage"
	ActionSetShellManage     Action = "shell.manage"
	ActionSetUsbManage       Action = "usb.manage"
	ActionSetAppsManage      Action = "apps.manage"
)

// actionSets maps an aggregate action to the base actions it implies.
var actionSets = map[Action][]Action{
	ActionSetDeviceManage:   {ActionDeviceRead, ActionDeviceWrite, ActionDeviceDelete, ActionDeviceAssign},
	ActionSetUpdateManage:   {ActionUpdateRead, ActionUpdateWrite, ActionUpdateDelete},
	ActionSetMembersManage:  {ActionMembersRead, ActionMembersWrite, ActionMembersDelete},
	ActionSetSettingsManage: {ActionSettingsRead, ActionSettingsWrite},
	ActionSetOrgAdmin: {
		ActionSettingsRead, ActionSettingsWrite,
		ActionMembersRead, ActionMembersWrite, ActionMembersDelete,
		ActionAuditRead, ActionKeysManage,
	},
	ActionSetCommandManage: {ActionCommand},
	// Shell access is its own aggregate so it can be granted without also
	// granting the ability to dispatch arbitrary commands, and vice versa.
	ActionSetShellManage:     {ActionShell},
	ActionSetUsbManage:       {ActionUsb},
	ActionSetAppsManage:      {ActionAppsRead, ActionAppsWrite},
	ActionSetTelemetryManage: {ActionTelemetryRead, ActionTelemetryWrite},
	ActionSetObservability: {
		ActionTelemetryRead, ActionDiagnosticsRead,
		ActionEventsRead, ActionLogsRead,
		ActionConnectionsRead, ActionStatsRead,
		ActionAlertRead,
	},
	ActionSetInboxManage: {ActionInboxRead, ActionInboxWrite},
}

// IsActionSet reports whether an action is an aggregate action set.
func IsActionSet(a Action) bool {
	_, ok := actionSets[a]
	return ok
}

// ExpandAction returns the base actions an action expands to. A base action
// expands to itself; an aggregate expands to its members.
func ExpandAction(a Action) []Action {
	if members, ok := actionSets[a]; ok {
		return members
	}
	return []Action{a}
}

// ExpandActionSets returns the permission set with every aggregate action
// expanded into its base members. The result contains only base actions; an
// aggregate grant therefore satisfies a check for any of its members.
func ExpandActionSets(perms ScopedPermissions) ScopedPermissions {
	if len(perms) == 0 {
		return perms
	}
	seen := make(map[ScopedPermission]struct{}, len(perms))
	out := make(ScopedPermissions, 0, len(perms))
	for _, p := range perms {
		for _, base := range ExpandAction(p.Action) {
			sp := ScopedPermission{Action: base, Scope: p.Scope}
			if _, ok := seen[sp]; !ok {
				seen[sp] = struct{}{}
				out = append(out, sp)
			}
		}
	}
	return out
}

// ActionSets returns the catalog of aggregate actions and their members, for
// introspection (e.g. an admin API listing grantable action sets).
func ActionSets() map[Action][]Action {
	out := make(map[Action][]Action, len(actionSets))
	for k, v := range actionSets {
		out[k] = append([]Action(nil), v...)
	}
	return out
}

// Device API catalog — the single source of truth for what each device API
// requires. One entry per API; nothing else in the system repeats it. The
// device's @VyzorixApi annotation, where one exists, is documentation only and is
// never consulted, so it cannot drift into authority.
//
// Each entry carries both axes: the capability Requirement (does the role hold
// the tags?) and the scoped Action+Scope (on which resource may it act?).
// Resolution is ResolveSessionCapability in grants.go.

// ApiRequirement is the requirement for one device API.
type ApiRequirement struct {
	// Action is the resource action evaluated against the scoped engine, e.g.
	// ActionDeviceRead for a read-only device API.
	Action Action
	// Scope is the resource scope prefix, e.g. ScopeDevices. The concrete scope
	// (devices:imei:<id>) is built by the caller from the target device.
	Scope string
	// Tags is the capability requirement.
	Tags Requirement
	// MinTier is the lowest device tier that can serve the API. Resolution
	// intersects on it, so an unrooted device never receives a root-only API.
	MinTier Tier
}

// DeviceAPICatalog maps an API name to its requirement. This is the authority;
// adding an API is one entry here plus its handler.
var DeviceAPICatalog = map[string]ApiRequirement{
	// Battery: no runtime permission, public read.
	"battery": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	// Device identity and platform facts: public read.
	"device_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	// Internal storage capacity: app-visible data, read.
	"storage": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	// Connectivity state: public read.
	"network": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	// Recent logcat: private, readable by user and above.
	"logcat": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	// Session open/close/resize: a shell is arbitrary code execution, so it is
	// its own capability and never implied by command.execute.
	"session": {
		Action:  ActionShell,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpWrite, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	// USB: reachable two ways. Unrooted (tier 1) through UsbManager and a handed
	// fd; rooted (tier 2) through the device nodes directly. Any expresses the
	// alternative a flat conjunction cannot, and MinTier stays 1 because the
	// UsbManager path serves an unrooted device.
	"usb": {
		Action:  ActionUsb,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{HwUsb, DataHardware, OpWrite, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	// USB device-node access: rooted devices only. This is the tier-2-only entry —
	// an unrooted device can never be handed it, because resolution intersects on
	// MinTier.
	"usb_nodes": {
		Action:  ActionUsb,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{HwUsb, DataHardware, OpWrite, ProtectionSignature}},
		MinTier: TierRooted,
	},
	// The concrete USB APIs the device serves. Each is read-or-write as the device
	// declares it: usb_state reads the attachment list, usb_attach/usb_detach claim
	// and release a device (the write half). All three carry the hardware tags, so
	// the role that holds usb.manage reaches them and no other role does.
	"usb_state": {
		Action:  ActionUsb,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{HwUsb, DataHardware, OpRead, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"usb_attach": {
		Action:  ActionUsb,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{HwUsb, DataHardware, OpWrite, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"usb_detach": {
		Action:  ActionUsb,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{HwUsb, DataHardware, OpWrite, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	// Full filesystem read: advanced devices only.
	"filesystem": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, VisibilityPrivate}},
		MinTier: TierAdvanced,
	},
	// --- The device's own API surface, mirroring android-core's ApiCatalogue. ---
	// Each entry's tags are the device handler's declared @VyzorixApi tags, so the
	// server's decision and the device's classification cannot drift. A read is
	// device.read; a write is command.execute (the action the scoped engine gates).
	"accessibility": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpWrite, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"airplane_mode": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"app_ops": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"app_storage": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"audio_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"battery_history": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"bluetooth_control": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataHardware, OpWrite, ProtectionDangerous, VisibilityPrivate, HwBluetooth}},
		MinTier: TierUnrooted,
	},
	"bluetooth_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"brightness": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"build_fingerprint": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"call_capability": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"call_control": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpWrite, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"camera_list": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"clipboard_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"container_status": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"cpu_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"default_apps": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"developer_options": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"device_admin_action": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpWrite, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"device_admin_request": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpWrite, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"device_credentials": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpWrite, ProtectionSignature, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"device_owner": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"device_owner_capabilities": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"display_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"dns_servers": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"do_not_disturb": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"escalation_recon": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"external_storage": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"file_descriptors": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"font_scale": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"haptic_settings": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"input_devices": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"input_method": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"installed_apps": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"kernel_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"load_average": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"locale_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"location": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"memory_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"mount_points": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"network_interfaces": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"nfc_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"notification_policy": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"package_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"packages_dump": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"phone_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"phone_state_detailed": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"power_save": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"process_list": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"proxy_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"rotation_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"screen_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"screen_timeout": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"security_patch": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"sensor_list": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"sms_capability": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"sms_read": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"sms_received": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"storage_all_files": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionDangerous, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"storage_volumes": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"system_properties": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"systemui_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"thermal": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"time_info": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"usb_control": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataHardware, OpWrite, ProtectionSignature, VisibilityPrivate, HwUsb}},
		MinTier: TierUnrooted,
	},
	"user_restrictions": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPublic, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"vpn_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"wallpaper": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpWrite, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"wifi_control": {
		Action:  ActionCommand,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataHardware, OpWrite, ProtectionDangerous, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
	"wifi_state": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataApp, OpRead, ProtectionNormal, VisibilityPublic}},
		MinTier: TierUnrooted,
	},
	"wireless_debug": {
		Action:  ActionDeviceRead,
		Scope:   ScopeDevices,
		Tags:    Requirement{All: []Tag{DataPrivate, OpRead, ProtectionNormal, VisibilityPrivate}},
		MinTier: TierUnrooted,
	},
}

// ApiRequirementFor returns the requirement for an API and whether it is known.
// An unknown API is not in the catalog and therefore never resolvable — the
// fail-closed default.
func ApiRequirementFor(api string) (ApiRequirement, bool) {
	req, ok := DeviceAPICatalog[api]
	return req, ok
}

// Resolution lives in ResolveSessionCapability (grants.go). There is deliberately
// no second resolver here: an earlier revision had one that applied tags and tier
// but not the resource axis, which meant two functions could disagree about the
// same session. One resolver, one answer.

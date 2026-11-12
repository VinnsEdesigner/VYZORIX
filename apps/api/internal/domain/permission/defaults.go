package permission

// DefaultScopesForRole returns the scoped permissions a role grants by default.
// Mapping is by role name so this package stays dependency-free; the caller
// passes membership.Role (or another role string). Custom grants are added on
// top of these defaults by the operator's permission evaluator.
func DefaultScopesForRole(roleName string) ScopedPermissions {
	devicesAll := WildcardScope(ScopeDevices)
	updatesAll := WildcardScope(ScopeUpdates)
	orgAll := WildcardScope(ScopeOrg)
	telemetryAll := WildcardScope(ScopeTelemetry)
	diagnosticsAll := WildcardScope(ScopeDiagnostics)
	inboxAll := WildcardScope(ScopeInbox)

	var perms ScopedPermissions

	appendViewer := func() {
		perms = ScopedPermissions{
			{ActionDeviceRead, devicesAll},
			{ActionUpdateRead, updatesAll},
			{ActionSettingsRead, orgAll},
			{ActionTelemetryRead, telemetryAll},
			{ActionDiagnosticsRead, diagnosticsAll},
			{ActionEventsRead, devicesAll},
			{ActionLogsRead, devicesAll},
			{ActionConnectionsRead, devicesAll},
			{ActionStatsRead, orgAll},
			{ActionSearchRead, orgAll},
			{ActionAlertRead, orgAll},
			{ActionNotificationsRead, orgAll},
			{ActionServiceAccountsRead, orgAll},
			{ActionAnnotationsRead, orgAll},
			{ActionConfigVersionsRead, orgAll},
			// Visibility into supervised apps; starting/stopping them is an
			// operator action.
			{ActionAppsRead, devicesAll},
		}
	}
	appendOperator := func() {
		perms = append(perms,
			ScopedPermission{ActionDeviceWrite, devicesAll},
			ScopedPermission{ActionCommand, devicesAll},
			ScopedPermission{ActionUpdateRead, updatesAll},
			ScopedPermission{ActionTelemetryRead, telemetryAll},
			ScopedPermission{ActionDiagnosticsRead, diagnosticsAll},
			ScopedPermission{ActionInboxRead, inboxAll},
			ScopedPermission{ActionInboxWrite, inboxAll},
			// App lifecycle is routine operations work. Shell and USB are
			// deliberately absent: both are arbitrary device access, granted to
			// admins only.
			ScopedPermission{ActionAppsWrite, devicesAll},
		)
	}
	appendAdmin := func() {
		perms = append(perms,
			ScopedPermission{ActionDeviceDelete, devicesAll},
			ScopedPermission{ActionDeviceAssign, devicesAll},
			ScopedPermission{ActionUpdateWrite, updatesAll},
			ScopedPermission{ActionUpdateDelete, updatesAll},
			ScopedPermission{ActionSettingsWrite, orgAll},
			ScopedPermission{ActionKeysManage, orgAll},
			ScopedPermission{ActionAuditRead, orgAll},
			ScopedPermission{ActionTelemetryWrite, telemetryAll},
			ScopedPermission{ActionMembersRead, orgAll},
			ScopedPermission{ActionMembersWrite, orgAll},
			ScopedPermission{ActionMembersDelete, orgAll},
			ScopedPermission{ActionAlertWrite, orgAll},
			ScopedPermission{ActionNotificationsWrite, orgAll},
			ScopedPermission{ActionServiceAccountsWrite, orgAll},
			ScopedPermission{ActionAnnotationsWrite, orgAll},
			ScopedPermission{ActionConfigVersionsWrite, orgAll},
			// Arbitrary device access. A shell is unbounded code execution and a
			// USB attach can flash external hardware, so both are admin-only and
			// never implied by the operator defaults above.
			ScopedPermission{ActionShell, devicesAll},
			ScopedPermission{ActionUsb, devicesAll},
		)
	}
	appendSuperAdmin := func() {
		perms = append(perms,
			ScopedPermission{ActionAdminLockout, orgAll},
			ScopedPermission{ActionAdminImpersonate, orgAll},
		)
	}

	switch roleName {
	case "viewer":
		appendViewer()
	case "operator":
		appendViewer()
		appendOperator()
	case "admin":
		appendViewer()
		appendOperator()
		appendAdmin()
	case "super_admin":
		appendViewer()
		appendOperator()
		appendAdmin()
		appendSuperAdmin()
	default:
		// Unknown role: fall back to viewer-only (fail to the least privilege).
		appendViewer()
	}

	return perms
}

// DefaultTagsForRole returns the capability tags a role holds by default. It is
// the tag-axis counterpart of DefaultScopesForRole: the scoped permissions answer
// "on which resource may this role act", the tags answer "does this role hold the
// capability an API requires". Both are consulted; neither implies the other.
//
// The role ladder is the same four roles, each a strict superset of the one
// below, so a viewer can never satisfy a requirement an operator cannot.
//
// Fail-closed: an unknown role falls back to the viewer set, mirroring
// DefaultScopesForRole.
func DefaultTagsForRole(roleName string) TagSet {
	viewer := TagSet{
		DataPublic,
		OpRead,
		ProtectionNormal,
		VisibilityPublic,
	}

	operator := append(append(TagSet{}, viewer...),
		DataApp,
		DataPrivate,
		VisibilityPrivate,
	)

	admin := append(append(TagSet{}, operator...),
		OpWrite,
		ProtectionDangerous,
		ProtectionSignature,
		DataHardware,
		HwUsb,
		HwBluetooth,
	)

	// super_admin holds the same tags as admin. That is not an oversight: the tag
	// axis carries capability dimensions (data, op, protection, visibility,
	// hardware), and super_admin's extra authority is org-admin *actions*
	// (admin.lockout, admin.impersonate), which the scoped engine grants and this
	// axis does not model.

	switch roleName {
	case "viewer":
		return viewer
	case "operator":
		return operator
	case "admin":
		return admin
	case "super_admin":
		return admin
	default:
		return viewer
	}
}

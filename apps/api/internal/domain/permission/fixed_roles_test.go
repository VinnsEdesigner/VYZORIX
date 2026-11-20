package permission

import "testing"

// TestFixedRoleCatalogShape pins the Grafana-derived matrix: every fixed role
// exists as a reader and a writer, and each may be granted at both breadths.
func TestFixedRoleCatalogShape(t *testing.T) {
	groups := map[string]map[AccessLevel]bool{}
	for _, role := range FixedRoles() {
		if groups[role.Group] == nil {
			groups[role.Group] = map[AccessLevel]bool{}
		}
		groups[role.Group][role.Access] = true
	}

	for group, levels := range groups {
		if !levels[AccessRead] {
			t.Errorf("group %q has no reader role", group)
		}
		if !levels[AccessWrite] {
			t.Errorf("group %q has no writer role", group)
		}
	}
}

// TestWriterSupersedesReader: a writer role must grant everything its reader
// does, so promoting a token never silently removes access.
func TestWriterSupersedesReader(t *testing.T) {
	for _, role := range FixedRoles() {
		if role.Access != AccessWrite {
			continue
		}
		readerID := role.Group + ":reader"
		// Find the reader in the same group by comparing action coverage instead
		// of trusting an id convention.
		var reader *FixedRole
		for i := range fixedRoleCatalog {
			candidate := &fixedRoleCatalog[i]
			if candidate.Group == role.Group && candidate.Access == AccessRead {
				reader = candidate
				break
			}
		}
		if reader == nil {
			t.Errorf("group %q has a writer (%s) but no reader (%s)", role.Group, role.ID, readerID)
			continue
		}
		writerActions := map[Action]bool{}
		for _, a := range role.Actions {
			writerActions[a] = true
		}
		for _, a := range reader.Actions {
			if !writerActions[a] {
				t.Errorf("%s is missing reader action %s held by %s", role.ID, a, reader.ID)
			}
		}
	}
}

// TestFixedRoleGrantExpansion checks that a grant expands to its actions, and
// that an unknown role or scope expands to nothing (fail closed).
func TestFixedRoleGrantExpansion(t *testing.T) {
	writer := FixedRoleGrant{Role: "devices:writer", Scope: ScopeOrgFixed}
	perms := writer.ScopedPermissions()
	if len(perms) == 0 {
		t.Fatal("devices:writer expanded to nothing")
	}
	if !perms.Grants(ActionCommand, WildcardScope(ScopeDevices)) {
		t.Error("devices:writer should grant command.execute on devices")
	}
	if perms.Grants(ActionCommand, WildcardScope(ScopeOrg)) {
		t.Error("a devices action must not be granted at org scope")
	}

	if got := (FixedRoleGrant{Role: "nope:nope", Scope: ScopeOrgFixed}).ScopedPermissions(); len(got) != 0 {
		t.Errorf("unknown role expanded to %d permissions, want 0", len(got))
	}

	if IsValidFixedRoleGrant(FixedRoleGrant{Role: "devices:reader", Scope: "bogus"}) {
		t.Error("an unknown scope must be invalid")
	}
	if !IsValidFixedRoleGrant(FixedRoleGrant{Role: "devices:reader", Scope: ScopeAllFixed}) {
		t.Error("a known role at 'all' should be valid")
	}
}

// TestTokenCannotBeSuperAdmin: a bearer credential must never carry the
// instance-scoped super_admin authority.
func TestTokenCannotBeSuperAdmin(t *testing.T) {
	if ValidOrgRole("super_admin") {
		t.Error("a service account token must not be assignable super_admin")
	}
	for _, role := range OrgRoles() {
		if !ValidOrgRole(role) {
			t.Errorf("OrgRoles lists %q but ValidOrgRole rejects it", role)
		}
	}
}

// TestEveryFixedRoleActionHasScope guards the expansion: an action without a
// scope mapping would silently drop out of a role's permissions.
func TestEveryFixedRoleActionHasScope(t *testing.T) {
	for _, role := range FixedRoles() {
		for _, action := range role.Actions {
			if RequiredScope(action) == "" {
				t.Errorf("role %s grants %s, which has no scope mapping", role.ID, action)
			}
			if _, ok := actionAccess[action]; !ok {
				t.Errorf("role %s grants %s, which has no access level", role.ID, action)
			}
		}
	}
}

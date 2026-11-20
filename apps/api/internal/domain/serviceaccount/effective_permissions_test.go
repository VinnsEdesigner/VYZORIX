package serviceaccount

import (
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// TestEffectivePermissions_RolesAreTheOnlySource pins the permission contract:
// a token's reach comes from its basic role and fixed roles, and the coarse
// Scopes field grants nothing.
//
// This is a regression guard. Scopes used to be a second permission source, so a
// token created with roles alone still received every org-scoped read action
// from a defaulted "read" scope — a silent widening the UI could not show.
func TestEffectivePermissions_RolesAreTheOnlySource(t *testing.T) {
	org := permission.WildcardScope(permission.ScopeOrg)
	devices := permission.WildcardScope(permission.ScopeDevices)

	scopesOnly := &Token{Scopes: []string{"admin"}}
	perms := scopesOnly.EffectivePermissions()
	if len(perms) != 0 {
		t.Errorf("scopes must grant nothing, got %d permissions", len(perms))
	}
	if perms.Grants(permission.ActionSettingsWrite, org) {
		t.Error("an admin scope must not grant settings.write")
	}
	if perms.Grants(permission.ActionDeviceRead, devices) {
		t.Error("an admin scope must not grant device.read")
	}

	withRole := &Token{
		Scopes: []string{"read"},
		Roles:  []permission.FixedRoleGrant{{Role: "devices:writer", Scope: permission.ScopeOrgFixed}},
	}
	if !withRole.EffectivePermissions().Grants(permission.ActionCommand, devices) {
		t.Error("devices:writer must grant command.execute")
	}

	withBasic := &Token{BasicRole: "admin"}
	if !withBasic.EffectivePermissions().Grants(permission.ActionSettingsWrite, org) {
		t.Error("an admin basic role must grant settings.write")
	}

	if got := (&Token{}).EffectivePermissions(); len(got) != 0 {
		t.Errorf("a token with no roles must grant nothing, got %d", len(got))
	}
}

// TestEffectivePermissions_ReaderIsSubsetOfWriter: assigning a writer role must
// never remove access a reader already had.
func TestEffectivePermissions_ReaderIsSubsetOfWriter(t *testing.T) {
	devices := permission.WildcardScope(permission.ScopeDevices)
	reader := &Token{Roles: []permission.FixedRoleGrant{{Role: "devices:reader", Scope: permission.ScopeOrgFixed}}}
	writer := &Token{Roles: []permission.FixedRoleGrant{{Role: "devices:writer", Scope: permission.ScopeOrgFixed}}}

	if !reader.EffectivePermissions().Grants(permission.ActionDeviceRead, devices) {
		t.Fatal("devices:reader must grant device.read")
	}
	if !writer.EffectivePermissions().Grants(permission.ActionDeviceRead, devices) {
		t.Error("devices:writer must also grant device.read")
	}
	if reader.EffectivePermissions().Grants(permission.ActionCommand, devices) {
		t.Error("devices:reader must not grant command.execute")
	}
}

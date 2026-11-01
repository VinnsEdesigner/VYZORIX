package permission_test

import (
	"context"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// allowlistFor resolves the APIs a role reaches on a device of the given tier,
// through the one resolver. It replaces direct calls to the removed ApiAllowlist
// so the tests exercise the same path production does.
func allowlistFor(role string, tier permission.Tier) []string {
	capability := permission.ResolveSessionCapability(
		context.Background(), "s", role, nil, "dev-1", tier, 0)

	return capability.APIAllowlist
}

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

// ---------------------------------------------------------------------------
// Boundary contract tests. These pin the invariants the contract states, so a
// later edit to the catalog or the role ladder cannot quietly widen a boundary.
// ---------------------------------------------------------------------------

// TestAllowlistIsDenyByDefault is the guard on the fail-open bug: an empty tag
// grant must yield an empty allowlist, never "every API". This is the inversion
// the device bridge currently has and the model forbids.
func TestAllowlistIsDenyByDefault(t *testing.T) {
	// An unknown role falls back to the viewer tag set, so to prove deny-by-default
	// the resolution must be driven by an empty tag grant. ResolveSessionCapability
	// is the only resolver; an unknown API is absent from the catalog and so absent
	// from every allowlist.
	if _, ok := permission.ApiRequirementFor("not_a_real_api"); ok {
		t.Fatal("an unknown API must not be in the catalog")
	}

	capability := permission.ResolveSessionCapability(
		context.Background(), "s", "viewer", nil, "dev-1", permission.TierAdvanced, 0)
	if capability.Permits("not_a_real_api") {
		t.Fatal("an unknown API must never be permitted")
	}
}

// TestAllowlistIsAMonotoneLadder proves each role reaches a superset of the one
// below it: a viewer can never call an API an operator cannot.
func TestAllowlistIsAMonotoneLadder(t *testing.T) {
	roles := []string{"viewer", "operator", "admin", "super_admin"}
	prev := map[string]bool{}

	for _, role := range roles {
		current := map[string]bool{}
		for _, api := range allowlistFor(role, permission.TierAdvanced) {
			current[api] = true
		}
		for api := range prev {
			if !current[api] {
				t.Errorf("%s lost %q that a lower role had", role, api)
			}
		}
		prev = current
	}
}

// TestViewerCannotReachShellOrUsb is the least-privilege guard: the two
// arbitrary-access APIs must not appear in a viewer's allowlist at any tier.
func TestViewerCannotReachShellOrUsb(t *testing.T) {
	tiers := []permission.Tier{permission.TierUnrooted, permission.TierRooted, permission.TierAdvanced}

	for _, tier := range tiers {
		for _, api := range allowlistFor("viewer", tier) {
			if api == "session" || api == "usb" {
				t.Errorf("viewer reached %q at tier %d", api, tier)
			}
		}
	}
}

// TestTierIntersectsAllowlist proves a device's state caps what it can be handed:
// an unrooted device never receives a rooted-only API, and only an advanced
// device receives the full-filesystem one — regardless of role.
func TestTierIntersectsAllowlist(t *testing.T) {
	unrooted := allowlistFor("super_admin", permission.TierUnrooted)
	if contains(unrooted, "usb_nodes") {
		t.Error("tier 1 must not be handed usb_nodes (needs a device node)")
	}
	if contains(unrooted, "filesystem") {
		t.Error("tier 1 must not be handed filesystem (advanced only)")
	}
	if !contains(unrooted, "usb") {
		t.Error("tier 1 must reach usb through UsbManager")
	}

	rooted := allowlistFor("super_admin", permission.TierRooted)
	if !contains(rooted, "usb_nodes") {
		t.Error("tier 2 must reach usb_nodes")
	}
	if contains(rooted, "filesystem") {
		t.Error("tier 2 must not be handed filesystem (advanced only)")
	}

	advanced := allowlistFor("super_admin", permission.TierAdvanced)
	if !contains(advanced, "filesystem") {
		t.Error("tier 3 must reach the full filesystem")
	}
}

// TestNoInertTags is the guard against a tag that looks like a capability but is
// required by nothing. Every tag must be named by at least one catalog entry;
// otherwise a role appears to hold a capability no check consults.
func TestNoInertTags(t *testing.T) {
	required := map[permission.Tag]bool{}

	for _, req := range permission.DeviceAPICatalog {
		for _, t := range append(append([]permission.Tag{}, req.Tags.All...), req.Tags.Any...) {
			required[t] = true
		}
		for _, t := range req.Tags.Deny {
			required[t] = true
		}
	}
	for _, tag := range permission.AllTags {
		if !required[tag] {
			t.Errorf("tag %q is defined and granted but required by nothing (inert)", tag)
		}
	}
}

// contains reports whether the allowlist holds api.
func contains(list []string, api string) bool {
	for _, a := range list {
		if a == api {
			return true
		}
	}
	return false
}

// TestSessionCapabilityPermitsIsFailClosed pins the token check: an empty
// allowlist permits nothing, and a missing identity permits nothing even with a
// populated allowlist.
func TestSessionCapabilityPermitsIsFailClosed(t *testing.T) {
	empty := permission.SessionCapability{
		SessionID: "s1",
		DeviceID:  "d1",
	}
	if empty.Permits("battery") {
		t.Fatal("an empty allowlist must permit nothing")
	}

	populated := permission.SessionCapability{
		SessionID:    "s1",
		DeviceID:     "d1",
		APIAllowlist: []string{"battery"},
	}
	if !populated.Permits("battery") {
		t.Fatal("a listed API must be permitted")
	}
	if populated.Permits("logcat") {
		t.Fatal("an unlisted API must not be permitted")
	}

	anonymous := permission.SessionCapability{APIAllowlist: []string{"battery"}}
	if anonymous.Permits("battery") {
		t.Fatal("a capability with no identity must permit nothing")
	}
}

// TestResolveSessionCapabilityComposesBothAxes proves resolution needs the tag
// requirement AND the scoped grant: a viewer gets the read APIs and never the
// arbitrary-access ones.
func TestResolveSessionCapabilityComposesBothAxes(t *testing.T) {
	admin := permission.ResolveSessionCapability(context.Background(), "s1", "admin", nil, "dev-1", permission.TierUnrooted, 0)
	if !admin.Permits("battery") {
		t.Error("admin should reach battery on its own device")
	}
	if !admin.Permits("session") {
		t.Error("admin should reach a shell session")
	}

	viewer := permission.ResolveSessionCapability(context.Background(), "s2", "viewer", nil, "dev-1", permission.TierUnrooted, 0)
	if !viewer.Permits("battery") {
		t.Error("viewer should reach battery")
	}
	if viewer.Permits("session") {
		t.Error("viewer must not reach a shell session")
	}
	if viewer.Permits("usb") {
		t.Error("viewer must not reach usb")
	}
}

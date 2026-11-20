package api

import (
	"testing"

	authhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/auth"
	organizationhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"

	"github.com/gin-gonic/gin"
)

// userRouteServer registers the user/access route groups and returns the engine,
// so a test can enumerate the real route table rather than restate it.
func userRouteServer(t *testing.T) *gin.Engine {
	t.Helper()

	s := &Server{
		authHandlers:      &authhandlers.AllHandlers{Admin: &authhandlers.AdminHandler{}},
		memberHandler:     &organizationhandlers.MemberHandler{},
		invitationHandler: &organizationhandlers.InvitationHandler{},
	}

	r := gin.New()
	group := r.Group("/v1")

	s.setupOrganizationRoutes(group)
	s.setupInstanceRoutes(group)

	return r
}

// registeredRoutes renders the engine's route table as "METHOD /path" strings.
func registeredRoutes(r *gin.Engine) map[string]struct{} {
	out := make(map[string]struct{}, len(r.Routes()))
	for _, route := range r.Routes() {
		out[route.Method+" "+route.Path] = struct{}{}
	}

	return out
}

// TestDirectAccountCreationIsNotRoutable is the regression guard for the
// invite-only rule. Direct account creation bypassed the password policy and the
// breach check while the console told the user otherwise, and identity editing
// rewrote a login address with no re-verification. Neither may come back: an
// account joins an organisation through an invitation, and its email and password
// are self-service.
func TestDirectAccountCreationIsNotRoutable(t *testing.T) {
	routes := registeredRoutes(userRouteServer(t))

	for _, gone := range []string{
		"POST /v1/auth/admin/users",
		"PATCH /v1/auth/admin/users/:id",
		"POST /v1/auth/admin/users/:id/password",
		"GET /v1/auth/admin/users/:id/organizations",
	} {
		if _, ok := routes[gone]; ok {
			t.Errorf("%s must not be routable: direct account creation and identity editing are removed", gone)
		}
	}
}

// TestAccountStateIsInstanceScoped pins that disabling, enabling and deleting an
// account are deployment authority. Each reaches across every organisation the
// person belongs to, so an org role must not hold them; the org-local siblings are
// members.write and members.delete, which act on one membership.
func TestAccountStateIsInstanceScoped(t *testing.T) {
	s := registrationServer(t)
	guards := s.ScopeGuards()

	found := 0
	for _, g := range guards {
		if g.Action != permission.ActionAdminUsers {
			continue
		}
		found++
		if g.Store != guardStoreInstance {
			t.Errorf("admin.users is guarded on the %q store; it must be instance-scoped", g.Store)
		}
	}
	if found == 0 {
		t.Fatal("no admin.users guard registered: the account-state routes are unguarded")
	}

	// The org ladder must not satisfy it, or an org super_admin could disable a
	// user in another tenant.
	orgGrants := permission.DefaultScopesForRole("super_admin")
	if orgGrants.Grants(permission.ActionAdminUsers, permission.WildcardScope(permission.ScopeInstance)) {
		t.Error("an org role satisfies admin.users; account state is not deployment authority")
	}
}

// TestInvitationCreateIsScopedToMembersWrite is the regression guard for the
// ungated invite route. Adding an invitation adds a member, so it must require
// members.write on the current organisation like the sibling member routes —
// otherwise the only check is the service's role test, which no route audit sees.
func TestInvitationCreateIsScopedToMembersWrite(t *testing.T) {
	s := &Server{
		authHandlers:        &authhandlers.AllHandlers{Settings: &authhandlers.SettingsHandler{}},
		organizationHandler: &organizationhandlers.OrganizationHandler{},
		memberHandler:       &organizationhandlers.MemberHandler{},
		invitationHandler:   &organizationhandlers.InvitationHandler{},
	}

	s.setupOrganizationRoutes(gin.New().Group("/v1"))

	found := 0
	for _, g := range s.ScopeGuards() {
		if g.Action == permission.ActionMembersWrite && g.Store == guardStoreOrg {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected an org members.write guard on both the member and invitation routes, found %d", found)
	}
}

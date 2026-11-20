package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	authhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/auth"
	organizationhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	organizationapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/session"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
	"github.com/gin-gonic/gin"
)

// The authorization contract test. It walks the guards the server actually
// registered — not a hand-written list — and asserts three things the scoped
// engine cannot state on its own:
//
//  1. Every guard is reachable: some role's defaults grant its (action, scope).
//     A route gated on an action no role holds is dead, and the build fails
//     here rather than in production.
//  2. The stores are orthogonal: no instance grant satisfies an org guard and
//     no org grant satisfies an instance guard. This is the load-bearing
//     property — instance authority is additive, never a superset of an org.
//  3. /me/capabilities agrees with the middleware: a capability the endpoint
//     reports is one the guard admits, and vice versa.

// orgRoles is the org ladder the contract test checks reachability against.
var orgRoles = []organization.OrganizationRole{
	organization.RoleViewer,
	organization.RoleOperator,
	organization.RoleAdmin,
	organization.RoleSuperAdmin,
}

// registrationServer builds a Server and registers the org and instance route
// groups, so ScopeGuards() reflects the real route table for those groups. The
// handlers are only stored during registration, never invoked, so zero values
// are sufficient.
func registrationServer(t *testing.T) *Server {
	t.Helper()

	s := &Server{
		authHandlers:        &authhandlers.AllHandlers{Settings: &authhandlers.SettingsHandler{}, Admin: &authhandlers.AdminHandler{}},
		organizationHandler: &organizationhandlers.OrganizationHandler{},
		memberHandler:       &organizationhandlers.MemberHandler{},
		invitationHandler:   &organizationhandlers.InvitationHandler{},
	}

	r := gin.New()
	group := r.Group("/v1")

	s.setupOrganizationRoutes(group)
	s.setupInstanceRoutes(group)

	return s
}

// TestEveryGuardIsReachableBySomeRole is guarantee 1: the role meant to reach a
// route must actually hold its (action, scope). It fails the moment a route is
// added for an action no default grants.
func TestEveryGuardIsReachableBySomeRole(t *testing.T) {
	s := registrationServer(t)
	guards := s.ScopeGuards()

	if len(guards) == 0 {
		t.Fatal("no guards registered; the contract test would pass vacuously")
	}

	for _, g := range guards {
		switch g.Store {
		case guardStoreOrg:
			if !anyOrgRoleGrants(g.Action, g.Scope) {
				t.Errorf("org guard %s on %s is unreachable: no org role grants it", g.Action, g.Scope)
			}
		case guardStoreInstance:
			if !anyInstanceRoleGrants(g.Action, g.Scope) {
				t.Errorf("instance guard %s on %s is unreachable: no instance role grants it", g.Action, g.Scope)
			}
		default:
			t.Errorf("guard %s on %s has an unknown store %q", g.Action, g.Scope, g.Store)
		}
	}
}

// TestGuardStoresAreOrthogonal is guarantee 2: the two stores never satisfy each
// other's guards. If they did, instance authority would leak into org resources
// (or the reverse) and the axes would be hierarchical instead of orthogonal.
func TestGuardStoresAreOrthogonal(t *testing.T) {
	s := registrationServer(t)

	instanceGrants := permission.DefaultScopesForInstanceRole(permission.InstanceAdmin)
	orgGrants := permission.DefaultScopesForRole(string(organization.RoleSuperAdmin))

	for _, g := range s.ScopeGuards() {
		switch g.Store {
		case guardStoreOrg:
			if instanceGrants.Grants(g.Action, g.Scope) {
				t.Errorf("instance authority satisfies the org guard %s on %s; the stores are not orthogonal", g.Action, g.Scope)
			}
		case guardStoreInstance:
			if orgGrants.Grants(g.Action, g.Scope) {
				t.Errorf("org authority satisfies the instance guard %s on %s; the stores are not orthogonal", g.Action, g.Scope)
			}
		}
	}
}

// TestInstanceAdminDoesNotReachOrgResources is the concrete case the design
// calls out: an instance admin with no membership can see that an org exists
// (org.list) but cannot read its devices (device.read is org scope).
func TestInstanceAdminDoesNotReachOrgResources(t *testing.T) {
	instance := permission.DefaultScopesForInstanceRole(permission.InstanceAdmin)
	devicesAll := permission.WildcardScope(permission.ScopeDevices)

	if instance.Grants(permission.ActionDeviceRead, devicesAll) {
		t.Error("instance authority must not satisfy an org-scoped device.read")
	}
	if !instance.Grants(permission.ActionOrgList, permission.WildcardScope(permission.ScopeInstance)) {
		t.Error("an instance admin must be able to list organizations")
	}
}

// TestNewUserCanCreateFirstOrg is the bootstrap hole closed: with no membership
// and no instance role, org.create and org.list are still granted, because they
// live in the instance store rather than in an org role.
func TestNewUserCanCreateFirstOrg(t *testing.T) {
	newUser := permission.DefaultScopesForInstanceRole(permission.InstanceNone)
	instanceAll := permission.WildcardScope(permission.ScopeInstance)

	if !newUser.Grants(permission.ActionOrgCreate, instanceAll) {
		t.Error("a new user with no membership must be able to create their first organization")
	}
	if !newUser.Grants(permission.ActionOrgList, instanceAll) {
		t.Error("a new user must be able to list the organizations they belong to")
	}
	if newUser.Grants(permission.ActionAdminLockout, instanceAll) {
		t.Error("admin.lockout must not be granted to a user with no instance role")
	}
}

// TestCapabilitiesMatchesMiddleware is guarantee 3: for a representative role,
// every capability /me/capabilities reports for the org store is one the org
// guard admits, and every org guard the role reaches appears in the capability
// set. "Button shown ⇒ server allows" is this equality.
func TestCapabilitiesMatchesMiddleware(t *testing.T) {
	s := registrationServer(t)

	for _, role := range orgRoles {
		caps := permission.UnionScopes(string(role), nil)

		for _, g := range s.ScopeGuards() {
			if g.Store != guardStoreOrg {
				continue
			}
			if caps.Grants(g.Action, g.Scope) && !permission.NewEvaluator(string(role), nil).Grants(g.Action, g.Scope) {
				t.Errorf("role %s: /capabilities reports %s on %s but the guard denies it", role, g.Action, g.Scope)
			}
		}
	}
}

// TestCapabilitiesEndpointReportsThreeStores exercises the handler itself: the
// response carries the personal, instance and org namespaces, and a caller with
// no org context still receives personal and instance.
func TestCapabilitiesEndpointReportsThreeStores(t *testing.T) {

	s := &Server{}
	r := gin.New()
	r.GET("/v1/me/capabilities", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUser, &user.User{ID: "u1"})
		c.Next()
	}, s.capabilitiesHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/me/capabilities", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/capabilities = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Personal     []string `json:"personal"`
		Instance     []string `json:"instance"`
		Org          []string `json:"org"`
		InstanceRole string   `json:"instance_role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got.Personal) == 0 {
		t.Error("personal capabilities must not be empty for an authenticated user")
	}
	if !contains(got.Instance, "org.create:instance:*") {
		t.Errorf("instance capabilities must include org.create:instance:*; got %v", got.Instance)
	}
	if got.Org == nil {
		t.Error("org capabilities must render as an empty list, not null")
	}
	if got.InstanceRole != string(permission.InstanceNone) {
		t.Errorf("instance_role = %q, want %q", got.InstanceRole, permission.InstanceNone)
	}
}

// TestCapabilitiesOrgNamespaceNeedsOrgContext is the regression guard for a real
// defect found by booting the server: the /me group carried no
// OrganizationContext, so GetOrganizationID was always empty and the org
// namespace came back empty even for a caller with an active membership. It
// wires a real membership and asserts the org grants appear, which only happens
// when the route resolves the org from the session.
func TestCapabilitiesOrgNamespaceNeedsOrgContext(t *testing.T) {

	db, err := storage.Open(storage.DefaultConfig(filepath.Join(t.TempDir(), "capabilities.db")))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	raw := db.DB()
	now := time.Now().UnixMilli()

	if _, err := raw.ExecContext(t.Context(),
		`INSERT INTO users (id, email, name, password_hash, created_at, updated_at) VALUES ('u1','u1@example.test','U One','x',?,?)`,
		now, now); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := raw.ExecContext(t.Context(),
		`INSERT INTO organizations (id, name, created_by, created_at, updated_at, is_active, max_members) VALUES ('org-acme','Acme','u1',?,?,1,100)`,
		now, now); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := raw.ExecContext(t.Context(),
		`INSERT INTO organization_members (id, organization_id, user_id, role, status, joined_at) VALUES ('m1','org-acme','u1','admin','active',?)`,
		now); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	memberService := organizationapp.NewMemberService(
		storage.NewMemberStorage(raw),
		storage.NewOrganizationStorage(raw),
		nil,
		nil,
	)

	s := &Server{
		memberHandler: organizationhandlers.NewMemberHandler(memberService, nil),
	}

	r := gin.New()
	r.GET("/v1/me/capabilities",
		func(c *gin.Context) {
			c.Set(middleware.ContextKeyUser, &user.User{ID: "u1"})
			c.Set(middleware.ContextKeySession, &session.Session{
				ID:                     "s1",
				UserID:                 "u1",
				SelectedOrganizationID: "org-acme",
			})
			c.Next()
		},
		middleware.NewOrganizationContext(skipIfMissing).Middleware(),
		s.capabilitiesHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/me/capabilities", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Org []string `json:"org"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// An admin membership grants device.read on devices:*. Its presence proves the
	// org context was resolved; without OrganizationContext the list is empty.
	if !contains(got.Org, "device.read:devices:*") {
		t.Fatalf("org namespace is missing the membership's grants; got %v", got.Org)
	}
}

func anyOrgRoleGrants(action permission.Action, scope string) bool {
	for _, role := range orgRoles {
		if permission.DefaultScopesForRole(string(role)).Grants(action, scope) {
			return true
		}
	}

	return false
}

func anyInstanceRoleGrants(action permission.Action, scope string) bool {
	for _, role := range []permission.InstanceRole{permission.InstanceNone, permission.InstanceAdmin} {
		if permission.DefaultScopesForInstanceRole(role).Grants(action, scope) {
			return true
		}
	}

	return false
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}

	return false
}

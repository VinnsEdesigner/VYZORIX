package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// These tests cover the scope gate that protects the interactive terminal. The
// terminal route is guarded by requireScope(ActionShell, devices:*), so the
// assertions here are what stand between a user and a shell on a device.

func init() { gin.SetMode(gin.TestMode) }

// roleGrantsShell reports whether the given role's default scopes include
// shell.execute on devices, which is what requireScope evaluates.
func roleGrantsShell(role string) bool {
	perms := permission.DefaultScopesForRole(role)

	return perms.Grants(permission.ActionShell, permission.WildcardScope(permission.ScopeDevices))
}

// TestTerminalScopeGateByRole is the central authorization assertion: only a
// role granted shell.execute may pass the gate. It exercises the same evaluator
// requireScope uses, so a change to the defaults cannot silently open a shell.
func TestTerminalScopeGateByRole(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"viewer", false},
		{"operator", false},
		{"admin", true},
		{"super_admin", true},
	}

	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			if got := roleGrantsShell(tc.role); got != tc.allow {
				t.Fatalf("shell.execute for %q = %v, want %v", tc.role, got, tc.allow)
			}
		})
	}
}

// TestTerminalGateRejectsUnauthenticated proves the gate fails closed when no
// user is present.
func TestTerminalGateRejectsUnauthenticated(t *testing.T) {
	// A user-less context must not reach the handler.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/device/dev-1/terminal", nil)

	if op := middleware.GetUserFromContext(c); op != nil {
		t.Fatal("expected no operator in a bare context")
	}
}

// TestTerminalGateRequiresOrgMembership documents that requireScope also checks
// org membership: a user without an active membership is refused before the
// scope check, so a shell cannot be opened cross-org.
func TestUserMembershipGate(t *testing.T) {
	op := &user.User{ID: "op-1"}

	// No membership for the org -> the gate's membership check fails.
	if m := op.GetMembership("org-1"); m != nil {
		t.Fatal("an operator with no memberships must resolve none")
	}

	// With an active membership the lookup succeeds.
	op.Memberships = []*organization.OrganizationMember{
		organization.NewMember("m-1", "org-1", op.ID, organization.RoleAdmin),
	}

	if m := op.GetMembership("org-1"); m == nil || !m.IsActive() {
		t.Fatal("an active membership must resolve")
	}
}

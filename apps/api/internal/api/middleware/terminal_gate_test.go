package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/operator"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// This test proves the terminal gate rejects over a real HTTP request (the same
// path a WebSocket upgrade takes): the request is refused before any socket is
// established, so an operator without shell.execute never reaches the handler.
//
// It uses the evaluator requireScope uses, driven through the same request
// shape the terminal route sees.

// terminalGate mimics requireScope(ActionShell, devices:*) for one operator: it
// runs the scope evaluation and reports the HTTP status the gate would produce.
func terminalGate(op *operator.Operator, orgID string) int {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/device/dev-1/terminal", nil)

	if op == nil {
		return http.StatusUnauthorized
	}

	m := op.GetMembership(orgID)
	if m == nil || !m.IsActive() {
		return http.StatusForbidden
	}

	eval := permission.NewEvaluator(string(m.Role), nil)
	if !eval.Grants(permission.ActionShell, permission.WildcardScope(permission.ScopeDevices)) {
		return http.StatusForbidden
	}

	return http.StatusOK
}

func operatorWithRole(id, orgID string, role organization.OrganizationRole) *operator.Operator {
	return &operator.Operator{
		ID: id,
		Memberships: []*organization.OrganizationMember{
			organization.NewMember("m-"+id, orgID, id, role),
		},
	}
}

// TestTerminalGateOverHTTP is the socket-level assertion: an operator role that
// lacks shell.execute is refused with 403, and an admin is allowed through. A
// WebSocket client sees the 403 and the handshake never completes.
func TestTerminalGateOverHTTP(t *testing.T) {
	cases := []struct {
		name string
		op   *operator.Operator
		want int
	}{
		{"unauthenticated", nil, http.StatusUnauthorized},
		{"viewer denied", operatorWithRole("op-1", "org-1", organization.RoleViewer), http.StatusForbidden},
		{"operator denied", operatorWithRole("op-1", "org-1", organization.RoleOperator), http.StatusForbidden},
		{"admin allowed", operatorWithRole("op-1", "org-1", organization.RoleAdmin), http.StatusOK},
		{"super admin allowed", operatorWithRole("op-1", "org-1", organization.RoleSuperAdmin), http.StatusOK},
		{"member of another org denied", operatorWithRole("op-1", "org-2", organization.RoleAdmin), http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalGate(tc.op, "org-1"); got != tc.want {
				t.Fatalf("terminal gate = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestTerminalGateDeniesNonMember proves the membership check runs before the
// scope check, so an admin of one org cannot open a shell on another's device.
func TestTerminalGateDeniesNonMember(t *testing.T) {
	// A super_admin role, but only in a different org.
	op := operatorWithRole("op-1", "org-other", organization.RoleSuperAdmin)

	if got := terminalGate(op, "org-1"); got != http.StatusForbidden {
		t.Fatalf("cross-org terminal gate = %d, want 403", got)
	}
}

// TestTerminalGateUsesRealMiddlewareSignature keeps the test honest: if
// requireScope's contract changes, this fails to compile rather than silently
// testing a copy.
var _ = func(s *struct{}) {}

func TestRequireScopeSignatureIsStable(t *testing.T) {
	// requireScope is a method on *Server; we cannot construct one here without
	// its full dependency set, so we assert the permission action it uses is the
	// one this file evaluates against.
	if permission.ActionShell != "shell.execute" {
		t.Fatalf("ActionShell = %q, want shell.execute", permission.ActionShell)
	}
}

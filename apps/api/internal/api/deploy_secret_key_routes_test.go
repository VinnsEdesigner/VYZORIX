package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// TestSetSecretOnADeviceWithNoKeyIs409 is the guard for a plain error surfacing as a 500.
//
// A device with no secret key registered cannot have a value wrapped to it. That is a state
// of the device the caller can fix, not a server fault, so it must read as a conflict. Before
// the sentinel existed the error fell through the handler's default branch and answered 500,
// telling the user the server had broken.
func TestSetSecretOnADeviceWithNoKeyIs409(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy/"+deployDevice+"/secrets/web",
		[]byte(`{"key":"API_TOKEN","value":"v"}`), nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no secret key") {
		t.Fatalf("the reason is not explained: %s", w.Body.String())
	}
	// The code must say "state conflict", not "already exists": a device with no key is not a
	// duplicate identifier, and a client branching on the code would take the wrong branch.
	if !strings.Contains(w.Body.String(), `"code":"RESOURCE_CONFLICT"`) {
		t.Fatalf("wire code is not RESOURCE_CONFLICT: %s", w.Body.String())
	}
}

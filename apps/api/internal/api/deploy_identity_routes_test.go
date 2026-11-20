package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// GET /v1/deploy/whoami is what `vyzorix login` calls to prove a credential and
// show what it reaches. These tests exercise it through setupDeployRoutes, the
// same registration the server uses, so the static route cannot be shadowed by
// the /:imei sibling without a failure here.

// seedUser inserts a user row so the identity endpoint can resolve a
// name for the API-key path, which carries only an id.
func seedUser(t *testing.T, f *deployFixture, id, name, email string) {
	t.Helper()

	_, err := f.db.DB().ExecContext(t.Context(),
		`INSERT INTO users (id, email, name, password_hash, created_at, updated_at) VALUES (?, ?, ?, 'x', 0, 0)`,
		id, email, name)
	if err != nil {
		t.Fatalf("seed operator %s: %v", id, err)
	}
}

// seedOrganization inserts an org row so the identity endpoint can resolve its
// display name.
func seedOrganization(t *testing.T, f *deployFixture, id, name, createdBy string) {
	t.Helper()

	_, err := f.db.DB().ExecContext(t.Context(),
		`INSERT INTO organizations (id, name, created_by, created_at, updated_at) VALUES (?, ?, ?, 0, 0)`,
		id, name, createdBy)
	if err != nil {
		t.Fatalf("seed organization %s: %v", id, err)
	}
}

// TestWhoamiReportsUserRoleAndOrg is the happy path: a named user in an
// org gets its name, role and org name back.
func TestWhoamiReportsUserRoleAndOrg(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	seedUser(t, f, deployOp, "Ada Lovelace", "ada@example.test")
	seedOrganization(t, f, deployOrg, "Acme Devices", deployOp)

	r := f.engineFor(memberUser(organization.RoleAdmin), deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/whoami", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/deploy/whoami = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		UserName string `json:"userName"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		OrgID    string `json:"orgId"`
		OrgName  string `json:"orgName"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.UserName != "Ada Lovelace" {
		t.Errorf("operatorName = %q, want Ada Lovelace", got.UserName)
	}
	if got.Email != "ada@example.test" {
		t.Errorf("email = %q, want ada@example.test", got.Email)
	}
	if got.Role != string(organization.RoleAdmin) {
		t.Errorf("role = %q, want admin", got.Role)
	}
	if got.OrgName != "Acme Devices" {
		t.Errorf("orgName = %q, want Acme Devices", got.OrgName)
	}
}

// TestWhoamiReportsDeviceTier covers the device half: a named device yields its
// tier, and the tier words are the contract the CLI prints.
func TestWhoamiReportsDeviceTier(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	if _, err := f.db.DB().ExecContext(t.Context(),
		`UPDATE devices SET tier = ? WHERE id = ?`, int(device.TierRooted), deployDevice); err != nil {
		t.Fatalf("set tier: %v", err)
	}

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/whoami?device="+deployDevice, nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET whoami = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Device *struct {
			ID   string `json:"id"`
			Tier string `json:"tier"`
		} `json:"device"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Device == nil {
		t.Fatal("device summary missing")
	}
	if got.Device.ID != deployDevice {
		t.Errorf("device.id = %q, want %q", got.Device.ID, deployDevice)
	}
	if got.Device.Tier != "rooted" {
		t.Errorf("device.tier = %q, want rooted", got.Device.Tier)
	}
}

// TestWhoamiNamesADeployCredential covers the CI path: a deploy credential has no
// user, so it reports itself and the device it is scoped to, and it cannot be
// pointed at another device.
func TestWhoamiNamesADeployCredential(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	secret, _, err := f.server.deployCredentials.Mint(t.Context(), deployOrg, deployDevice, "ci", deployOp)
	if err != nil {
		t.Fatalf("mint credential: %v", err)
	}

	r := f.engineFor(nil, "")
	// The engineFor injects no user and no org; a credential sets both itself.
	w := do(t, r, http.MethodGet, "/v1/deploy/whoami", nil, map[string]string{
		middleware.HeaderDeployCredential: secret,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("GET whoami as credential = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		UserName string `json:"userName"`
		OrgID    string `json:"orgId"`
		Device   *struct {
			ID string `json:"id"`
		} `json:"device"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.UserName != "deploy credential" {
		t.Errorf("operatorName = %q, want deploy credential", got.UserName)
	}
	if got.OrgID != deployOrg {
		t.Errorf("orgId = %q, want %q", got.OrgID, deployOrg)
	}
	if got.Device == nil || got.Device.ID != deployDevice {
		t.Errorf("device = %+v, want %s", got.Device, deployDevice)
	}

	// A credential is scoped to its device: naming another is a refusal, not a
	// silent answer for its own device.
	w = do(t, r, http.MethodGet, "/v1/deploy/whoami?device="+otherDevice, nil, map[string]string{
		middleware.HeaderDeployCredential: secret,
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("whoami for another device = %d, want 404", w.Code)
	}
}

// TestWhoamiRefusesAnAnonymousCall guards the auth boundary: no user and no
// credential is a 401.
func TestWhoamiRefusesAnAnonymousCall(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	r := f.engineFor(nil, deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/whoami", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous whoami = %d, want 401", w.Code)
	}
}

// TestWhoamiIsNotReadAsADeviceID is the regression for the route seam: /whoami is
// a static sibling of /:imei, and the static must win. If it did not, whoami would
// be read as the device id "whoami" and answered 200 with an empty identity or
// refused as not-in-org — never the user's own name.
func TestWhoamiIsNotReadAsADeviceID(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	seedUser(t, f, deployOp, "Ada Lovelace", "ada@example.test")

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/whoami", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("whoami = %d, want 200", w.Code)
	}

	var got struct {
		UserName string `json:"userName"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.UserName != "Ada Lovelace" {
		t.Fatalf("operatorName = %q: whoami was read as a device id", got.UserName)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// TestWhoamiReportsDeviceABIs pins that the identity endpoint exposes the
// device's comma-joined ABIs as a list, which `vyzorix build` reads to target the
// right architecture.
func TestWhoamiReportsDeviceABIs(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	// Give the fixture's registered device the ABIs a modern phone reports.
	if _, err := f.db.DB().ExecContext(t.Context(),
		`UPDATE devices SET supported_abis = ? WHERE id = ?`,
		"arm64-v8a,armeabi-v7a", deployDevice); err != nil {
		t.Fatalf("set device ABIs: %v", err)
	}

	r := f.engineFor(memberUser(organization.RoleAdmin), deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/whoami?device="+deployDevice, nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("whoami = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Device struct {
			ID            string   `json:"id"`
			SupportedABIs []string `json:"supportedAbis"`
		} `json:"device"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}

	if len(got.Device.SupportedABIs) != 2 {
		t.Fatalf("supportedAbis = %v, want two entries", got.Device.SupportedABIs)
	}

	if got.Device.SupportedABIs[0] != "arm64-v8a" || got.Device.SupportedABIs[1] != "armeabi-v7a" {
		t.Errorf("supportedAbis = %v, want [arm64-v8a armeabi-v7a]", got.Device.SupportedABIs)
	}
}

// TestBuildSchemaRouteIsPublic proves the build-recipe schema is served without a
// credential, because the build CLI validates a recipe before it holds one, and
// that an unknown version is refused rather than silently answered.
func TestBuildSchemaRouteIsPublic(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	// The public router, with no tenant group and no injected principal.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	f.server.setupDeploySchemaRoute(r.Group("/v1"))

	w := do(t, r, http.MethodGet, "/v1/deploy/build-schema/1", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("build-schema/1 = %d, want 200 (public); body=%s", w.Code, w.Body.String())
	}

	var doc map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("build schema is not JSON: %v", err)
	}

	if _, ok := doc["$defs"]; !ok {
		t.Errorf("build schema has no $defs; body=%s", w.Body.String())
	}

	unknown := do(t, r, http.MethodGet, "/v1/deploy/build-schema/999", nil, nil)
	if unknown.Code != http.StatusNotFound {
		t.Errorf("build-schema/999 = %d, want 404", unknown.Code)
	}
}

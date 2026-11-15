package api

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
)

// The deploy HTTP surface, exercised through the production route registration.
//
// These tests exist because the handler and the route are two files, and the one
// bug that shipped here lived exactly in the seam between them: the handler read
// c.Param("imei") on a route that has no device segment, so every deploy was
// refused. A test that builds its own routes would not have caught it, so these
// call setupDeployRoutes — the same function the server calls.
//
// The stores are the real SQLite repositories over a migrated temp database, and
// the services are the real ones. Only the authenticated principal is injected,
// because that is what cookieAuth would have established one layer up.

// deployOrg and deployDevice are the fixture's tenant and its registered device.
const (
	deployOrg    = "org_01H"
	deployDevice = "d_01H"
	deployOp     = "op_01H"
	otherOrg     = "org_02H"
	otherDevice  = "d_02H"
)

// deployManifest is a minimal valid manifest. The device and org are server-
// injected fields the file may not contradict; here they match the fixture.
const deployManifest = `
[device]
id  = "d_01H"
org = "org_01H"

[[app]]
name = "vfd"
cmd  = "/home/apps/vfd/run.sh"
port = 8080
expose = "loopback"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/vyzor-arm64.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
`

// deployFixture is a running deploy surface over real stores.
type deployFixture struct {
	server   *Server
	engine   *gin.Engine
	configs  *storage.DeployConfigRepository
	creds    *storage.DeployCredentialRepository
	db       *storage.SQLite
	user *user.User
	orgID    string
}

// newDeployFixture builds the deploy surface over a migrated temp database with
// one registered device in deployOrg and one in otherOrg.
func newDeployFixture(t *testing.T) *deployFixture {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := storage.Open(storage.DefaultConfig(filepath.Join(t.TempDir(), "deploy.db")))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	raw := db.DB()
	devices := storage.NewDeviceRepository(raw)
	configs := storage.NewDeployConfigRepository(raw)
	secrets := storage.NewAppSecretRepository(raw)
	secretKeys := storage.NewDeviceSecretKeyRepository(raw)
	creds := storage.NewDeployCredentialRepository(raw)
	grants := storage.NewGrantRepository(raw)
	users := storage.NewUserRepository(raw)
	organizations := storage.NewOrganizationStorage(raw)

	signer := testServerSigner(t)

	svc := deploy.NewDeployService(configs, devices, signer.SignDocument, deploy.DeviceSettings{}, nil)
	secretSvc := deploy.NewSecretService(secrets, secretKeys)
	credSvc := deploy.NewCredentialService(creds)

	s := &Server{
		log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeviceRepo:        devices,
		grantRepo:         grants,
		deployCredentials: credSvc,
	}
	s.deployHandler = deployhandlers.NewHandler(svc, secretSvc, credSvc, devices, nil, grants, users, organizations, s.log)

	// Seed a registered device in each org. RegisteredAt > 0 is what the storage
	// layer reads as LifecycleRegistered, which the predicate requires.
	seedRegisteredDevice(t, devices, deployDevice, deployOrg)
	seedRegisteredDevice(t, devices, otherDevice, otherOrg)

	f := &deployFixture{
		server:  s,
		configs: configs,
		creds:   creds,
		db:      db,
		orgID:   deployOrg,
	}

	return f
}

// engineFor builds a router whose tenant group is the real deploy registration,
// with the authenticated principal injected ahead of it.
func (f *deployFixture) engineFor(op *user.User, orgID string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	tenant := r.Group("/v1")
	tenant.Use(func(c *gin.Context) {
		if op != nil {
			c.Set(middleware.ContextKeyUser, op)
		}

		if orgID != "" {
			c.Set(middleware.ContextKeyOrganizationID, orgID)
		}

		c.Next()
	})

	f.server.setupDeployRoutes(tenant)

	return r
}

// memberUser is an active user in the fixture org with the given role.
func memberUser(role organization.OrganizationRole) *user.User {
	op := &user.User{ID: deployOp}
	op.Memberships = []*organization.OrganizationMember{
		organization.NewMember("mem_01H", deployOrg, deployOp, role),
	}

	return op
}

// grantDeploy writes the device.deploy grant a user needs. A super-admin
// needs none; every other role does, and only on the device it names.
func (f *deployFixture) grantDeploy(t *testing.T, scope string) {
	t.Helper()

	err := f.server.grantRepo.Save(t.Context(), &permission.ResourcePermission{
		ID:          "grant_deploy",
		OrgID:       deployOrg,
		SubjectType: permission.SubjectOperator,
		SubjectID:   deployOp,
		Actions:     []permission.Action{permission.ActionDeviceDeploy},
		Scope:       scope,
	})
	if err != nil {
		t.Fatalf("save grant: %v", err)
	}
}

// do runs one request against the fixture's router and returns the recorder.
func do(t *testing.T, r *gin.Engine, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

// TestDeployRouteTakesTheDeviceFromTheManifest is the regression for the bug this
// suite exists for: POST /v1/deploy has no :imei, so the handler must not read
// the path. Before the fix the device id was the empty string and the predicate
// refused every deploy as not-in-org — a 404 on the happy path.
func TestDeployRouteTakesTheDeviceFromTheManifest(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/deploy = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Version      int    `json:"version"`
		AuthorizedBy string `json:"authorizedBy"`
		Apps         []struct {
			Name   string `json:"name"`
			Custom bool   `json:"custom"`
		} `json:"apps"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Version != 1 {
		t.Fatalf("first deploy must be version 1, got %d", got.Version)
	}

	if got.AuthorizedBy != deployOp {
		t.Fatalf("authorizedBy = %q, want %q", got.AuthorizedBy, deployOp)
	}

	if len(got.Apps) != 1 || got.Apps[0].Name != "vfd" {
		t.Fatalf("apps = %v, want [vfd]", got.Apps)
	}

	// The config must have been persisted against the manifest's device, not "".
	record, err := f.configs.Current(t.Context(), deployDevice)
	if err != nil {
		t.Fatalf("the deploy must persist against the manifest's device: %v", err)
	}

	if record.Version != 1 {
		t.Fatalf("stored version = %d, want 1", record.Version)
	}
}

// TestDeployRefusesCrossOrgDeviceAsNotFound pins the existence-leak rule: a device
// in another tenant must read as 404, never 403, so a caller cannot tell "absent"
// from "someone else's".
func TestDeployRefusesCrossOrgDeviceAsNotFound(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	// The grant covers the device, so the refusal must come from the org binding
	// and not from the grant.
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	manifest := `
[device]
id  = "d_02H"
org = "org_01H"

[[app]]
name = "vfd"
cmd  = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/vyzor-arm64.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
`

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(manifest), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-org device = %d, want 404 (never 403); body=%s", w.Code, w.Body.String())
	}
}

// TestDeployRefusesWithoutTheGrant pins that a user with no device.deploy
// grant is refused, and that the refusal is a 403 and not a 404.
func TestDeployRefusesWithoutTheGrant(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	// No grant is written.

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no grant = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// TestDeployRefusesAnUnregisteredDevice pins the 409: the request is well-formed
// and the device is in the org, but it is not yet registered, so it cannot take a
// config.
func TestDeployRefusesAnUnregisteredDevice(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	// A pending device: RegisteredAt == 0 reads as LifecyclePending.
	pending := &device.Device{ID: "d_pending", OrganizationID: deployOrg}
	if err := f.server.DeviceRepo.Create(t.Context(), pending); err != nil {
		t.Fatalf("seed pending device: %v", err)
	}

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	manifest := `
[device]
id  = "d_pending"
org = "org_01H"

[[app]]
name = "vfd"
cmd  = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/vyzor-arm64.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
`

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(manifest), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("unregistered device = %d, want 409; body=%s", w.Code, w.Body.String())
	}
}

// TestDeployDryRunStoresNothing pins the dry-run contract: it compiles and
// authorises but stops short of delivery, so no config is persisted.
func TestDeployDryRunStoresNothing(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy?dry_run=true", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("dry run = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		DryRun bool `json:"dryRun"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !got.DryRun {
		t.Fatal("the response must report dryRun=true")
	}

	if _, err := f.configs.Current(t.Context(), deployDevice); err == nil {
		t.Fatal("a dry run must not persist a config")
	} else if !errors.Is(err, deployconfig.ErrNotFound) {
		t.Fatalf("unexpected error reading config after dry run: %v", err)
	}
}

// TestDeployRefusesAnInvalidManifest pins that a schema error is a 400 and names
// the offending field, so the CLI can print it.
func TestDeployRefusesAnInvalidManifest(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	// `bogus` is not a key the manifest defines, so strict decoding refuses it.
	manifest := `
[device]
id  = "d_01H"
org = "org_01H"
bogus = "nope"

[[app]]
name = "vfd"
cmd  = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/vyzor-arm64.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
`

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(manifest), nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid manifest = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

// TestDeployRequiresAnOrganization pins that a request with no org context is
// refused before any work happens.
func TestDeployRequiresAnOrganization(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	r := f.engineFor(memberUser(organization.RoleOperator), "")

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no org = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

// TestDeployRefusesAnUnauthenticatedCaller pins that a request with an org but no
// principal is refused.
func TestDeployRefusesAnUnauthenticatedCaller(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	r := f.engineFor(nil, deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no caller = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// TestCurrentRequiresTheGrant pins that reading a device's config takes the same
// grant as writing it: the config names the apps and their secrets.
func TestCurrentRequiresTheGrant(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/"+deployDevice, nil, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET without grant = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// TestCurrentReturnsNotFoundBeforeTheFirstDeploy pins the 404 for a device that
// exists and is authorised but has never been deployed to.
func TestCurrentReturnsNotFoundBeforeTheFirstDeploy(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodGet, "/v1/deploy/"+deployDevice, nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no config yet = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

// TestSuperAdminDeploysWithoutAGrant pins the other half of the predicate: a
// super-admin needs no explicit grant.
func TestSuperAdminDeploysWithoutAGrant(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	// No grant is written.

	r := f.engineFor(memberUser(organization.RoleSuperAdmin), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("super-admin deploy = %d, want 202; body=%s", w.Code, w.Body.String())
	}
}

// TestMintedCredentialDeploysAndIsRefusedAfterRevocation covers end to end:
// a device-scoped credential mints, authenticates a deploy with no session, and
// stops working the moment it is revoked.
func TestMintedCredentialDeploysAndIsRefusedAfterRevocation(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	// Mint with the user's session.
	w := do(t, r, http.MethodPost, "/v1/deploy/"+deployDevice+"/credentials",
		[]byte(`{"name":"ci"}`), map[string]string{"Content-Type": "application/json"})
	if w.Code != http.StatusCreated {
		t.Fatalf("mint = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var minted struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &minted); err != nil {
		t.Fatalf("decode mint: %v", err)
	}

	if minted.Token == "" {
		t.Fatal("mint must return the token exactly once")
	}

	// Deploy with the credential alone: no user in the context.
	credEngine := f.engineFor(nil, "")
	headers := map[string]string{middleware.HeaderDeployCredential: minted.Token}

	w = do(t, credEngine, http.MethodPost, "/v1/deploy", []byte(deployManifest), headers)
	if w.Code != http.StatusAccepted {
		t.Fatalf("credential deploy = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	// The config carries the member who authorised it. A credential deploy has
	// no user in the context, so the member must come from the credential's
	// minting user — not from GetUserFromContext, which is nil here.
	record, err := f.configs.Current(t.Context(), deployDevice)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if !strings.Contains(record.Document, "member "+deployOp) {
		t.Fatalf("credential deploy must record the minting operator as member; document=\n%s", record.Document)
	}

	// Revoke with the user's session, then the credential must be refused.
	w = do(t, r, http.MethodDelete, "/v1/deploy/"+deployDevice+"/credentials/"+minted.ID, nil, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d, want 204; body=%s", w.Code, w.Body.String())
	}

	w = do(t, credEngine, http.MethodPost, "/v1/deploy", []byte(deployManifest), headers)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked credential = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// TestCredentialCannotReachAnotherDevice pins the scoping: a credential minted
// for one device is refused on a path that names another.
func TestCredentialCannotReachAnotherDevice(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy/"+deployDevice+"/credentials",
		[]byte(`{"name":"ci"}`), map[string]string{"Content-Type": "application/json"})
	if w.Code != http.StatusCreated {
		t.Fatalf("mint = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var minted struct {
		Token string `json:"token"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &minted); err != nil {
		t.Fatalf("decode mint: %v", err)
	}

	credEngine := f.engineFor(nil, "")
	headers := map[string]string{middleware.HeaderDeployCredential: minted.Token}

	w = do(t, credEngine, http.MethodGet, "/v1/deploy/"+otherDevice, nil, headers)
	if w.Code != http.StatusForbidden {
		t.Fatalf("credential on another device = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// TestSecretSetListUnsetNeverReturnsAValue is the invariant at the HTTP
// seam: a value goes in on set, and no response ever carries it back.
func TestSecretSetListUnsetNeverReturnsAValue(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	// A secret is wrapped to the device's key, so the device needs one.
	seedSecretKey(t, f, deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	const plaintext = "postgres://user:hunter2@db/app"

	w := do(t, r, http.MethodPost, "/v1/deploy/"+deployDevice+"/secrets/vfd",
		[]byte(`{"key":"DATABASE_URL","value":"`+plaintext+`"}`),
		map[string]string{"Content-Type": "application/json"})
	if w.Code != http.StatusOK {
		t.Fatalf("set secret = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	if bytes.Contains(w.Body.Bytes(), []byte(plaintext)) {
		t.Fatal("the set response must not echo the value")
	}

	// List returns names and generations only.
	w = do(t, r, http.MethodGet, "/v1/deploy/"+deployDevice+"/secrets/vfd", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list secrets = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	if bytes.Contains(w.Body.Bytes(), []byte(plaintext)) {
		t.Fatalf("list must never return a value; body=%s", w.Body.String())
	}

	var listed struct {
		Secrets []struct {
			Key        string `json:"key"`
			Generation int64  `json:"generation"`
		} `json:"secrets"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(listed.Secrets) != 1 || listed.Secrets[0].Key != "DATABASE_URL" {
		t.Fatalf("list = %+v, want one DATABASE_URL entry", listed.Secrets)
	}

	if listed.Secrets[0].Generation < 1 {
		t.Fatalf("generation = %d, want >= 1", listed.Secrets[0].Generation)
	}

	// Unset removes it.
	w = do(t, r, http.MethodDelete, "/v1/deploy/"+deployDevice+"/secrets/vfd/DATABASE_URL", nil, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("unset secret = %d, want 204; body=%s", w.Code, w.Body.String())
	}

	w = do(t, r, http.MethodGet, "/v1/deploy/"+deployDevice+"/secrets/vfd", nil, nil)
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list after unset: %v", err)
	}

	if len(listed.Secrets) != 0 {
		t.Fatalf("after unset the list must be empty, got %+v", listed.Secrets)
	}
}

// TestSecretSetRequiresTheGrant pins that writing a credential takes the same
// grant as writing what runs.
func TestSecretSetRequiresTheGrant(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	seedSecretKey(t, f, deployDevice)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy/"+deployDevice+"/secrets/vfd",
		[]byte(`{"key":"K","value":"v"}`), map[string]string{"Content-Type": "application/json"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("set secret without grant = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// seedRegisteredDevice inserts a device whose RegisteredAt marks it registered.
func seedRegisteredDevice(t *testing.T, repo *storage.DeviceRepository, id, orgID string) {
	t.Helper()

	d := &device.Device{
		ID:             id,
		OrganizationID: orgID,
		RegisteredAt:   1_700_000_000_000,
		LastSeen:       1_700_000_000_000,
	}

	if err := repo.Create(t.Context(), d); err != nil {
		t.Fatalf("seed device %s: %v", id, err)
	}
}

// seedSecretKey gives a device the PURPOSE_DECRYPT key a secret is wrapped to.
func seedSecretKey(t *testing.T, f *deployFixture, deviceID string) {
	t.Helper()

	priv, err := cryptohmac.GenerateSecretKeyPair()
	if err != nil {
		t.Fatalf("generate secret key: %v", err)
	}

	publicPEM, err := cryptohmac.EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode secret key: %v", err)
	}

	_, err = f.db.DB().ExecContext(t.Context(),
		`UPDATE devices SET secret_public_key = ? WHERE id = ?`, publicPEM, deviceID)
	if err != nil {
		t.Fatalf("set secret key: %v", err)
	}
}

// testServerSigner builds the server's config signer from a fresh P-256 key.
func testServerSigner(t *testing.T) *cryptohmac.ServerSigner {
	t.Helper()

	key, err := cryptohmac.GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal signing key: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	signer, err := cryptohmac.NewServerSigner(string(pemBytes))
	if err != nil {
		t.Fatalf("build signer: %v", err)
	}

	return signer
}

// TestSchemaRouteIsPublicAndCoexistsWithTheDeviceRoute registers the public
// schema route beside the tenant deploy routes, the way the server does. It pins
// what a handler unit test cannot: gin accepts the static /v1/deploy/schema/:version
// path next to the tenant /v1/deploy/:imei path (a conflict panics at
// registration), and the schema is served without a credential.
func TestSchemaRouteIsPublicAndCoexistsWithTheDeviceRoute(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)

	gin.SetMode(gin.TestMode)

	r := gin.New()
	v1 := r.Group("/v1")

	f.server.setupDeployRoutes(v1)
	f.server.setupDeploySchemaRoute(v1)

	w := do(t, r, http.MethodGet, "/v1/deploy/schema/1", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/deploy/schema/1 = %d, want 200 with no auth; body=%s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "DeployManifest") {
		t.Fatalf("body is not the manifest schema: %s", w.Body.String())
	}

	w = do(t, r, http.MethodGet, "/v1/deploy/schema/999", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown schema version = %d, want 404", w.Code)
	}
}

// recordingPusher records the devices a deploy pushed a config to.
type recordingPusher struct {
	mu      sync.Mutex
	devices []string
}

func (p *recordingPusher) PushConfig(_ context.Context, deviceID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.devices = append(p.devices, deviceID)

	return true
}

func (p *recordingPusher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.devices)
}

// TestDeployPushesToAConnectedDevice pins that a successful deploy notifies the
// target device immediately, so a device that is connected does not wait for its
// next reconnect to pick the new config up.
func TestDeployPushesToAConnectedDevice(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	pusher := &recordingPusher{}
	f.server.deployHandler.SetConfigPusher(pusher)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/deploy = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	if pusher.count() != 1 {
		t.Fatalf("a successful deploy must push the config once, got %d", pusher.count())
	}
}

// TestDeployDryRunDoesNotPush pins that a dry run is inert: it compiles and signs
// but neither stores nor notifies, so a device is never told to apply a version
// that was never persisted.
func TestDeployDryRunDoesNotPush(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	pusher := &recordingPusher{}
	f.server.deployHandler.SetConfigPusher(pusher)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy?dry_run=true", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("dry run = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	if pusher.count() != 0 {
		t.Fatalf("a dry run must not push a config, got %d pushes", pusher.count())
	}
}

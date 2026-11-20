package deploy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	libv "github.com/VinnsEdesigner/libv/go"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// The deploy orchestration: authorize, compile, sign, persist. The handler is a
// thin adapter over this, so the sequence and its ordering live in one place and
// are testable without HTTP.

// DeployRequest is one user's deploy. The caller is resolved by the handler,
// which is the authentication boundary, and passed in so the service never has to
// reach back into a request context it does not own.
//
// DeviceID is the target. On `POST /v1/deploy` the path names no device, so the
// handler leaves it empty and the service takes it from the manifest's `device.id`
// — the file is the declaration, and the endpoint is the target.
type DeployRequest struct {
	Caller   *Caller
	OrgID    string
	DeviceID string
	MemberID string
	Manifest []byte
	DryRun   bool
}

// DeployResult is what the user is told.
type DeployResult struct {
	AuthorizedBy string
	Document     string
	DeviceID     string
	Artifacts    []ArtifactConfig
	AppNames     []string
	Apps         []AppMarking
	Version      int
	DryRun       bool
}

// AuditSink records a deploy action. It is a narrow interface so the service does not
// depend on the audit package, and a service built without one records nothing rather
// than panicking.
//
// A rollback has no HTTP route of its own — provisioning calls it directly — so the
// service records it here rather than relying on a handler to do it.
type AuditSink interface {
	DeployFromService(ctx context.Context, e DeployEvent)
}

// The action and result values the service emits. They are the audit package's own
// constants spelled out, because the deploy package must not import the audit package; a
// test pins the two sets together so a rename on one side fails the build rather than
// silently producing an unrecognised action.
const (
	// ActionDeployed is audit.ActionDeploySucceeded.
	ActionDeployed = "deploy_succeeded"
	// ActionRolledBack is audit.ActionDeployRolledBack.
	ActionRolledBack = "deploy_rolled_back"
	// ActionApplied is audit.ActionDeployApplied.
	ActionApplied = "deploy_applied"
	// ResultSuccess is audit.ResultSuccess.
	ResultSuccess = "success"
)

// DeployEvent is the audit record of one deploy action, as the service sees it.
type DeployEvent struct {
	UserID   string
	OrgID    string
	DeviceID string
	Action   string
	Result   string
	Detail   string
	Version  int
}

// SecretReadiness reports which of a device's secrets are set, by name.
//
// It is a narrow interface over the app-secret store, and it deliberately returns names
// only: the check is "does this key exist", and nothing about a value ever reaches the
// config, the job, or the log.
type SecretReadiness interface {
	// SetSecretNames returns the names of the secrets stored for an app on a device.
	SetSecretNames(ctx context.Context, deviceID, app string) ([]string, error)
}

// DeployService orchestrates a deploy end to end.
type DeployService struct {
	configs  deployconfig.Repository
	lookup   DeviceLookup
	sign     DocumentSigner
	settings SettingsSource
	catalog  *Catalog
	audit    AuditSink
	secrets  SecretReadiness
	locks    *deployLocks
	now      func() time.Time
}

// SetSecretReadiness injects the secret store the apply-time check reads. A service built
// without one skips the check, which keeps every existing test and the offline path
// working; the server always wires it.
func (s *DeployService) SetSecretReadiness(r SecretReadiness) { s.secrets = r }

// deployLocks serialises deploys per device.
//
// The version is part of the signed document, so it must be chosen before signing:
// read current, add one, sign, write. Two deploys that run that sequence at the same
// time both read v3 and both write v4, and one of them is silently lost — the second
// write overwrites the first document. Holding a per-device lock for the whole
// sequence is what makes "one deploy, one version" true rather than likely.
type deployLocks struct {
	held map[string]struct{}
	mu   sync.Mutex
}

func newDeployLocks() *deployLocks { return &deployLocks{held: map[string]struct{}{}} }

// acquire claims a device's slot, reporting false when one is already held.
func (l *deployLocks) acquire(deviceID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.held[deviceID]; ok {
		return false
	}
	l.held[deviceID] = struct{}{}

	return true
}

// release frees a device's slot.
func (l *deployLocks) release(deviceID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.held, deviceID)
}

// ErrDeployInProgress is returned when a second deploy targets a device whose deploy
// is still running. Refusing is honest: the caller can retry, whereas silently
// proceeding would produce the version collision the lock exists to prevent.
var ErrDeployInProgress = errors.New("a deploy is already running for this device")

// SettingsSource supplies the server-owned settings a compile injects: the
// tier cap, the stream path and the feature flags. They come from the device
// record and the server's policy, never from the manifest.
type SettingsSource interface {
	ForDevice(ctx context.Context, dev *device.Device) Injection
}

// NewDeployService builds a DeployService. A nil catalog marks every app custom,
// which is the honest answer when no provisioning file declared one.
func NewDeployService(
	configs deployconfig.Repository,
	lookup DeviceLookup,
	sign DocumentSigner,
	settings SettingsSource,
	catalog *Catalog,
) *DeployService {
	return &DeployService{
		configs:  configs,
		lookup:   lookup,
		sign:     sign,
		settings: settings,
		catalog:  catalog,
		locks:    newDeployLocks(),
		now:      time.Now,
	}
}

// SetAudit injects the audit sink. It is a setter because the audit logger is built
// alongside the database, after this service.
func (s *DeployService) SetAudit(a AuditSink) { s.audit = a }

// Deploy runs the sequence for one request.
func (s *DeployService) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	caller := req.Caller

	// The caller is resolved for the requested org, but the org it is actually
	// in is the authority. A resolver that returned a caller for a different
	// org would otherwise let a deploy land in the wrong tenant.
	if caller == nil || caller.OrgID != req.OrgID {
		return nil, &DeniedError{Denial: DenialNotInOrg}
	}

	// The device is named by the manifest, not by the path: `POST /v1/deploy` has
	// no device segment. Reading the id on its own — before any validation — keeps
	// the ordering below, so an unauthorised caller is refused without being told
	// which manifest keys are wrong.
	deviceID := req.DeviceID
	if deviceID == "" {
		peeked, err := PeekDeviceID(req.Manifest)
		if err != nil {
			return nil, err
		}

		deviceID = peeked
	}

	if deviceID == "" {
		return nil, &ManifestError{Field: "device.id", Message: "required"}
	}

	// Authorize before parsing: an unauthorised caller must not learn whether the
	// device exists, and must not reach the compiler at all.
	dev, err := AuthorizeDeploy(ctx, s.lookup, caller, deviceID)
	if err != nil {
		return nil, err
	}

	manifest, err := ParseManifest(req.Manifest)
	if err != nil {
		return nil, err
	}

	// The manifest's own device id must match the target: a file that names a
	// different device is a mistake, and silently ignoring it would deploy the
	// wrong thing.
	if manifest.Device.ID != "" && manifest.Device.ID != deviceID {
		return nil, &ManifestError{
			Field:   "device.id",
			Message: fmt.Sprintf("names %q, but the deploy targets %q", manifest.Device.ID, deviceID),
		}
	}

	// The org in the file must be the caller's org. The caller is already bound to
	// req.OrgID above, so this is the file being checked against the tenant, not a
	// second authorization: a manifest copied from another tenant is refused rather
	// than deployed under the caller's org.
	if manifest.Device.Org != "" && manifest.Device.Org != req.OrgID {
		return nil, &ManifestError{
			Field:   "device.org",
			Message: fmt.Sprintf("names %q, but the deploy targets org %q", manifest.Device.Org, req.OrgID),
		}
	}

	// The manifest declares the secrets each app needs. Checking they exist here means a
	// missing value fails the apply with a message naming the key, instead of surfacing
	// later when the device asks for a secret the config names and gets a 404.
	if err = s.checkDeclaredSecrets(ctx, deviceID, manifest); err != nil {
		return nil, err
	}

	injection := s.settings.ForDevice(ctx, dev)
	injection.MemberID = req.MemberID
	injection.DeviceID = deviceID
	injection.OrgID = req.OrgID

	// A dry run stores nothing, so it needs no lock: two previews at the same version are
	// harmless, and they must not block each other or a real deploy.
	if req.DryRun {
		return s.compilePreview(ctx, manifest, injection)
	}

	// The version is inside the signed document, so it has to be chosen before signing.
	// Read-increment-sign-write is one sequence: without the lock, two deploys to the same
	// device both read v3, both write v4, and one document is silently overwritten.
	if !s.locks.acquire(deviceID) {
		return nil, ErrDeployInProgress
	}
	defer s.locks.release(deviceID)

	current, err := s.currentVersion(ctx, req.OrgID, deviceID)
	if err != nil {
		return nil, err
	}

	injection.Version = current + 1

	cfg, err := Compile(manifest, injection)
	if err != nil {
		return nil, err
	}

	document, err := SignConfig(cfg, s.sign)
	if err != nil {
		return nil, err
	}

	result := resultFromConfig(cfg, injection.Version, false, s.catalog)
	result.DeviceID = deviceID

	if err := s.persist(ctx, deviceID, injection.Version, document); err != nil {
		return nil, err
	}

	// The service records the deploy, not only the HTTP handler. A provisioned apply
	// (a repository sync) calls Deploy directly and never touches the handler, so
	// auditing in the handler alone left every provisioned deploy with no audit row —
	// the same fleet change, recorded or not depending on which door it came through.
	s.auditDeployed(ctx, req, deviceID, injection.Version)

	return result, nil
}

// auditDeployed writes the deploy event when a sink is wired.
func (s *DeployService) auditDeployed(ctx context.Context, req DeployRequest, deviceID string, version int) {
	if s.audit == nil {
		return
	}

	s.audit.DeployFromService(ctx, DeployEvent{
		UserID:   req.MemberID,
		OrgID:    req.OrgID,
		DeviceID: deviceID,
		Version:  version,
		Action:   ActionDeployed,
		Result:   ResultSuccess,
		Detail:   "deployed",
	})
}

// compilePreview compiles and signs a dry run without touching the store.
//
// The preview reports the version a real deploy would take (current + 1), read without
// the lock: a preview is not a reservation, and taking the lock here would make a
// preview block a real deploy that is about to start.
func (s *DeployService) compilePreview(ctx context.Context, manifest *Manifest, injection Injection) (*DeployResult, error) {
	current, err := s.currentVersion(ctx, injection.OrgID, injection.DeviceID)
	if err != nil {
		return nil, err
	}

	injection.Version = current + 1

	cfg, err := Compile(manifest, injection)
	if err != nil {
		return nil, err
	}

	document, err := SignConfig(cfg, s.sign)
	if err != nil {
		return nil, err
	}

	result := resultFromConfig(cfg, injection.Version, true, s.catalog)
	result.DeviceID = injection.DeviceID
	result.Document = document

	return result, nil
}

// Deliverable is the config a device is served: the signed document and the
// version it carries, so the device can compare what it holds to what is on
// offer before fetching.
type Deliverable struct {
	Document string
	Version  int
	IssuedAt int64
}

// DeliverableForDevice returns the signed config a device should be running, or
// ErrNoConfig when the device has never been deployed to.
//
// This is the delivery half of the deploy feature: the device_config frame and
// the re-fetch endpoint both read the *stored* config, not a fresh compile. The
// stored config is the one the user's manifest produced and the server
// signed; rebuilding it here would issue a different document at a different
// version and the device would apply something nobody deployed.
func (s *DeployService) DeliverableForDevice(ctx context.Context, deviceID string) (*Deliverable, error) {
	record, err := s.configs.CurrentForDevice(ctx, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, ErrNoConfig
	}

	if err != nil {
		return nil, err
	}

	return &Deliverable{
		Document: record.Document,
		Version:  record.Version,
		IssuedAt: record.CreatedAt.UnixMilli(),
	}, nil
}

// MarkApplied records that a device reported a version as applied (step 8).
// It is how the console knows a deploy landed rather than merely being stored.
//
// A version the store does not hold as current is not an error: the device may
// report late, after a newer deploy has superseded the one it applied. The
// update simply matches no row, and the current config's applied_at stays unset
// until the device applies the version it is actually running.
func (s *DeployService) MarkApplied(ctx context.Context, deviceID string, version int) error {
	if err := s.configs.MarkApplied(ctx, deviceID, version, s.now()); err != nil {
		return err
	}

	// The device reporting a version applied is the only signal that a deploy actually
	// took effect, so it is recorded. The actor is the device, not a person: nobody pressed
	// anything, and the row should not be attributed to whoever last deployed.
	if s.audit != nil {
		s.audit.DeployFromService(ctx, DeployEvent{
			DeviceID: deviceID,
			Version:  version,
			Action:   ActionApplied,
			Result:   ResultSuccess,
			Detail:   "device reported the config applied",
		})
	}

	return nil
}

// Current returns the config a device should be running, after the same
// authorization a deploy requires: a caller who may not deploy to a device may
// not read its config either, since the config names its apps and their secrets.
func (s *DeployService) Current(ctx context.Context, caller *Caller, orgID, deviceID string) (*deployconfig.Record, error) {
	if caller == nil || caller.OrgID != orgID {
		return nil, &DeniedError{Denial: DenialNotInOrg}
	}

	if _, err := AuthorizeDeploy(ctx, s.lookup, caller, deviceID); err != nil {
		return nil, err
	}

	record, err := s.configs.Current(ctx, orgID, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, ErrNoConfig
	}

	return record, err
}

// ErrNoConfig is returned when a device has never been deployed to.
var ErrNoConfig = errors.New("device has no config")

// ErrNothingToRevert is returned when a device has no last-good config to restore.
var ErrNothingToRevert = errors.New("device has no previous config to revert to")

// CurrentVersion reports the version the device's current config carries.
//
// ok is false when the device has no config at all, which is distinct from version
// 0: a caller detecting drift must not treat "never deployed" as "drifted from
// version 0". It takes the org because it is a fact about a device in a tenant, and
// the store refuses a device outside it — provisioning passes the repository's org.
func (s *DeployService) CurrentVersion(ctx context.Context, orgID, deviceID string) (int, bool, error) {
	record, err := s.configs.Current(ctx, orgID, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	return record.Version, true, nil
}

// SecretRequirement is one secret an app declares, and whether the device has it set.
//
// It carries no value: the console needs to know that a key is missing so it can offer to
// set it, and nothing more.
type SecretRequirement struct {
	App string `json:"app"`
	Key string `json:"key"`
	// Set is true when the device has this secret stored.
	Set bool `json:"set"`
}

// SecretReadiness reports, for a device, which of the secrets its current config declares
// are set. A key the config names but the device lacks is what makes a fetch fail, so this
// is the read that turns a late device-side 404 into an early, fixable console state.
//
// ok is false when the device has no config at all.
func (s *DeployService) SecretReadinessFor(ctx context.Context, orgID, deviceID string) ([]SecretRequirement, bool, error) {
	record, err := s.configs.Current(ctx, orgID, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	declared, err := DeclaredSecrets(record.Document)
	if err != nil {
		return nil, true, err
	}

	if s.secrets == nil {
		// Without a store the readiness is unknown, so every declared key reads as unset
		// rather than falsely as set.
		return declared, true, nil
	}

	out := make([]SecretRequirement, 0, len(declared))
	byApp := make(map[string]map[string]struct{}, len(declared))

	for _, req := range declared {
		names, ok := byApp[req.App]
		if !ok {
			set, listErr := s.secrets.SetSecretNames(ctx, deviceID, req.App)
			if listErr != nil {
				return nil, true, listErr
			}

			names = make(map[string]struct{}, len(set))
			for _, name := range set {
				names[name] = struct{}{}
			}
			byApp[req.App] = names
		}

		_, present := names[req.Key]
		out = append(out, SecretRequirement{App: req.App, Key: req.Key, Set: present})
	}

	return out, true, nil
}

// DeclaredSecrets reads the secret names a signed config declares, per app.
//
// It parses the document with the format's own parser rather than matching text, so a
// comment or another app's node cannot be mistaken for a declaration.
func DeclaredSecrets(document string) ([]SecretRequirement, error) {
	doc, err := libv.Parse(document)
	if err != nil {
		return nil, err
	}

	var out []SecretRequirement

	for _, appNode := range doc.Root.Find("device.apps.app") {
		if len(appNode.Tokens) < 2 {
			continue
		}
		app := appNode.Tokens[1]

		for _, child := range appNode.Children {
			if child.Key() == "secret" && len(child.Tokens) > 1 {
				out = append(out, SecretRequirement{App: app, Key: child.Tokens[1]})
			}
		}
	}

	return out, nil
}

// CurrentDocument returns the signed config a device currently holds.
//
// ok is false when the device has no config, which the route turns into a 404 rather
// than serving an empty body a client would treat as a valid document.
//
// It takes the org: the document names the device's apps and their secret names, so it
// is tenant material, and a device outside the caller's organisation must read as
// not-found rather than as a document.
func (s *DeployService) CurrentDocument(ctx context.Context, orgID, deviceID string) (string, int, bool, error) {
	record, err := s.configs.Current(ctx, orgID, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}

	return record.Document, record.Version, true, nil
}

// Rollback restores a device's last-good config as its current one.
//
// The stored document is reused verbatim rather than recompiled: it is already
// signed at a known version, and rebuilding it would issue a different document
// than the one that previously ran on the device. The restored record is promoted
// to current by SaveCurrent, which demotes the config being replaced, so the
// retention rule (two versions) still holds.
//
// Provisioning calls this when a manifest disappears from the repository and the
// caller explicitly asked for revert_missing: without it, "reverted" would be a
// message describing work that never happened.
func (s *DeployService) Rollback(ctx context.Context, caller *Caller, orgID, deviceID string) (*deployconfig.Record, error) {
	if caller == nil || caller.OrgID != orgID {
		return nil, &DeniedError{Denial: DenialNotInOrg}
	}

	if _, err := AuthorizeDeploy(ctx, s.lookup, caller, deviceID); err != nil {
		return nil, err
	}

	lastGood, err := s.configs.LastGood(ctx, orgID, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, ErrNothingToRevert
	}
	if err != nil {
		return nil, err
	}

	restored := &deployconfig.Record{
		DeviceID:  deviceID,
		Kind:      deployconfig.KindCurrent,
		Version:   lastGood.Version,
		Document:  lastGood.Document,
		CreatedAt: s.now(),
	}
	if err := s.configs.SaveCurrent(ctx, restored); err != nil {
		return nil, err
	}

	// A rollback changes what the device runs, so it is recorded where it happens rather
	// than left to whichever caller invoked it: provisioning's revert_missing path is the
	// only caller today, and an audit trail that depends on the caller remembering is not
	// one.
	s.auditRollback(ctx, orgID, deviceID, restored.Version)

	return restored, nil
}

// auditRollback writes the rollback event when a sink is wired.
func (s *DeployService) auditRollback(ctx context.Context, orgID, deviceID string, version int) {
	if s.audit == nil {
		return
	}

	s.audit.DeployFromService(ctx, DeployEvent{
		OrgID:    orgID,
		DeviceID: deviceID,
		Version:  version,
		Action:   ActionRolledBack,
		Result:   ResultSuccess,
		Detail:   "restored the last known-good config",
	})
}

// CurrentForDevice returns the signed config a device should be running.
//
// There is no caller here on purpose: this read is the device's own, made with
// the device's own credential, and the handler has already bound the request to
// that device. It is not the user path, which goes through Current and its
// authorisation.
func (s *DeployService) CurrentForDevice(ctx context.Context, deviceID string) (*deployconfig.Record, error) {
	record, err := s.configs.CurrentForDevice(ctx, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, ErrNoConfig
	}

	return record, err
}

// checkDeclaredSecrets refuses a manifest that names a secret the device has not been
// given.
//
// The check reads names, never values: it asks the store which keys are set and compares
// the set. A service with no store wired skips it, so the check is a property of the
// running server rather than of every unit test.
func (s *DeployService) checkDeclaredSecrets(ctx context.Context, deviceID string, m *Manifest) error {
	if s.secrets == nil {
		return nil
	}

	for i := range m.Apps {
		app := &m.Apps[i]
		if len(app.Secrets) == 0 {
			continue
		}

		set, err := s.secrets.SetSecretNames(ctx, deviceID, app.Name)
		if err != nil {
			return err
		}

		have := make(map[string]struct{}, len(set))
		for _, name := range set {
			have[name] = struct{}{}
		}

		for _, required := range app.Secrets {
			if _, ok := have[required]; ok {
				continue
			}

			return &SecretError{
				App:     app.Name,
				Key:     required,
				Message: "is declared by the manifest but has not been set for this device",
			}
		}
	}

	return nil
}

// persist stores the new current config, which demotes the previous one.
func (s *DeployService) persist(ctx context.Context, deviceID string, version int, document string) error {
	return s.configs.SaveCurrent(ctx, &deployconfig.Record{
		DeviceID:  deviceID,
		Kind:      deployconfig.KindCurrent,
		Version:   version,
		Document:  document,
		CreatedAt: s.now(),
	})
}

// currentVersion returns the device's current version, or 0 when it has none.
func (s *DeployService) currentVersion(ctx context.Context, orgID, deviceID string) (int, error) {
	current, err := s.configs.Current(ctx, orgID, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return current.Version, nil
}

// resultFromConfig builds the user-facing summary.
func resultFromConfig(cfg *Config, version int, dryRun bool, catalog *Catalog) *DeployResult {
	result := &DeployResult{
		Version:      version,
		AuthorizedBy: cfg.MemberID,
		AppNames:     make([]string, 0, len(cfg.Apps)),
		DryRun:       dryRun,
	}

	for i := range cfg.Apps {
		result.AppNames = append(result.AppNames, cfg.Apps[i].Name)
		result.Artifacts = append(result.Artifacts, cfg.Apps[i].Artifacts...)
	}

	result.Apps = catalog.Mark(result.AppNames)

	return result
}

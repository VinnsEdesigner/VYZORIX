package deploy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// The deploy orchestration: authorize, compile, sign, persist. The handler is a
// thin adapter over this, so the sequence and its ordering live in one place and
// are testable without HTTP.

// DeployRequest is one operator's deploy. The caller is resolved by the handler,
// which is the authentication boundary, and passed in so the service never has to
// reach back into a request context it does not own.
//
// DeviceID is the target. On `POST /v1/deploy` the path names no device, so the
// handler leaves it empty and the service takes it from the manifest's `device.id`
// (§3, §4.1) — the file is the declaration, and the endpoint is the target.
type DeployRequest struct {
	Caller   *Caller
	OrgID    string
	DeviceID string
	MemberID string
	Manifest []byte
	DryRun   bool
}

// DeployResult is what the operator is told.
type DeployResult struct {
	AuthorizedBy string
	Document     string
	Artifacts    []ArtifactConfig
	AppNames     []string
	Version      int
	DryRun       bool
}

// DeployService orchestrates a deploy end to end.
type DeployService struct {
	configs  deployconfig.Repository
	lookup   DeviceLookup
	sign     DocumentSigner
	settings SettingsSource
	now      func() time.Time
}

// SettingsSource supplies the server-owned settings a compile injects: the
// tier cap, the stream path and the feature flags. They come from the device
// record and the server's policy, never from the manifest.
type SettingsSource interface {
	ForDevice(ctx context.Context, dev *device.Device) Injection
}

// NewDeployService builds a DeployService.
func NewDeployService(
	configs deployconfig.Repository,
	lookup DeviceLookup,
	sign DocumentSigner,
	settings SettingsSource,
) *DeployService {
	return &DeployService{
		configs:  configs,
		lookup:   lookup,
		sign:     sign,
		settings: settings,
		now:      time.Now,
	}
}

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

	injection := s.settings.ForDevice(ctx, dev)
	injection.MemberID = req.MemberID
	injection.DeviceID = deviceID
	injection.OrgID = req.OrgID

	current, err := s.currentVersion(ctx, deviceID)
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

	result := resultFromConfig(cfg, injection.Version, req.DryRun)

	if req.DryRun {
		result.Document = document

		return result, nil
	}

	if err := s.persist(ctx, deviceID, injection.Version, document); err != nil {
		return nil, err
	}

	return result, nil
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

	record, err := s.configs.Current(ctx, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, ErrNoConfig
	}

	return record, err
}

// ErrNoConfig is returned when a device has never been deployed to.
var ErrNoConfig = errors.New("device has no config")

// CurrentForDevice returns the signed config a device should be running.
//
// There is no caller here on purpose: this read is the device's own, made with
// the device's own credential, and the handler has already bound the request to
// that device. It is not the operator path, which goes through Current and its
// authorisation.
func (s *DeployService) CurrentForDevice(ctx context.Context, deviceID string) (*deployconfig.Record, error) {
	record, err := s.configs.Current(ctx, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return nil, ErrNoConfig
	}

	return record, err
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
func (s *DeployService) currentVersion(ctx context.Context, deviceID string) (int, error) {
	current, err := s.configs.Current(ctx, deviceID)
	if errors.Is(err, deployconfig.ErrNotFound) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return current.Version, nil
}

// resultFromConfig builds the operator-facing summary.
func resultFromConfig(cfg *Config, version int, dryRun bool) *DeployResult {
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

	return result
}

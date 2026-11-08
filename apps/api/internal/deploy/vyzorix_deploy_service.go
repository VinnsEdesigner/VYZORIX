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

// DeployRequest is one operator's deploy.
type DeployRequest struct {
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
	caller   func(ctx context.Context, orgID string) (*Caller, error)
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
	caller func(ctx context.Context, orgID string) (*Caller, error),
	lookup DeviceLookup,
	sign DocumentSigner,
	settings SettingsSource,
) *DeployService {
	return &DeployService{
		configs:  configs,
		caller:   caller,
		lookup:   lookup,
		sign:     sign,
		settings: settings,
		now:      time.Now,
	}
}

// Deploy runs the sequence for one request.
func (s *DeployService) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	caller, err := s.caller(ctx, req.OrgID)
	if err != nil {
		return nil, err
	}

	// The caller is resolved for the requested org, but the org it is actually
	// in is the authority. A resolver that returned a caller for a different
	// org would otherwise let a deploy land in the wrong tenant.
	if caller == nil || caller.OrgID != req.OrgID {
		return nil, &DeniedError{Denial: DenialNotInOrg}
	}

	// Authorize before parsing: an unauthorised caller must not learn whether the
	// device exists, and must not reach the compiler at all.
	dev, err := AuthorizeDeploy(ctx, s.lookup, caller, req.DeviceID)
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
	if manifest.Device.ID != "" && manifest.Device.ID != req.DeviceID {
		return nil, &ManifestError{
			Field:   "device.id",
			Message: fmt.Sprintf("names %q, but the deploy targets %q", manifest.Device.ID, req.DeviceID),
		}
	}

	injection := s.settings.ForDevice(ctx, dev)
	injection.MemberID = req.MemberID
	injection.DeviceID = req.DeviceID
	injection.OrgID = req.OrgID

	current, err := s.currentVersion(ctx, req.DeviceID)
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
		result.Version = current
		result.Document = document

		return result, nil
	}

	if err := s.persist(ctx, req.DeviceID, injection.Version, document); err != nil {
		return nil, err
	}

	return result, nil
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

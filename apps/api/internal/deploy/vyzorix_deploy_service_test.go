package deploy

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// memConfigs is an in-memory deployconfig.Repository.
type memConfigs struct {
	records map[string]*deployconfig.Record
}

func newMemConfigs() *memConfigs {
	return &memConfigs{records: map[string]*deployconfig.Record{}}
}

func (m *memConfigs) Current(_ context.Context, deviceID string) (*deployconfig.Record, error) {
	rec, ok := m.records[deviceID]
	if !ok {
		return nil, deployconfig.ErrNotFound
	}

	return rec, nil
}

func (m *memConfigs) LastGood(_ context.Context, _ string) (*deployconfig.Record, error) {
	return nil, deployconfig.ErrNotFound
}

func (m *memConfigs) SaveCurrent(_ context.Context, record *deployconfig.Record) error {
	m.records[record.DeviceID] = record

	return nil
}

func (m *memConfigs) MarkApplied(_ context.Context, _ string, _ int, _ time.Time) error {
	return nil
}

// memDevices is a DeviceRepository over a fixed set of devices.
type memDevices struct {
	devices map[string]*device.Device
}

func (m *memDevices) FindByIDAndOrganization(_ context.Context, id, orgID string) (*device.Device, error) {
	dev, ok := m.devices[id]
	if !ok || dev.OrganizationID != orgID {
		return nil, ErrDeviceNotInOrg
	}

	return dev, nil
}

// FindForOrg satisfies DeviceLookup, the org-scoped lookup the predicate uses.
func (m *memDevices) FindForOrg(ctx context.Context, id, orgID string) (*device.Device, error) {
	return m.FindByIDAndOrganization(ctx, id, orgID)
}

// fixedSettings is a SettingsSource with a fixed injection.
type fixedSettings struct{}

func (fixedSettings) ForDevice(_ context.Context, _ *device.Device) Injection {
	return Injection{
		Name:             "field-ops-07",
		StreamPath:       "/v1/device/d/stream",
		TierCap:          2,
		HeartbeatSeconds: 60,
		TerminalEnabled:  true,
	}
}

// serviceFixture builds a DeployService over in-memory stores.
func serviceFixture(t *testing.T, caller *Caller) (*DeployService, *memConfigs, *Caller) {
	t.Helper()

	sign, _ := signer(t)

	configs := newMemConfigs()
	devices := &memDevices{devices: map[string]*device.Device{
		"d_01H": {ID: "d_01H", OrganizationID: "org_01H", Lifecycle: device.LifecycleRegistered},
	}}

	svc := NewDeployService(
		configs,
		devices,
		sign,
		fixedSettings{},
	)
	svc.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }

	return svc, configs, caller
}

func deployCaller() *Caller {
	return &Caller{
		Authenticated: true,
		OrgID:         "org_01H",
		Permissions: permission.ScopedPermissions{
			{Action: permission.ActionDeviceDeploy, Scope: "devices:d_01H"},
		},
	}
}

// TestDeployPersistsAndReturnsTheVersion covers the happy path end to end.
func TestDeployPersistsAndReturnsTheVersion(t *testing.T) {
	t.Parallel()

	svc, configs, caller := serviceFixture(t, deployCaller())

	result, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(twoAppManifest),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}

	if result.Version != 1 {
		t.Fatalf("first deploy must be version 1, got %d", result.Version)
	}

	if result.AuthorizedBy != "op_01H" {
		t.Fatalf("the result must name the authoriser, got %q", result.AuthorizedBy)
	}

	if len(result.AppNames) != 2 {
		t.Fatalf("expected two apps, got %v", result.AppNames)
	}

	stored, err := configs.Current(context.Background(), "d_01H")
	if err != nil {
		t.Fatalf("the config must be persisted: %v", err)
	}

	if !strings.Contains(stored.Document, "v sig ecdsa-p256") {
		t.Fatal("the persisted document must be signed")
	}
}

// TestDeployVersionIsMonotonic proves a second deploy increments.
func TestDeployVersionIsMonotonic(t *testing.T) {
	t.Parallel()

	svc, _, caller := serviceFixture(t, deployCaller())
	ctx := context.Background()

	req := DeployRequest{Caller: caller, Manifest: []byte(twoAppManifest), OrgID: "org_01H", DeviceID: "d_01H", MemberID: "op_01H"}

	first, err := svc.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := svc.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if second.Version <= first.Version {
		t.Fatalf("version must increase: %d -> %d", first.Version, second.Version)
	}
}

// TestDryRunStoresNothing is the CLI guarantee: compile and authorise, then stop.
func TestDryRunStoresNothing(t *testing.T) {
	t.Parallel()

	svc, configs, caller := serviceFixture(t, deployCaller())

	result, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(twoAppManifest),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}

	if !result.DryRun {
		t.Fatal("the result must be marked a dry run")
	}

	if result.Document == "" {
		t.Fatal("a dry run must return the document for inspection")
	}

	// The version the operator is shown must be the one in the document they were
	// shown, or the preview describes a different deploy than the one it prints.
	if want := "v version " + strconv.Itoa(result.Version); !strings.Contains(result.Document, want) {
		t.Fatalf("the dry-run version %d is not the one in the document", result.Version)
	}

	if _, err := configs.Current(context.Background(), "d_01H"); !errors.Is(err, deployconfig.ErrNotFound) {
		t.Fatal("a dry run must not persist")
	}
}

// TestUnauthorizedCallerNeverReachesTheCompiler proves the ordering: an
// unauthorised caller is refused before the manifest is parsed, so a malformed
// manifest cannot be used to probe.
func TestUnauthorizedCallerNeverReachesTheCompiler(t *testing.T) {
	t.Parallel()

	caller := &Caller{Authenticated: true, OrgID: "org_01H"} // no grant
	svc, _, _ := serviceFixture(t, caller)

	_, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte("this is not valid toml [[["),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
	})

	if got := denialOf(t, err); got != DenialNoGrant {
		t.Fatalf("an unauthorised caller must be refused for lack of grant, got %q", got)
	}
}

// TestCrossOrgDeployIsRefused proves the org binding survives the whole path.
func TestCrossOrgDeployIsRefused(t *testing.T) {
	t.Parallel()

	caller := deployCaller()
	svc, _, _ := serviceFixture(t, caller)

	_, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(twoAppManifest),
		OrgID:    "org_other",
		DeviceID: "d_01H",
	})

	if got := denialOf(t, err); got != DenialNotInOrg {
		t.Fatalf("a cross-org deploy must be refused, got %q", got)
	}
}

// TestManifestNamingAnotherDeviceIsRefused proves the file cannot retarget.
func TestManifestNamingAnotherDeviceIsRefused(t *testing.T) {
	t.Parallel()

	svc, _, caller := serviceFixture(t, deployCaller())

	other := strings.Replace(twoAppManifest, `id  = "d_01H"`, `id  = "d_OTHER"`, 1)

	_, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(other),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	})
	if err == nil {
		t.Fatal("a manifest naming another device must be refused")
	}

	if !strings.Contains(err.Error(), "d_OTHER") {
		t.Fatalf("the refusal must name both devices, got %q", err)
	}
}

// TestDeployRefusesAManifestCarryingASecret proves the scan runs on this path.
func TestDeployRefusesAManifestCarryingASecret(t *testing.T) {
	t.Parallel()

	svc, _, caller := serviceFixture(t, deployCaller())

	bad := strings.Replace(twoAppManifest, `MI_PORT = "8080"`, `API_TOKEN = "short"`, 1)

	_, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(bad),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	})
	if err == nil {
		t.Fatal("a manifest whose env names a secret must be refused")
	}
}

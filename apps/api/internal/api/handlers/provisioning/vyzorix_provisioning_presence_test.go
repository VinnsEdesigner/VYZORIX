package provisioninghandler

import (
	"context"
	"testing"
	"time"

	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// stubPresence answers the resource list's presence read with a fixed map.
type stubPresence struct {
	byID map[string]svc.DeviceEvidence
	err  error
}

func (s stubPresence) PresenceByIDs(_ context.Context, _ string, _ []string) (map[string]svc.DeviceEvidence, error) {
	return s.byID, s.err
}

func presenceVersion(v int) *int { return &v }

// TestResourceJSONCarriesPresence proves the resource payload says whether the device can be
// seen. Without it a console shows "applied" from a stored row and cannot tell the difference
// between a device that just confirmed and one that has been dark for a week.
func TestResourceJSONCarriesPresence(t *testing.T) {
	reportedAt := time.Now().Add(-time.Minute)
	lastSeen := time.Now()

	res := &domain.Resource{
		ID: "res-1", Kind: domain.KindDevice, DeviceID: "dev-1",
		Path: "devices/dev-1/vyzorix.toml", State: domain.ResourceApplied, ConfigVersion: 4,
	}

	out := toResourceJSON(res, svc.DeviceEvidence{
		Found: true, Online: false, ReportedVersion: presenceVersion(4),
		ReportedAt: reportedAt, LastSeen: lastSeen,
	})

	if out.DeviceOnline == nil || *out.DeviceOnline {
		t.Fatalf("device_online = %v, want a false value for an unreachable device", out.DeviceOnline)
	}
	if out.DeviceReportedVersion == nil || *out.DeviceReportedVersion != 4 {
		t.Fatalf("device_reported_version = %v, want 4", out.DeviceReportedVersion)
	}
	if out.DeviceReportedAt == nil || *out.DeviceReportedAt != reportedAt.UnixMilli() {
		t.Fatalf("device_reported_at = %v, want %d", out.DeviceReportedAt, reportedAt.UnixMilli())
	}
	if out.DeviceLastSeen == nil || *out.DeviceLastSeen != lastSeen.UnixMilli() {
		t.Fatalf("device_last_seen = %v, want %d", out.DeviceLastSeen, lastSeen.UnixMilli())
	}
}

// An org resource has no device, so it must carry no presence at all rather than a zero-value
// "offline".
func TestResourceJSONOmitsPresenceForOrgResources(t *testing.T) {
	res := &domain.Resource{ID: "res-2", Kind: domain.KindAlerts, Path: "orgs/acme/alerts.toml", State: domain.ResourceApplied}

	out := toResourceJSON(res, svc.DeviceEvidence{Found: true, Online: true, ReportedVersion: presenceVersion(4)})

	if out.DeviceOnline != nil {
		t.Fatalf("device_online = %v, want nil for an org resource", out.DeviceOnline)
	}
	if out.DeviceReportedVersion != nil || out.DeviceReportedAt != nil || out.DeviceLastSeen != nil {
		t.Fatal("an org resource must carry no device presence")
	}
}

// A device the server could not read is unknown, not offline: the difference is what stops the
// console claiming a device is down when a query merely failed.
func TestResourceJSONLeavesUnknownPresenceEmpty(t *testing.T) {
	res := &domain.Resource{ID: "res-3", Kind: domain.KindDevice, DeviceID: "dev-3", Path: "devices/dev-3/vyzorix.toml"}

	out := toResourceJSON(res, svc.DeviceEvidence{})

	if out.DeviceOnline != nil {
		t.Fatalf("device_online = %v, want nil when the device could not be read", out.DeviceOnline)
	}
}

// presenceFor must never turn a read failure into an offline verdict; it returns no presence
// and the list renders "unknown".
func TestPresenceForSwallowsAReadFailure(t *testing.T) {
	h := &SyncHandler{presence: stubPresence{err: context.DeadlineExceeded}}

	got := h.presenceFor(context.Background(), "org-1", []*domain.Resource{
		{ID: "res-1", DeviceID: "dev-1"},
	})

	if got != nil {
		t.Fatalf("presenceFor = %v, want nil when the read failed", got)
	}
}

// With no presence reader wired the list still works; it just has no presence detail.
func TestPresenceForWithoutAReader(t *testing.T) {
	h := &SyncHandler{}

	if got := h.presenceFor(context.Background(), "org-1", []*domain.Resource{{ID: "res-1", DeviceID: "dev-1"}}); got != nil {
		t.Fatalf("presenceFor = %v, want nil with no reader wired", got)
	}
}

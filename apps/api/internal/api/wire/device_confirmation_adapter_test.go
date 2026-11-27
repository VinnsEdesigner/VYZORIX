package wire

import (
	"context"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// stubDeviceFinder answers with one device, or an error, so the adapter's scoping and
// presence logic can be tested without a database. `byID` lets a test hold more than one
// device, and an id absent from both maps reads as not found.
type stubDeviceFinder struct {
	dev  *device.Device
	byID map[string]*device.Device
	err  error
}

func (s stubDeviceFinder) FindByID(_ context.Context, id string) (*device.Device, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.byID != nil {
		return s.byID[id], nil
	}

	return s.dev, nil
}

// stubLive answers the hub's "is there a socket right now" question.
type stubLive struct {
	online bool
}

func (s stubLive) Online(_ string) bool { return s.online }

func intPtr(v int) *int { return &v }

// TestAdapterReportsPresenceAndReport is the guard for the adapter's job: it must carry the
// reported version and the timestamps, because the sync's freshness check reads them. An
// adapter that returned only a version would let a week-old report read as current.
func TestAdapterReportsPresenceAndReport(t *testing.T) {
	reportedAt := time.Now().Add(-time.Minute).UnixMilli()
	lastSeen := time.Now().UnixMilli()

	adapter := newDeviceConfirmationAdapter(stubDeviceFinder{dev: &device.Device{
		ID:                    "dev-1",
		OrganizationID:        "org-1",
		Online:                true,
		ReportedConfigVersion: intPtr(7),
		ReportedAt:            reportedAt,
		LastSeen:              lastSeen,
	}}, stubLive{online: false})

	got := adapter.ConfirmConfig(context.Background(), "org-1", "dev-1", 7)

	if !got.Found {
		t.Fatal("Found = false for a device that exists in the organisation")
	}
	if got.ReportedVersion == nil || *got.ReportedVersion != 7 {
		t.Fatalf("ReportedVersion = %v, want 7", got.ReportedVersion)
	}
	if got.ReportedAt.UnixMilli() != reportedAt {
		t.Fatalf("ReportedAt = %v, want %d", got.ReportedAt, reportedAt)
	}
	if got.LastSeen.UnixMilli() != lastSeen {
		t.Fatalf("LastSeen = %v, want %d", got.LastSeen, lastSeen)
	}
	// The hub says no socket, so the stored online column must not be trusted over it.
	if got.Online {
		t.Fatal("Online = true, want the live connection state (false) to win over the stored column")
	}
}

// A device in another organisation is not this organisation's to manage, so it must read as
// absent rather than as evidence.
func TestAdapterScopesToTheOrganisation(t *testing.T) {
	adapter := newDeviceConfirmationAdapter(stubDeviceFinder{dev: &device.Device{
		ID: "dev-1", OrganizationID: "org-2", Online: true, ReportedConfigVersion: intPtr(7),
	}}, stubLive{online: true})

	if got := adapter.ConfirmConfig(context.Background(), "org-1", "dev-1", 7); got.Found {
		t.Fatal("Found = true for a device in another organisation")
	}
}

// An unreadable device is "cannot verify", never "verified false": the sync distinguishes the
// two and words its pending reason differently.
func TestAdapterReportsUnknownWhenTheDeviceCannotBeRead(t *testing.T) {
	adapter := newDeviceConfirmationAdapter(stubDeviceFinder{dev: nil}, nil)

	if got := adapter.ConfirmConfig(context.Background(), "org-1", "dev-1", 1); got.Found {
		t.Fatal("Found = true for a device that could not be read")
	}
}

// A never-reported device keeps a nil version, so "no report" stays distinguishable from a
// reported zero.
func TestAdapterKeepsNoReportDistinct(t *testing.T) {
	adapter := newDeviceConfirmationAdapter(stubDeviceFinder{dev: &device.Device{ID: "dev-1", OrganizationID: "org-1"}}, stubLive{})

	got := adapter.ConfirmConfig(context.Background(), "org-1", "dev-1", 0)
	if !got.Found {
		t.Fatal("Found = false for a registered device")
	}
	if got.ReportedVersion != nil {
		t.Fatalf("ReportedVersion = %v, want nil for a device that never reported", got.ReportedVersion)
	}
	if !got.ReportedAt.IsZero() {
		t.Fatalf("ReportedAt = %v, want the zero time", got.ReportedAt)
	}
}

// PresenceByIDs must skip the blank id an org resource carries and the device it cannot read,
// so the list renders "unknown" instead of inventing an offline device.
func TestPresenceByIDsSkipsUnreadableAndBlank(t *testing.T) {
	adapter := newDeviceConfirmationAdapter(stubDeviceFinder{byID: map[string]*device.Device{
		"dev-1": {ID: "dev-1", OrganizationID: "org-1", Online: true, ReportedConfigVersion: intPtr(2)},
	}}, stubLive{online: true})

	got, err := adapter.PresenceByIDs(context.Background(), "org-1", []string{"", "dev-1", "dev-missing"})
	if err != nil {
		t.Fatalf("PresenceByIDs: %v", err)
	}
	if _, present := got[""]; present {
		t.Fatal("the blank id must not appear in the map")
	}
	if _, present := got["dev-missing"]; present {
		t.Fatal("an unreadable device must be absent, not reported as offline")
	}
	if !got["dev-1"].Online {
		t.Fatal("dev-1 should read online from the live connection")
	}
}

// A nil adapter is never built (the caller skips wiring), but a nil finder must not panic in
// the constructor.
func TestNewAdapterIsNilWithoutADeviceStore(t *testing.T) {
	if newDeviceConfirmationAdapter(nil, stubLive{}) != nil {
		t.Fatal("a nil device store must yield a nil adapter")
	}
}

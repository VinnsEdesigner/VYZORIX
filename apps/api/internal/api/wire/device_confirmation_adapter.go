package wire

import (
	"context"
	"time"

	provisioninghandler "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/provisioning"
	provisioningapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// deviceFinder is the device read the confirmation adapter needs.
type deviceFinder interface {
	FindByID(ctx context.Context, id string) (*device.Device, error)
}

// liveConnection tells the adapter whether a device holds a live socket right now.
//
// It is separate from the stored `online` column on purpose. The column is written on
// connect and disconnect, so after a crash it can claim a device is online when nothing is
// connected; the hub's own map is the only thing that knows about the socket in front of it.
type liveConnection interface {
	Online(deviceID string) bool
}

// deviceConfirmationAdapter answers "is this device actually running the config we signed?"
// for the provisioning sync engine.
//
// It exists because the deploy service can only prove it stored a signed document. Whether
// the device fetched and applied it is a separate fact, and the only honest source for it is
// what the device itself reports — and, when that report is old, whether it is reachable at
// all. A stored version alone would let a device that has been offline for a week keep a
// resource reading "applied".
type deviceConfirmationAdapter struct {
	devices deviceFinder
	// live may be nil: with no hub wired the adapter falls back to the stored column, which
	// is less current but still better than no evidence at all.
	live liveConnection
}

// newDeviceConfirmationAdapter builds the adapter. A nil finder yields a nil adapter, so the
// caller skips wiring rather than installing a check that always confirms.
func newDeviceConfirmationAdapter(devices deviceFinder, live liveConnection) *deviceConfirmationAdapter {
	if devices == nil {
		return nil
	}

	return &deviceConfirmationAdapter{devices: devices, live: live}
}

// ConfirmConfig reports what the device is and what it last reported running.
//
// A device that is not found, or belongs to another organisation, is `Found == false`: that
// is "we cannot verify this", which the sync records as pending with a message naming the
// cause. A device that exists but reports a different version (or has never reported) is
// `Found == true` with a non-matching `ReportedVersion`.
func (a *deviceConfirmationAdapter) ConfirmConfig(ctx context.Context, orgID, deviceID string, _ int) provisioningapp.DeviceEvidence {
	dev, err := a.devices.FindByID(ctx, deviceID)
	if err != nil || dev == nil {
		return provisioningapp.DeviceEvidence{}
	}

	// A device in another organisation is not this organisation's to manage. Treating it as
	// absent keeps the answer scoped to the caller.
	if dev.OrganizationID != "" && dev.OrganizationID != orgID {
		return provisioningapp.DeviceEvidence{}
	}

	return evidenceFor(dev, a.online(dev))
}

// PresenceByIDs reports presence for several devices in one read, so the resource list does
// not cost one device query per row. A device that cannot be read is simply absent from the
// map, which the caller renders as "unknown" rather than "offline".
func (a *deviceConfirmationAdapter) PresenceByIDs(ctx context.Context, orgID string, deviceIDs []string) (map[string]provisioningapp.DeviceEvidence, error) {
	out := make(map[string]provisioningapp.DeviceEvidence, len(deviceIDs))

	for _, id := range deviceIDs {
		if id == "" {
			continue
		}

		dev, err := a.devices.FindByID(ctx, id)
		if err != nil || dev == nil {
			continue
		}
		if dev.OrganizationID != "" && dev.OrganizationID != orgID {
			continue
		}

		out[id] = evidenceFor(dev, a.online(dev))
	}

	return out, nil
}

// online prefers the live socket and falls back to the stored column.
func (a *deviceConfirmationAdapter) online(dev *device.Device) bool {
	if a.live != nil {
		return a.live.Online(dev.ID)
	}

	return dev.Online
}

// evidenceFor turns a device row into the evidence the sync and the console read.
func evidenceFor(dev *device.Device, online bool) provisioningapp.DeviceEvidence {
	return provisioningapp.DeviceEvidence{
		Found:           true,
		Online:          online,
		ReportedVersion: dev.ReportedConfigVersion,
		ReportedAt:      timeFromMillis(dev.ReportedAt),
		LastSeen:        timeFromMillis(dev.LastSeen),
	}
}

// timeFromMillis reads a stored epoch-millis timestamp as a time, mapping "never" to the
// zero time so callers can tell an absent report from a very old one.
func timeFromMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}

	return time.UnixMilli(ms)
}

// The adapter answers both questions the sync asks — "is it running this version?" and "is
// it reachable, and what did it last report?" — so a change to either contract fails here
// rather than at wiring time.
var (
	_ provisioningapp.DeviceConfirmation = (*deviceConfirmationAdapter)(nil)
	_ provisioninghandler.PresenceSource = (*deviceConfirmationAdapter)(nil)
)

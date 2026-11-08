package deploy

import (
	"context"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// DefaultHeartbeatSeconds is how often a device reports when the server has no
// per-device override. It is a server value, not a manifest one.
const DefaultHeartbeatSeconds = 60

// DeviceSettings derives the server-owned injection from the device record.
//
// Everything here is policy the server owns: a manifest cannot set a tier, a
// stream path or the feature flags (§5.1). The values mirror what the
// registration path already issues for a claimed device, so a deploy and a claim
// produce the same document for the same device.
type DeviceSettings struct{}

// ForDevice returns the injection for one device. The member, device, org and
// version are filled in by the service, which is the only place that knows the
// authenticated caller and the current version.
func (DeviceSettings) ForDevice(_ context.Context, dev *device.Device) Injection {
	return Injection{
		Name:             dev.DeviceName,
		StreamPath:       "/v1/device/" + dev.ID + "/stream",
		TierCap:          int(dev.Tier),
		HeartbeatSeconds: DefaultHeartbeatSeconds,
		TerminalEnabled:  true,
		USBEnabled:       false,
	}
}

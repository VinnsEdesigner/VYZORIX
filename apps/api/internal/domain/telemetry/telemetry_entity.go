package telemetry

import (
	"encoding/json"
	"time"
)

// TelemetryFrame represents a single telemetry data frame from a device.
type TelemetryFrame struct {
	Timestamp    any             `json:"timestamp,omitempty"`
	Type         string          `json:"type"`
	DeviceID     string          `json:"deviceId,omitempty"`
	ActiveDevice string          `json:"activeDevice,omitempty"`
	Raw          json.RawMessage `json:"-"`
	Uptime       int64           `json:"uptime,omitempty"`
	RiskScore    int             `json:"riskScore,omitempty"`
	AudioMode    int             `json:"audioMode,omitempty"`
	BufferLevel  int             `json:"bufferLevel,omitempty"`
	ThermalTemp  float64         `json:"thermalTemp,omitempty"`
	SpeakerOn    bool            `json:"speakerOn,omitempty"`
	// AppVersion is the app version the device reports it is running, so the
	// device record tracks the current build rather than the one at enroll.
	AppVersion string `json:"appVersion,omitempty"`
	// ConfigVersion is the config version the device says it actually holds. It is
	// what the applier recorded on a successful apply, not what was requested, so
	// comparing it to the stored config is what makes drift visible continuously.
	//
	// A pointer, so "the device did not report" (nil) is distinguishable from "the
	// device reports version 0" (no config installed). The first is unknown and must
	// never read as healthy; the second is a real state.
	ConfigVersion *int `json:"configVersion,omitempty"`
	// AppDigests is app name → a digest of the artifacts installed for it. It catches
	// the drift the version cannot: an app edited or replaced in place.
	AppDigests map[string]string `json:"appDigests,omitempty"`
}

// TelemetryEntry represents a stored telemetry record.
type TelemetryEntry struct {
	ID          string    `json:"id"`
	DeviceID    string    `json:"deviceId"`
	ReceivedAt  time.Time `json:"receivedAt"`
	Payload     string    `json:"payload"`
	RiskScore   int       `json:"riskScore"`
	BufferLevel int       `json:"bufferLevel"`
	ThermalTemp float64   `json:"thermalTemp"`
	Uptime      int64     `json:"uptime"`
}

package telemetry

import (
	"encoding/json"
	"time"
)

// TelemetryFrame represents a single telemetry data frame from a device.
type TelemetryFrame struct {
	Timestamp     any               `json:"timestamp,omitempty"`
	AppDigests    map[string]string `json:"appDigests,omitempty"`
	ConfigVersion *int              `json:"configVersion,omitempty"`
	AppVersion    string            `json:"appVersion,omitempty"`
	Type          string            `json:"type"`
	DeviceID      string            `json:"deviceId,omitempty"`
	ActiveDevice  string            `json:"activeDevice,omitempty"`
	Raw           json.RawMessage   `json:"-"`
	RiskScore     int               `json:"riskScore,omitempty"`
	ThermalTemp   float64           `json:"thermalTemp,omitempty"`
	BufferLevel   int               `json:"bufferLevel,omitempty"`
	AudioMode     int               `json:"audioMode,omitempty"`
	Uptime        int64             `json:"uptime,omitempty"`
	SpeakerOn     bool              `json:"speakerOn,omitempty"`
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

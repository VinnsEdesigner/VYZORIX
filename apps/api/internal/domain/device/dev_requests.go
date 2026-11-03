package device

import "encoding/json"

// UpdateDeviceRequest is the payload for updating device settings.
type UpdateDeviceRequest struct {
	FirmwareVersion *string         `json:"firmwareVersion,omitempty"`
	Name            *string         `json:"name,omitempty"`
	Config          json.RawMessage `json:"config,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
}

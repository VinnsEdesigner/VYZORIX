// Package hub provides WebSocket functionality.
package hub

import "context"

// DeviceFrameHandler handles a device-originated WebSocket frame that is not
// telemetry. ReadPump decodes the frame's "type" and hands anything other than
// "telemetry" here, so the transport layer stays free of feature logic while the
// handler (which lives above it, with access to the application services) can
// route command results, PTY frames, app registrations and so on.
//
// Implementations must be safe for concurrent use: one handler serves every
// connected device.
type DeviceFrameHandler interface {
	// HandleDeviceFrame processes one frame. It reports whether the frame type
	// was recognised; an unrecognised frame is logged and dropped by the caller
	// rather than treated as an error, so a newer device cannot break the hub.
	HandleDeviceFrame(ctx context.Context, deviceID, frameType string, raw []byte) bool
}

// SetDeviceFrameHandler installs the handler for non-telemetry device frames.
// It is optional: with no handler set, non-telemetry frames are logged and
// dropped, which is the pre-existing behaviour.
func (h *Hub) SetDeviceFrameHandler(handler DeviceFrameHandler) {
	h.mu.Lock()
	h.deviceFrameHandler = handler
	h.mu.Unlock()
}

// deviceFrameHandlerFor returns the installed handler under the read lock.
func (h *Hub) deviceFrameHandlerFor() DeviceFrameHandler {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.deviceFrameHandler
}

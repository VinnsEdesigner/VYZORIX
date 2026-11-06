// Package hub provides WebSocket functionality.
package hub

import "context"

// FrameKind distinguishes a text (JSON) device frame from a binary one.
type FrameKind uint8

const (
	// FrameText is a JSON frame: the device stream's original protocol, used for
	// telemetry, command results and app registration.
	FrameText FrameKind = iota
	// FrameBinary is a binary frame, used for PTY traffic where per-chunk JSON
	// would mean base64 and escaping on every keystroke and byte of output.
	FrameBinary
)

// DeviceFrame is one inbound frame from a device.
//
// Raw is the original message bytes. gorilla/websocket allocates a fresh slice
// per ReadMessage, so a handler may retain it; it is not reused by the reader.
type DeviceFrame struct {
	DeviceID string
	// Type is the decoded JSON "type" for a text frame, and "" for a binary one.
	Type string
	Raw  []byte
	Kind FrameKind
}

// DeviceFrameHandler handles a device-originated WebSocket frame that is not
// telemetry. ReadPump decodes the frame and hands anything other than
// "telemetry" here, so the transport layer stays free of feature logic while the
// handler (which lives above it, with access to the application services) can
// route command results, PTY frames, app registrations and so on.
//
// It reports whether the frame was recognised; an unrecognised frame is logged
// and dropped by the caller rather than treated as an error, so a newer device
// cannot break the hub.
//
// Implementations must be safe for concurrent use: one handler serves every
// connected device.
type DeviceFrameHandler interface {
	HandleDeviceFrame(ctx context.Context, frame DeviceFrame) bool
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

// SetDeviceDisconnectHook installs a callback invoked when a device's WebSocket
// connection closes. It exists so features that hold per-device state (the PTY
// relay's sessions) can release it: a session cannot outlive the connection it
// runs over, and leaving one behind would strand a shell with no way to reach it.
//
// The hook runs outside the hub lock and must not block: it is called on the
// unregistration path.
func (h *Hub) SetDeviceDisconnectHook(fn func(deviceID string)) {
	h.mu.Lock()
	h.deviceDisconnectHook = fn
	h.mu.Unlock()
}

// deviceDisconnectHookFor returns the installed hook under the read lock.
func (h *Hub) deviceDisconnectHookFor() func(deviceID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.deviceDisconnectHook
}

// SetDeviceConnectHook installs a callback invoked when a device's WebSocket
// connection is (re)established. It is the counterpart to
// SetDeviceDisconnectHook: because a PTY session now outlives the socket, a
// reconnecting device must be told to re-attach the shells it still holds.
//
// Like the disconnect hook, it runs outside the hub lock and must not block: it
// is called on the registration path.
func (h *Hub) SetDeviceConnectHook(fn func(deviceID string)) {
	h.mu.Lock()
	h.deviceConnectHook = fn
	h.mu.Unlock()
}

// deviceConnectHookFor returns the installed hook under the read lock.
func (h *Hub) deviceConnectHookFor() func(deviceID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.deviceConnectHook
}

// DispatchDeviceFrame hands a frame to the installed handler under a panic guard
// and reports whether it was handled.
//
// The guard is load-bearing: ReadPump runs in its own goroutine, and an
// unrecovered panic in any goroutine terminates the whole process. A malformed
// frame that trips a handler bug must therefore drop the frame, not take the
// server down with it. A panic is treated as "not handled" so the caller logs it
// like any other unrecognised frame.
func (h *Hub) DispatchDeviceFrame(ctx context.Context, frame DeviceFrame) (handled bool) {
	handler := h.deviceFrameHandlerFor()
	if handler == nil {
		return false
	}

	defer func() {
		if r := recover(); r != nil {
			handled = false
			if h.log != nil {
				h.log.Error("panic in device frame handler recovered",
					"deviceId", frame.DeviceID,
					"kind", frame.Kind,
					"type", frame.Type,
					"panic", r,
				)
			}
		}
	}()

	return handler.HandleDeviceFrame(ctx, frame)
}

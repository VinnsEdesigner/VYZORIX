// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"log/slog"

	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// PTYDeviceHandler decodes binary PTY frames arriving from a device and drives
// the relay. It implements the PTYDeviceSink the device frame handler calls.
//
// The device only ever sends data and acknowledgements upstream; open, resize
// and close are user-driven and travel the other way.
type PTYDeviceHandler struct {
	relay *hub.PTYRelay
	// users resolves the user side for a session so output can be
	// delivered. It is a lookup rather than a stored reference because the
	// user's connection can be replaced on reconnect.
	users PTYUserLookup
	log       *slog.Logger
}

// PTYUserLookup returns the user endpoint for a session, or nil if the
// session has no live user (the user's browser closed, for instance).
type PTYUserLookup interface {
	UserFor(sessionID string) hub.PTYUser
}

// NewPTYDeviceHandler creates the device-side PTY handler.
func NewPTYDeviceHandler(relay *hub.PTYRelay, users PTYUserLookup, log *slog.Logger) *PTYDeviceHandler {
	if log == nil {
		log = slog.Default()
	}

	return &PTYDeviceHandler{relay: relay, users: users, log: log}
}

// usersFor resolves the user lookup, or nil when it was not wired.
//
// A nil lookup is a wiring mistake, but it must not be a panic: this runs on the
// device's read pump, so dereferencing it would drop the device's connection and
// lose every terminal on it. Returning nil makes it a logged drop instead.
func (h *PTYDeviceHandler) usersFor() PTYUserLookup {
	if h.users == nil {
		h.log.Error("pty device handler has no operator lookup; terminal output cannot be delivered")

		return nil
	}

	return h.users
}

// HandleDeviceBinary handles one binary frame from a device. It returns whether
// the frame was a well-formed PTY frame.
func (h *PTYDeviceHandler) HandleDeviceBinary(deviceID string, raw []byte) bool {
	channel, frameType, payload, err := hub.DecodePTYFrame(raw)
	if err != nil {
		// A malformed frame is dropped, never fatal: the connection stays up so a
		// single bad write does not kill an interactive session.
		h.log.Warn("bad pty frame from device", "deviceId", deviceID, "err", err)

		return false
	}

	if channel != hub.ChannelPTY {
		return false
	}

	switch frameType {
	case hub.PTYTypeData:
		h.handleDeviceData(deviceID, payload)
	case hub.PTYTypeAck:
		// The device acknowledged receipt of user input. Nothing to do on the
		// server side today; the device's own credit accounting governs input.
	case hub.PTYTypeClose:
		h.handleDeviceClose(deviceID, payload)
	case hub.PTYTypeError:
		h.log.Warn("device reported pty error", "deviceId", deviceID, "detail", string(payload))
	default:
		h.log.Debug("unknown pty frame type from device", "deviceId", deviceID, "type", frameType)
	}

	return true
}

// handleDeviceData forwards terminal output to the session's user.
//
// The device sends only the session ID and payload; the relay looks up the
// session, so a device cannot address an arbitrary session by crafting a frame.
func (h *PTYDeviceHandler) handleDeviceData(deviceID string, payload []byte) {
	sessionID, data, err := hub.DecodeSessionData(payload)
	if err != nil {
		h.log.Warn("pty data frame missing session header", "deviceId", deviceID, "err", err)

		return
	}

	session, err := h.relay.Registry().Get(sessionID)
	if err != nil || session.DeviceID != deviceID {
		// Unknown session, or one belonging to another device: drop. The device
		// must not be able to inject output into a session it does not own.
		h.log.Warn("pty data for unknown or foreign session", "deviceId", deviceID, "sessionId", sessionID)

		return
	}

	user := h.usersFor()
	if user == nil {
		return
	}

	op := user.UserFor(sessionID)
	if op == nil {
		return
	}

	h.relay.RelayOutput(sessionID, op, data)
}

// handleDeviceClose ends a session the device closed (its shell exited).
func (h *PTYDeviceHandler) handleDeviceClose(deviceID string, payload []byte) {
	sessionID, _, err := hub.DecodeSessionData(payload)
	if err != nil {
		return
	}

	session, err := h.relay.Registry().Get(sessionID)
	if err != nil || session.DeviceID != deviceID {
		return
	}

	h.relay.CloseSession(sessionID, "device_closed", h.users.UserFor(sessionID))
}

// Compile-time assertion that the device handler satisfies the sink.
var _ PTYDeviceSink = (*PTYDeviceHandler)(nil)

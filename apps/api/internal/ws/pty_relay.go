// Package hub provides WebSocket functionality.
package hub

import (
	"log/slog"
)

// PTYUser is the user-facing side of a session: where a device's output
// is delivered and where its keystrokes come from. The relay never touches the
// user's WebSocket directly; it calls this, so the transport stays in one
// place and the relay is testable without a browser.
type PTYUser interface {
	// DeliverOutput sends device output to the user's terminal. It reports
	// whether the user accepted the frame (that is, had credit); a false
	// return is the signal to throttle the device.
	DeliverOutput(sessionID string, payload []byte) bool
	// NotifyClosed tells the user a session ended.
	NotifyClosed(sessionID string, reason string)
}

// PTYDevice is the device-facing side of a session. Sending to a device goes
// through the hub's ephemeral path so terminal traffic is never persisted.
type PTYDevice interface {
	SendEphemeral(deviceID string, frame []byte) bool
}

// PTYRelay coordinates terminal sessions between users and devices.
//
// It owns no transport: input from the user is handed to RelayInput, output
// from the device to RelayOutput, and the relay applies the session registry
// (credit window, per-device cap) in between. That keeps every transport concern
// in the handlers and every session concern here.
type PTYRelay struct {
	registry *PTYRegistry
	devices  PTYDevice
	log      *slog.Logger
}

// NewPTYRelay creates a relay over a session registry.
func NewPTYRelay(registry *PTYRegistry, devices PTYDevice, log *slog.Logger) *PTYRelay {
	if log == nil {
		log = slog.Default()
	}

	return &PTYRelay{registry: registry, devices: devices, log: log}
}

// Registry exposes the session registry (used by the user handler to open
// and close sessions).
func (r *PTYRelay) Registry() *PTYRegistry {
	return r.registry
}

// SendToDevice sends a control frame to a device, resolving the device from the
// session when deviceID is empty. It is used for open/resize/close, which are
// user-driven and always name a session.
func (r *PTYRelay) SendToDevice(deviceID string, frameType byte, payload []byte) bool {
	if deviceID == "" {
		// Resolve from the session named in the payload.
		sessionID, _, err := DecodeSessionData(payload)
		if err != nil {
			return false
		}

		session, err := r.registry.Get(sessionID)
		if err != nil {
			return false
		}

		deviceID = session.DeviceID
	}

	if r.devices == nil {
		return false
	}

	frame := EncodePTYFrame(ChannelPTY, frameType, payload)
	if !r.devices.SendEphemeral(deviceID, frame) {
		r.log.Debug("pty control frame dropped", "deviceId", deviceID, "type", frameType)

		return false
	}

	return true
}

// RelayInput forwards user keystrokes to the device. It is one-way and
// best-effort: a dropped keystroke is preferable to blocking the user, and
// the terminal echoes locally only when the device cannot be reached.
func (r *PTYRelay) RelayInput(sessionID string, payload []byte) bool {
	session, err := r.registry.Get(sessionID)
	if err != nil {
		return false
	}

	if r.devices == nil {
		return false
	}

	// The payload is session-prefixed: a device multiplexes several sessions over
	// one socket, so a DATA frame that did not name its session would be
	// unroutable. This mirrors the output path, which prefixes the same way.
	frame := EncodePTYFrame(ChannelPTY, PTYTypeData, EncodeSessionData(sessionID, payload))
	if !r.devices.SendEphemeral(session.DeviceID, frame) {
		r.log.Debug("pty input dropped",
			"sessionId", sessionID, "deviceId", session.DeviceID, "bytes", len(payload))

		return false
	}

	r.registry.Touch(sessionID)

	return true
}

// RelayOutput forwards device output to the user, applying the credit
// window. When the user has no credit left the frame is dropped and the
// caller learns it, so the device can be told to pause.
func (r *PTYRelay) RelayOutput(sessionID string, user PTYUser, payload []byte) bool {
	if err := r.registry.TakeCredit(sessionID); err != nil {
		// Out of credit: the user is behind. Drop rather than buffer, so a
		// `cat` of a large file cannot grow server memory without bound.
		r.log.Debug("pty output dropped (no credit)", "sessionId", sessionID, "bytes", len(payload))

		return false
	}

	if !user.DeliverOutput(sessionID, payload) {
		// The user's own buffer was full; return the credit so the window is
		// not consumed by a frame that never arrived.
		r.registry.GrantCredit(sessionID, 1)

		return false
	}

	r.recordOutput(sessionID, payload)
	r.registry.Touch(sessionID)

	return true
}

// recordOutput appends delivered output to the session's cast file when
// recording is on. It records only what the user actually saw, so the cast
// matches the session rather than the device's raw stream.
func (r *PTYRelay) recordOutput(sessionID string, payload []byte) {
	session, err := r.registry.Get(sessionID)
	if err != nil || session.Recorder() == nil {
		return
	}

	if !session.Recorder().WriteOutput(payload) {
		r.log.Warn("pty recording dropped output (size cap reached)", "sessionId", sessionID)
	}
}

// AckCredit returns credit to a session when the user acknowledges frames.
// The cap lives in the registry so a hostile or buggy user cannot inflate
// the window.
func (r *PTYRelay) AckCredit(sessionID string, n int) {
	r.registry.GrantCredit(sessionID, n)
}

// CloseSession tears a session down and tells the user. It is safe to call
// for an unknown session (the device may have already disconnected).
func (r *PTYRelay) CloseSession(sessionID, reason string, user PTYUser) {
	session, err := r.registry.Close(sessionID)
	if err != nil {
		return
	}

	// Best-effort close frame to the device so it can reap the PTY. The device
	// sink may be absent (a relay built only to reap sessions), so guard it.
	//
	// The frame names its session: one device multiplexes several terminals over a
	// single socket, so a close that did not say which session would be
	// unroutable — the device could only guess, and with two terminals open it
	// would reap the wrong one. This mirrors the data path, which prefixes the
	// session for the same reason.
	if r.devices != nil {
		closeFrame := EncodePTYFrame(ChannelPTY, PTYTypeClose, EncodeSessionData(session.SessionID, nil))
		r.devices.SendEphemeral(session.DeviceID, closeFrame)
	}

	if user != nil {
		user.NotifyClosed(sessionID, reason)
	}
}

// ReattachDeviceSessions re-sends an OPEN for every live session on a device.
//
// It is called when a device (re)connects. A session now outlives the socket it
// was opened over: the device keeps the shell running across a drop and buffers
// its output, so a reconnect must be told to re-bind the same session ids rather
// than leaving the user staring at a dead terminal until they re-open one by
// hand. The device's relay treats an OPEN for a session it already holds as a
// re-attach (it flushes buffered output), so this is idempotent.
//
// Sessions are keyed by the id the user chose, and that id is the device's
// too, so the same OPEN the user's first request produced is what re-binds
// the shell. The window size is not known here; the device keeps the size it had,
// and the browser's next RESIZE corrects it if the view changed.
func (r *PTYRelay) ReattachDeviceSessions(deviceID string) int {
	if r.devices == nil {
		return 0
	}

	n := 0
	for _, session := range r.registry.ForDevice(deviceID) {
		payload := EncodeSessionData(session.SessionID, nil)
		if r.SendToDevice(deviceID, PTYTypeOpen, payload) {
			n++
		}
	}

	return n
}

// ExpireSessions closes sessions that have been idle past the TTL.
func (r *PTYRelay) ExpireSessions(user PTYUser) int {
	expired := r.registry.Expired()
	for _, session := range expired {
		r.CloseSession(session.SessionID, "idle_timeout", user)
	}

	return len(expired)
}

// Compile-time assertion that the hub satisfies the device side of the relay.
var _ PTYDevice = (*Hub)(nil)

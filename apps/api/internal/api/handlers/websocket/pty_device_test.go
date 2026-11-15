package websocket

import (
	"testing"

	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// stubUserLookup returns a fixed user for any session.
type stubUserLookup struct{ op hub.PTYUser }

func (s *stubUserLookup) UserFor(string) hub.PTYUser { return s.op }

// captureUser records delivered output.
type captureUser struct{ got [][]byte }

func (c *captureUser) DeliverOutput(_ string, payload []byte) bool {
	c.got = append(c.got, append([]byte(nil), payload...))

	return true
}

func (c *captureUser) NotifyClosed(string, string) {}

func newPTYHandlerWithSession(t *testing.T, deviceID, sessionID string) (*PTYDeviceHandler, *captureUser) {
	t.Helper()

	reg := hub.NewPTYRegistry(hub.PTYRegistryConfig{})
	if _, err := reg.Open(sessionID, deviceID, "op-1", "org-1", false); err != nil {
		t.Fatalf("Open: %v", err)
	}

	relay := hub.NewPTYRelay(reg, nil, discardLogger())
	op := &captureUser{}

	return NewPTYDeviceHandler(relay, &stubUserLookup{op: op}, discardLogger()), op
}

func TestPTYDeviceHandlerRoutesDataToUser(t *testing.T) {
	h, op := newPTYHandlerWithSession(t, "dev-1", "11111111-1111-4111-8111-111111111111")

	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData("11111111-1111-4111-8111-111111111111", []byte("output")))
	if !h.HandleDeviceBinary("dev-1", frame) {
		t.Fatal("a well-formed pty frame must be reported as handled")
	}

	if len(op.got) != 1 || string(op.got[0]) != "output" {
		t.Fatalf("delivered = %v, want [output]", op.got)
	}
}

// TestPTYDeviceHandlerRejectsForeignSession proves a device cannot inject output
// into another device's session by naming its ID.
func TestPTYDeviceHandlerRejectsForeignSession(t *testing.T) {
	h, op := newPTYHandlerWithSession(t, "dev-1", "11111111-1111-4111-8111-111111111111")

	// The frame names s1, but arrives from dev-2.
	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData("11111111-1111-4111-8111-111111111111", []byte("injected")))
	h.HandleDeviceBinary("dev-2", frame)

	if len(op.got) != 0 {
		t.Fatalf("delivered = %v, want nothing for a foreign device", op.got)
	}
}

func TestPTYDeviceHandlerDropsMalformedFrame(t *testing.T) {
	h, _ := newPTYHandlerWithSession(t, "dev-1", "11111111-1111-4111-8111-111111111111")

	// Too short to hold a header.
	if h.HandleDeviceBinary("dev-1", []byte{0x01}) {
		t.Fatal("a malformed frame must report unhandled")
	}

	// Valid frame header but a payload with no session header.
	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, nil)
	h.HandleDeviceBinary("dev-1", frame)
}

func TestPTYDeviceHandlerIgnoresUnknownChannel(t *testing.T) {
	h, _ := newPTYHandlerWithSession(t, "dev-1", "11111111-1111-4111-8111-111111111111")

	frame := hub.EncodePTYFrame(0x7F, hub.PTYTypeData, hub.EncodeSessionData("11111111-1111-4111-8111-111111111111", []byte("x")))
	if h.HandleDeviceBinary("dev-1", frame) {
		t.Fatal("a frame on an unknown channel must report unhandled")
	}
}

func TestPTYDeviceHandlerCloseEndsSession(t *testing.T) {
	h, op := newPTYHandlerWithSession(t, "dev-1", "11111111-1111-4111-8111-111111111111")

	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeClose, hub.EncodeSessionData("11111111-1111-4111-8111-111111111111", nil))
	if !h.HandleDeviceBinary("dev-1", frame) {
		t.Fatal("a close frame must be handled")
	}

	if len(op.got) != 0 {
		t.Fatal("a close frame must not deliver terminal output")
	}
}

// TestPTYDeviceHandlerWithoutUserLookupDoesNotPanic is the guard on the wiring
// bug that made the terminal silently one-way: deps.PTYUsers was declared but
// never populated, so the lookup was nil and the handler panicked on the first byte
// of device output. The panic was recovered by the frame dispatcher, so the user
// saw a shell that opened and then produced nothing, with no error anywhere.
func TestPTYDeviceHandlerWithoutUserLookupDoesNotPanic(t *testing.T) {
	reg := hub.NewPTYRegistry(hub.PTYRegistryConfig{})
	sessionID := "11111111-1111-4111-8111-111111111111"
	if _, err := reg.Open(sessionID, "dev-1", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open: %v", err)
	}

	relay := hub.NewPTYRelay(reg, nil, discardLogger())
	// A nil lookup: the mis-wiring this guards against.
	h := NewPTYDeviceHandler(relay, nil, discardLogger())

	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData(sessionID, []byte("hello")))

	// The frame is still recognised (it is well formed), and the call returns rather
	// than panicking.
	if !h.HandleDeviceBinary("dev-1", frame) {
		t.Fatal("a well-formed data frame must be recognised")
	}
}

func TestDeviceFrameHandlerRoutesBinaryToPTYSink(t *testing.T) {
	h, op := newPTYHandlerWithSession(t, "dev-1", "11111111-1111-4111-8111-111111111111")
	handler := NewDeviceFrameHandler(nil, h, discardLogger())

	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData("11111111-1111-4111-8111-111111111111", []byte("via-frame-handler")))
	if !handler.HandleDeviceFrame(t.Context(), hub.DeviceFrame{DeviceID: "dev-1", Raw: frame, Kind: hub.FrameBinary}) {
		t.Fatal("a binary frame must be routed to the PTY sink")
	}

	if len(op.got) != 1 || string(op.got[0]) != "via-frame-handler" {
		t.Fatalf("delivered = %v, want [via-frame-handler]", op.got)
	}
}

func TestDeviceFrameHandlerWithoutPTYSinkDropsBinary(t *testing.T) {
	handler := NewDeviceFrameHandler(nil, nil, discardLogger())

	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData("11111111-1111-4111-8111-111111111111", []byte("x")))
	if handler.HandleDeviceFrame(t.Context(), hub.DeviceFrame{DeviceID: "dev-1", Raw: frame, Kind: hub.FrameBinary}) {
		t.Fatal("with no PTY sink a binary frame must report unhandled")
	}
}

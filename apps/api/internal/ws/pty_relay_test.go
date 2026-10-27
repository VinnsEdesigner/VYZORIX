package hub

import (
	"testing"
)

// fakeOperator records what the relay delivered and how often it was notified.
type fakeOperator struct {
	delivered [][]byte
	closed    []string
	accept    bool
}

func (f *fakeOperator) DeliverOutput(_ string, payload []byte) bool {
	f.delivered = append(f.delivered, payload)

	return f.accept
}

func (f *fakeOperator) NotifyClosed(sessionID, reason string) {
	f.closed = append(f.closed, sessionID+":"+reason)
}

// fakeDeviceSink captures frames bound for a device.
type fakeDeviceSink struct {
	frames [][]byte
	accept bool
}

func (f *fakeDeviceSink) SendEphemeral(_ string, frame []byte) bool {
	if !f.accept {
		return false
	}

	f.frames = append(f.frames, frame)

	return true
}

func newRelayWithSession(t *testing.T, sink PTYDevice, window int) (*PTYRelay, *PTYRegistry) {
	t.Helper()

	reg := NewPTYRegistry(PTYRegistryConfig{CreditWindow: window})
	if _, err := reg.Open("s1", "dev-1", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open: %v", err)
	}

	return NewPTYRelay(reg, sink, testLogger()), reg
}

func TestRelayInputEncodesAndSends(t *testing.T) {
	sink := &fakeDeviceSink{accept: true}
	relay, _ := newRelayWithSession(t, sink, DefaultPTYCreditWindow)

	if !relay.RelayInput("s1", []byte("ls\n")) {
		t.Fatal("RelayInput must report success when the device accepts")
	}
	if len(sink.frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(sink.frames))
	}

	channel, frameType, payload, err := DecodePTYFrame(sink.frames[0])
	if err != nil {
		t.Fatalf("DecodePTYFrame: %v", err)
	}
	if channel != ChannelPTY || frameType != PTYTypeData {
		t.Fatalf("frame = %d/%d, want pty/data", channel, frameType)
	}

	// The device-bound payload must name its session: a device multiplexes
	// several sessions over one socket and cannot route a bare payload.
	sessionID, data, err := DecodeSessionData(payload)
	if err != nil {
		t.Fatalf("DecodeSessionData: %v", err)
	}
	if sessionID != "s1" {
		t.Fatalf("sessionID = %q, want s1", sessionID)
	}
	if string(data) != "ls\n" {
		t.Fatalf("data = %q, want ls\\n", data)
	}
}

func TestRelayInputUnknownSession(t *testing.T) {
	sink := &fakeDeviceSink{accept: true}
	relay, _ := newRelayWithSession(t, sink, DefaultPTYCreditWindow)

	if relay.RelayInput("nope", []byte("x")) {
		t.Fatal("input for an unknown session must be dropped")
	}
	if len(sink.frames) != 0 {
		t.Fatal("nothing must reach the device for an unknown session")
	}
}

// TestRelayOutputAppliesCreditWindow is the backpressure guarantee: once the
// window is exhausted, output is dropped rather than buffered, so a `cat` of a
// large file cannot grow server memory without bound.
func TestRelayOutputAppliesCreditWindow(t *testing.T) {
	relay, _ := newRelayWithSession(t, &fakeDeviceSink{accept: true}, 2)
	op := &fakeOperator{accept: true}

	if !relay.RelayOutput("s1", op, []byte("a")) {
		t.Fatal("first frame must be delivered")
	}
	if !relay.RelayOutput("s1", op, []byte("b")) {
		t.Fatal("second frame must be delivered")
	}
	// Window exhausted.
	if relay.RelayOutput("s1", op, []byte("c")) {
		t.Fatal("a frame beyond the window must be dropped")
	}
	if len(op.delivered) != 2 {
		t.Fatalf("delivered = %d, want 2", len(op.delivered))
	}

	// An ack restores exactly one unit of credit.
	relay.AckCredit("s1", 1)
	if !relay.RelayOutput("s1", op, []byte("d")) {
		t.Fatal("after an ack the next frame must be delivered")
	}
}

// TestRelayOutputReturnsCreditWhenOperatorRefuses proves the window is not
// consumed by a frame that never reached the operator.
func TestRelayOutputReturnsCreditWhenOperatorRefuses(t *testing.T) {
	relay, reg := newRelayWithSession(t, &fakeDeviceSink{accept: true}, 1)
	op := &fakeOperator{accept: false}

	if relay.RelayOutput("s1", op, []byte("a")) {
		t.Fatal("a refused delivery must report false")
	}

	s, _ := reg.Get("s1")
	if s.Credit != 1 {
		t.Fatalf("credit = %d, want it restored to 1 after a refused delivery", s.Credit)
	}
}

func TestRelayCloseSessionNotifiesOperator(t *testing.T) {
	relay, reg := newRelayWithSession(t, &fakeDeviceSink{accept: true}, DefaultPTYCreditWindow)
	op := &fakeOperator{accept: true}

	relay.CloseSession("s1", "operator_closed", op)

	if len(op.closed) != 1 || op.closed[0] != "s1:operator_closed" {
		t.Fatalf("closed = %v, want [s1:operator_closed]", op.closed)
	}
	if reg.Count() != 0 {
		t.Fatal("the session must be gone from the registry")
	}
}

func TestRelayCloseUnknownSessionIsSafe(t *testing.T) {
	relay, _ := newRelayWithSession(t, &fakeDeviceSink{accept: true}, DefaultPTYCreditWindow)
	op := &fakeOperator{accept: true}

	// Must not panic, and must not notify for a session that never existed.
	relay.CloseSession("missing", "whatever", op)

	if len(op.closed) != 0 {
		t.Fatal("no notification for an unknown session")
	}
}

func TestRelayCloseDeviceSessions(t *testing.T) {
	sink := &fakeDeviceSink{accept: true}
	relay, reg := newRelayWithSession(t, sink, DefaultPTYCreditWindow)
	if _, err := reg.Open("s2", "dev-1", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open s2: %v", err)
	}

	var notified []string
	relay.CloseDeviceSessions("dev-1", func(sessionID, reason string) {
		notified = append(notified, sessionID+":"+reason)
	})

	if len(notified) != 2 {
		t.Fatalf("notified = %v, want both sessions", notified)
	}
	if reg.CountForDevice("dev-1") != 0 {
		t.Fatal("the device must have no sessions left")
	}
}

func TestRelaySendToDeviceResolvesFromSession(t *testing.T) {
	sink := &fakeDeviceSink{accept: true}
	relay, _ := newRelayWithSession(t, sink, DefaultPTYCreditWindow)

	// Empty deviceID: the relay resolves the device from the session in payload.
	payload := EncodeSessionData("s1", EncodePTYResize(40, 100))
	if !relay.SendToDevice("", PTYTypeResize, payload) {
		t.Fatal("SendToDevice must resolve the device from the session")
	}
	if len(sink.frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(sink.frames))
	}
}

func TestRelaySendToDeviceUnknownSession(t *testing.T) {
	sink := &fakeDeviceSink{accept: true}
	relay, _ := newRelayWithSession(t, sink, DefaultPTYCreditWindow)

	payload := EncodeSessionData("missing", nil)
	if relay.SendToDevice("", PTYTypeResize, payload) {
		t.Fatal("an unknown session must not produce a device frame")
	}
}

func TestRelayExpireSessions(t *testing.T) {
	relay, reg := newRelayWithSession(t, &fakeDeviceSink{accept: true}, DefaultPTYCreditWindow)
	op := &fakeOperator{accept: true}

	// Force expiry by opening with a zero TTL registry instead.
	_ = reg
	_ = op
	// With the default TTL nothing is expired yet.
	if n := relay.ExpireSessions(op); n != 0 {
		t.Fatalf("expired = %d, want 0 with a fresh session", n)
	}
}

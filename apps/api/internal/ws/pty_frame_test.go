package hub

import (
	"bytes"
	"testing"
)

func TestPTYFrameRoundTrip(t *testing.T) {
	payload := []byte("hello terminal")

	frame := EncodePTYFrame(ChannelPTY, PTYTypeData, payload)

	channel, frameType, got, err := DecodePTYFrame(frame)
	if err != nil {
		t.Fatalf("DecodePTYFrame: %v", err)
	}
	if channel != ChannelPTY {
		t.Fatalf("channel = %d, want %d", channel, ChannelPTY)
	}
	if frameType != PTYTypeData {
		t.Fatalf("type = %d, want %d", frameType, PTYTypeData)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}

func TestPTYFrameRoundTripEmptyPayload(t *testing.T) {
	frame := EncodePTYFrame(ChannelPTY, PTYTypeClose, nil)

	_, frameType, payload, err := DecodePTYFrame(frame)
	if err != nil {
		t.Fatalf("DecodePTYFrame: %v", err)
	}
	if frameType != PTYTypeClose {
		t.Fatalf("type = %d, want close", frameType)
	}
	if len(payload) != 0 {
		t.Fatalf("payload = %q, want empty", payload)
	}
}

func TestDecodePTYFrameRejectsShortBuffer(t *testing.T) {
	for _, raw := range [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}} {
		if _, _, _, err := DecodePTYFrame(raw); err == nil {
			t.Fatalf("DecodePTYFrame(%v) must fail for a buffer shorter than the header", raw)
		}
	}
}

func TestDecodePTYFrameRejectsOversizedLength(t *testing.T) {
	// Declare a payload far larger than MaxPTYPayload. The decoder must reject it
	// on the declared length, before any allocation based on it.
	raw := make([]byte, PTYFrameHeaderSize)
	raw[0] = ChannelPTY
	raw[1] = PTYTypeData
	raw[2] = 0xFF
	raw[3] = 0xFF

	if _, _, _, err := DecodePTYFrame(raw); err == nil {
		t.Fatal("a frame declaring an oversized payload must be rejected")
	}
}

func TestDecodePTYFrameRejectsTruncatedPayload(t *testing.T) {
	// Header claims 10 bytes, only 3 are present.
	raw := []byte{ChannelPTY, PTYTypeData, 0x00, 0x0A, 'a', 'b', 'c'}

	if _, _, _, err := DecodePTYFrame(raw); err == nil {
		t.Fatal("a frame whose declared length exceeds its bytes must be rejected")
	}
}

func TestPTYResizeRoundTrip(t *testing.T) {
	rows, cols, err := DecodePTYResize(EncodePTYResize(48, 160))
	if err != nil {
		t.Fatalf("DecodePTYResize: %v", err)
	}
	if rows != 48 || cols != 160 {
		t.Fatalf("size = %dx%d, want 48x160", rows, cols)
	}
}

func TestDecodePTYResizeRejectsShortPayload(t *testing.T) {
	if _, _, err := DecodePTYResize([]byte{1, 2, 3}); err == nil {
		t.Fatal("a resize payload shorter than 4 bytes must be rejected")
	}
}

func TestSessionDataRoundTrip(t *testing.T) {
	data := []byte("ls -la\n")

	payload := EncodeSessionData("session-abc", data)

	sessionID, got, err := DecodeSessionData(payload)
	if err != nil {
		t.Fatalf("DecodeSessionData: %v", err)
	}
	if sessionID != "session-abc" {
		t.Fatalf("sessionID = %q, want session-abc", sessionID)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("data = %q, want %q", got, data)
	}
}

func TestSessionDataRoundTripEmptyData(t *testing.T) {
	sessionID, got, err := DecodeSessionData(EncodeSessionData("s1", nil))
	if err != nil {
		t.Fatalf("DecodeSessionData: %v", err)
	}
	if sessionID != "s1" || len(got) != 0 {
		t.Fatalf("got %q/%q, want s1 with empty data", sessionID, got)
	}
}

func TestDecodeSessionDataRejectsMalformed(t *testing.T) {
	cases := map[string][]byte{
		"empty":                  {},
		"zero length id":         {0x00},
		"id longer than payload": {0x05, 'a', 'b'},
		"oversized id":           {byte(MaxPTYSessionIDLen + 1), 'a'},
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := DecodeSessionData(raw); err == nil {
				t.Fatalf("DecodeSessionData(%v) must fail", raw)
			}
		})
	}
}

func TestSessionOpenCarriesTheIdentity(t *testing.T) {
	in := SessionIdentity{
		User:      "op-1",
		Name:      "Kamau",
		Email:     "kamau@example.com",
		Org:       "org-1",
		OrgName:   "Vyzorix",
		Role:      "operator",
		RoleLabel: "Operator",
		Tier:      1,
		ExpiresAt: 1234,
		Allowlist: []string{"camera", "battery"},
	}

	payload := EncodeSessionOpen("sess-1", 24, 80, in)

	id, rows, cols, got, err := DecodeSessionOpen(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id != "sess-1" || rows != 24 || cols != 80 {
		t.Fatalf("header = %q %d %d", id, rows, cols)
	}
	if got == nil {
		t.Fatal("identity missing")
	}
	if got.RoleLabel != "Operator" || got.User != "op-1" || len(got.Allowlist) != 2 {
		t.Fatalf("identity = %+v", got)
	}
}

// A device that predates the identity block still reads the window size: the size
// is the first four bytes after the session id, and the rest is ignored.
func TestSessionOpenWithoutIdentityIsSizeOnly(t *testing.T) {
	payload := EncodeSessionData("sess-2", EncodePTYResize(40, 100))

	id, rows, cols, got, err := DecodeSessionOpen(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id != "sess-2" || rows != 40 || cols != 100 {
		t.Fatalf("header = %q %d %d", id, rows, cols)
	}
	if got != nil {
		t.Fatalf("identity should be nil, got %+v", got)
	}
}

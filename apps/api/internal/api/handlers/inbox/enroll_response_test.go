package inbox

import (
	"encoding/json"
	"testing"
)

// TestEnrollResponseCarriesPollInterval guards the wire contract the device
// depends on: the enroll response must carry pollIntervalSeconds, under exactly
// that name, as a number.
//
// The device parses the field by name and falls back to its own default when the
// field is absent, so a rename here would not fail a build — it would silently
// stop telling devices how often to poll, and a claimed device whose socket was
// down would sit unconfigured. That is why it is pinned by a test.
func TestEnrollResponseCarriesPollInterval(t *testing.T) {
	encoded, err := json.Marshal(enrollResponse{
		DeviceID:            "dev-1",
		PairingCode:         "K7QM3X2P",
		PairingDisplay:      "K7QM-3X2P",
		ExpiresAt:           1767225600000,
		ExpiresInSeconds:    600,
		PollIntervalSeconds: PairingPollIntervalSeconds,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	raw, ok := decoded["pollIntervalSeconds"]
	if !ok {
		t.Fatalf("pollIntervalSeconds missing from the enroll response: %s", encoded)
	}

	// It must be a number, not a string: the device decodes it into a Long.
	if _, ok := raw.(float64); !ok {
		t.Fatalf("pollIntervalSeconds must be a JSON number, got %T", raw)
	}

	if got := int64(raw.(float64)); got != PairingPollIntervalSeconds {
		t.Fatalf("pollIntervalSeconds = %d, want %d", got, PairingPollIntervalSeconds)
	}
}

// TestPairingPollIntervalIsSane pins the cadence itself.
//
// A zero would make the device poll as fast as it can; a value above the pairing
// code's lifetime would make the fallback useless, since the code expires before
// the first poll. Ten seconds is the documented choice.
func TestPairingPollIntervalIsSane(t *testing.T) {
	if PairingPollIntervalSeconds <= 0 {
		t.Fatalf("poll interval must be positive, got %d", PairingPollIntervalSeconds)
	}

	const codeLifetimeSeconds = 600
	if PairingPollIntervalSeconds >= codeLifetimeSeconds {
		t.Fatalf(
			"poll interval %ds must be well under the code lifetime %ds",
			PairingPollIntervalSeconds, codeLifetimeSeconds,
		)
	}
}

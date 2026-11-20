package redaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const probeSecret = "sup3r-s3cret-value-that-must-never-appear"

// TestSecretCannotBePrinted is the core guarantee of the type layer. Every formatting
// and serialization path must yield the mask, because any one of them is a way a secret
// reaches a log or a response.
func TestSecretCannotBePrinted(t *testing.T) {
	s := NewSecret(probeSecret)

	renderings := map[string]string{
		"String()":        s.String(),
		"fmt %v":          fmt.Sprintf("%v", s),
		"fmt %+v":         fmt.Sprintf("%+v", s),
		"fmt %#v":         fmt.Sprintf("%#v", s),
		"fmt %s":          fmt.Sprintf("%s", s),
		"fmt %q":          fmt.Sprintf("%q", s),
		"fmt inside text": fmt.Sprintf("token is %v!", s),
	}

	for name, out := range renderings {
		if strings.Contains(out, probeSecret) {
			t.Errorf("%s leaked the secret: %q", name, out)
		}
	}

	// The mask, or an empty string for a zero value, but never the plaintext.
	if renderings["String()"] != Marker {
		t.Errorf("String() = %q, want %q", renderings["String()"], Marker)
	}
}

// TestSecretCannotBeMarshalled covers the serialization paths a response or a config dump
// would use.
func TestSecretCannotBeMarshalled(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Token Secret `json:"token"`
	}

	encoded, err := json.Marshal(payload{Name: "conn", Token: NewSecret(probeSecret)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), probeSecret) {
		t.Fatalf("JSON leaked the secret: %s", encoded)
	}

	// The struct dump path, which %#v and a naive logger both use.
	if out := fmt.Sprintf("%#v", payload{Token: NewSecret(probeSecret)}); strings.Contains(out, probeSecret) {
		t.Fatalf("struct dump leaked the secret: %s", out)
	}

	// A map holding a Secret, which is how an error context is usually shaped.
	if out := fmt.Sprintf("%v", map[string]any{"token": NewSecret(probeSecret)}); strings.Contains(out, probeSecret) {
		t.Fatalf("map dump leaked the secret: %s", out)
	}
}

// TestSecretIsMaskedInSlog proves the logging path, which is where a leak would do the
// most damage because logs are copied everywhere.
func TestSecretIsMaskedInSlog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Info("connecting", "token", NewSecret(probeSecret), "dsn", "db.internal")

	if strings.Contains(buf.String(), probeSecret) {
		t.Fatalf("slog leaked the secret: %s", buf.String())
	}
	if !strings.Contains(buf.String(), Marker) {
		t.Fatalf("slog did not record the mask: %s", buf.String())
	}
}

// TestSecretRevealIsTheOnlyWayOut documents and pins the escape hatch.
func TestSecretRevealIsTheOnlyWayOut(t *testing.T) {
	s := NewSecret(probeSecret)

	if s.Reveal() != probeSecret {
		t.Fatal("Reveal must return the plaintext; it is the only accessor")
	}
}

// TestSecretZeroIsEmptyNotMasked keeps "not set" distinguishable from "set but hidden".
func TestSecretZeroIsEmptyNotMasked(t *testing.T) {
	var s Secret

	if s.String() != "" {
		t.Fatalf("zero secret String() = %q, want empty", s.String())
	}
	if !s.IsZero() {
		t.Fatal("IsZero must be true for the zero value")
	}
	if fmt.Sprintf("%v", s) != "" {
		t.Fatalf("zero secret rendered as %q, want empty", fmt.Sprintf("%v", s))
	}
}

// TestSecretRoundTripDetectsMask proves a client echoing the mask back can be detected,
// so a write does not overwrite a stored secret with the literal word REDACTED.
func TestSecretRoundTripDetectsMask(t *testing.T) {
	var s Secret
	if err := json.Unmarshal([]byte(`"[REDACTED]"`), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !IsMarker(s.Reveal()) {
		t.Fatal("a mask echoed back must be detectable so the stored value can be kept")
	}
}

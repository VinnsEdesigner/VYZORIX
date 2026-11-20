package redaction

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// Secret is a string that cannot be printed.
//
// Every formatting and serialization path returns the mask, so a secret cannot leak
// through a log call, an error message, a JSON response, or a struct dump. The
// plaintext is reachable only through Reveal, which is a deliberate, greppable call —
// that is the whole point: leaking becomes an explicit act rather than an accident.
//
// This is the strongest layer. A regex over free text is best-effort; a type that
// refuses to render its value is a guarantee, and it holds for %v, %+v, %#v, %s, %q,
// json.Marshal, and any slog attribute.
type Secret string

// Compile-time proof that every rendering path is overridden.
var (
	_ fmt.Stringer   = Secret("")
	_ fmt.Formatter  = Secret("")
	_ fmt.GoStringer = Secret("")
	_ json.Marshaler = Secret("")
)

// NewSecret wraps a plaintext value.
func NewSecret(plaintext string) Secret {
	return Secret(plaintext)
}

// Reveal returns the plaintext. Calling it is the only way to obtain the value, so
// every place a secret is actually needed is visible to a reader and to a grep.
func (s Secret) Reveal() string {
	return string(s)
}

// IsZero reports whether the secret is empty, without revealing it.
func (s Secret) IsZero() bool {
	return s == ""
}

// String implements fmt.Stringer.
func (s Secret) String() string {
	if s.IsZero() {
		return ""
	}

	return Marker
}

// Format implements fmt.Formatter, covering %v, %+v, %#v, %s, and %q.
//
// A zero secret formats as empty rather than as the mask, so "not set" stays
// distinguishable from "set but hidden" in a log line.
func (s Secret) Format(f fmt.State, _ rune) {
	if s.IsZero() {
		_, _ = f.Write([]byte{})

		return
	}

	_, _ = f.Write([]byte(Marker))
}

// GoString implements fmt.GoStringer, which %#v uses.
func (s Secret) GoString() string {
	if s.IsZero() {
		return `redaction.Secret("")`
	}

	return "redaction.Secret(" + Marker + ")"
}

// MarshalJSON implements json.Marshaler.
func (s Secret) MarshalJSON() ([]byte, error) {
	if s.IsZero() {
		return []byte(`""`), nil
	}

	return json.Marshal(Marker)
}

// MarshalYAML implements yaml.Marshaler, so a secret in a config dump is masked too.
func (s Secret) MarshalYAML() (any, error) {
	if s.IsZero() {
		return "", nil
	}

	return Marker, nil
}

// UnmarshalJSON lets a Secret be read from config.
//
// It accepts a value, so a config file can supply a secret, and it accepts the mask so a
// round-tripped document does not overwrite a stored secret with the literal word
// "REDACTED". A caller that needs to distinguish the two checks IsMarker afterwards.
func (s *Secret) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*s = Secret(raw)

	return nil
}

// LogValue implements slog.LogValuer, so a Secret passed as a slog attribute is masked
// before any handler sees it. This is what makes the type safe with structured logging
// even when the handler has no redaction of its own.
func (s Secret) LogValue() slog.Value {
	if s.IsZero() {
		return slog.StringValue("")
	}

	return slog.StringValue(Marker)
}

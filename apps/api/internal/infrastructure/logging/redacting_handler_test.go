package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// TestRedactingHandlerMasksSensitiveKeys is the guard for the defect this replaced:
// the handler built a redacted copy of the record and then logged the ORIGINAL, so every
// raw attribute was written and the redaction did nothing.
func TestRedactingHandlerMasksSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer

	inner := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(&redactingHandler{
		Handler: inner,
		enabled: true,
	})

	logger.InfoContext(context.Background(), "connect",
		"password", "hunter2",
		"api_token", "tok_live_abcdef123456",
		"dsn", "postgres://u:pw@h/db",
		"safe", "keep-me",
	)

	out := buf.String()

	for _, leaked := range []string{"hunter2", "tok_live_abcdef123456", "u:pw@"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("the log leaked %q: %s", leaked, out)
		}
	}
	if !strings.Contains(out, "keep-me") {
		t.Fatalf("a safe field was redacted: %s", out)
	}
	if !strings.Contains(out, redaction.Marker) {
		t.Fatalf("no mask was written: %s", out)
	}

	// The attribute must appear exactly once. The old code appended a redacted copy
	// alongside the original, so a field appeared twice, once with the raw value.
	if n := strings.Count(out, `"password"`); n != 1 {
		t.Fatalf("the password attribute appears %d times, want 1: %s", n, out)
	}
}

// TestRedactingHandlerMasksSecretType proves the strongest layer still holds when the
// handler has no key list at all: the value masks itself.
func TestRedactingHandlerMasksSecretType(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(&redactingHandler{
		Handler: slog.NewJSONHandler(&buf, nil),
		enabled: true,
	})

	const plaintext = "typed-secret-value-xyz"
	logger.InfoContext(context.Background(), "connect", "anything", redaction.NewSecret(plaintext))

	if strings.Contains(buf.String(), plaintext) {
		t.Fatalf("a Secret leaked through the handler: %s", buf.String())
	}
}

// TestRedactingHandlerRedactsTheMessage proves the message itself is scrubbed, not only
// the attributes: an error string is the commonest place a credential appears.
func TestRedactingHandlerRedactsTheMessage(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(&redactingHandler{
		Handler: slog.NewJSONHandler(&buf, nil),
		enabled: true,
	})

	logger.InfoContext(context.Background(), "connect failed: libsql://db.turso.io?authToken=secrettok123")

	if strings.Contains(buf.String(), "secrettok123") {
		t.Fatalf("the message leaked a credential: %s", buf.String())
	}
}

// TestRedactingHandlerDisabledIsPassThrough proves the toggle still works.
func TestRedactingHandlerDisabledIsPassThrough(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(&redactingHandler{
		Handler: slog.NewJSONHandler(&buf, nil),
		enabled: false,
	})

	logger.InfoContext(context.Background(), "msg", "safe", "keep-me")

	if !strings.Contains(buf.String(), "keep-me") {
		t.Fatalf("a disabled handler altered the output: %s", buf.String())
	}
}

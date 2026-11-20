package redaction

import (
	"strings"
	"testing"
)

// TestRedactLibsqlToken is the guard for this project's own database credential.
//
// Turso/libsql carries its token in the query string (`?authToken=...`), not in the
// userinfo the existing pattern matched, so a driver error quoting the DSN put the live
// token into any log line or notification that included it. The userinfo pattern alone
// does not cover this shape.
func TestRedactLibsqlToken(t *testing.T) {
	cases := []string{
		"libsql://vyzorix.turso.io?authToken=supersecrettokenvalue",
		"failed to connect: libsql://db.turso.io?authToken=abc123&foo=bar",
		"turso token=abc123",
		"auth_token=abc123",
		"password=hunter2",
		"api_key=sk-live-123",
	}

	for _, in := range cases {
		out := DefaultRedactor.Redact(in)
		for _, secret := range []string{"supersecrettokenvalue", "abc123", "hunter2", "sk-live-123"} {
			if strings.Contains(out, secret) {
				t.Errorf("%q still exposes %q: %s", in, secret, out)
			}
		}
	}
}

// TestRedactKeepsBenignProse is the inverse guard: the credential pattern must match the
// `key=value` form, not the word "token" in a sentence, or ordinary error text becomes
// unreadable and the redactor stops being trusted.
func TestRedactKeepsBenignProse(t *testing.T) {
	for _, in := range []string{
		"token budget exceeded",
		"invalid token provided",
		"the tokenizer failed",
		"timeout waiting for host",
	} {
		if out := DefaultRedactor.Redact(in); out != in {
			t.Errorf("benign text was altered: %q -> %q", in, out)
		}
	}
}

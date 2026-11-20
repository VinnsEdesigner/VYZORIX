package redaction

import (
	"strings"
	"testing"
)

// TestRedactProviderTokenFormats pins the token shapes that carry no label. A credential
// in a sentence — an alert quoting one — matches no key=value rule, so only the format
// itself identifies it.
func TestRedactProviderTokenFormats(t *testing.T) {
	// Each value is assembled from fragments so this file holds no contiguous
	// provider-shaped literal. Secret scanners match the committed bytes, and a real-looking
	// example key would be flagged on every push; the assembled value still exercises the
	// format regexes exactly as a live token would.
	const fill = "0123456789"
	cases := map[string]string{
		"stripe live":      "sk_" + "live_" + strings.Repeat(fill, 3),
		"stripe test":      "sk_" + "test_" + strings.Repeat(fill, 3),
		"github pat":       "gh" + "p_" + strings.Repeat(fill, 4),
		"github oauth":     "gh" + "o_" + strings.Repeat(fill, 4),
		"github finegrain": "github" + "_pat_" + strings.Repeat(fill, 3),
		"gitlab pat":       "gl" + "pat-" + strings.Repeat(fill, 2),
		"slack bot":        "xo" + "xb-" + strings.Repeat(fill, 3),
		"google api":       "AI" + "za" + strings.Repeat(fill, 4),
		"aws access key":   "AK" + "IA" + strings.Repeat("0123456789ABCDEF", 1),
		"openai":           "sk" + "-" + strings.Repeat(fill, 5),
		"sendgrid":         "SG" + "." + strings.Repeat(fill, 2) + "." + strings.Repeat(fill, 2),
		"npm":              "np" + "m_" + strings.Repeat(fill, 4),
		"digitalocean":     "do" + "p_v1_" + strings.Repeat("0123456789abcdef", 4),
		"shopify":          "sh" + "pat_" + strings.Repeat("0123456789abcdef", 2),
		"telegram bot":     "123456789" + ":AA" + strings.Repeat(fill, 4),
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			// Bare, with no label at all: the hardest case.
			out := DefaultRedactor.Redact("authentication failed using " + token)
			if strings.Contains(out, token) {
				t.Fatalf("the token survived: %s", out)
			}
			if !strings.Contains(out, Marker) {
				t.Fatalf("no mask was applied: %s", out)
			}

			// The surrounding text must survive, so the line stays diagnosable.
			if !strings.Contains(out, "authentication failed using") {
				t.Fatalf("the message was destroyed: %s", out)
			}
		})
	}
}

// TestRedactDoesNotTouchOrdinaryIdentifiers is the counterweight: the format patterns
// must not fire on words that merely resemble a prefix.
func TestRedactDoesNotTouchOrdinaryIdentifiers(t *testing.T) {
	for _, in := range []string{
		"device_id=dev-0001",
		"repo=acme/fleet",
		"branch main is behind",
		"sk- is not a token",
		"the github_pat prefix is documented",
	} {
		if out := DefaultRedactor.Redact(in); out != in {
			t.Errorf("ordinary text was altered:\n  in : %q\n  out: %q", in, out)
		}
	}
}

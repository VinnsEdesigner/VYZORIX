package provisioning

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

const testWebhookSecret = "whsec_test_secret_value"

// githubSignature computes the header GitHub would send for a body.
// timestampedSignature signs the same material the verifier checks when a provider sends
// an X-Timestamp: "timestamp\nbody".
func timestampedSignature(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + body))

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func githubSignature(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestVerifyAcceptsAValidGitHubSignature is the happy path.
func TestVerifyAcceptsAValidGitHubSignature(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	h := http.Header{}
	h.Set("X-Hub-Signature-256", githubSignature(testWebhookSecret, string(body)))

	v := NewSignatureVerifier()

	if err := v.Verify(h, body, redaction.NewSecret(testWebhookSecret)); err != nil {
		t.Fatalf("a valid signature was rejected: %v", err)
	}
}

// TestVerifyRejectsATamperedBody is the property that matters: a signature is bound to
// the bytes, so a valid one cannot be moved onto a different payload.
func TestVerifyRejectsATamperedBody(t *testing.T) {
	original := []byte(`{"ref":"refs/heads/main"}`)
	tampered := []byte(`{"ref":"refs/heads/evil"}`)

	h := http.Header{}
	h.Set("X-Hub-Signature-256", githubSignature(testWebhookSecret, string(original)))

	v := NewSignatureVerifier()

	if err := v.Verify(h, tampered, redaction.NewSecret(testWebhookSecret)); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("a tampered body was accepted: %v", err)
	}
}

// TestVerifyRejectsAWrongSecret proves another repository's secret cannot sign for this
// one.
func TestVerifyRejectsAWrongSecret(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	h := http.Header{}
	h.Set("X-Hub-Signature-256", githubSignature("a-different-secret", string(body)))

	v := NewSignatureVerifier()

	if err := v.Verify(h, body, redaction.NewSecret(testWebhookSecret)); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("a signature from another secret was accepted: %v", err)
	}
}

// TestVerifyRejectsAMissingHeader proves an unsigned request is refused.
func TestVerifyRejectsAMissingHeader(t *testing.T) {
	v := NewSignatureVerifier()

	if err := v.Verify(http.Header{}, []byte("body"), redaction.NewSecret(testWebhookSecret)); !errors.Is(err, ErrSignatureMissing) {
		t.Fatalf("an unsigned request was accepted: %v", err)
	}
}

// TestVerifyRejectsNoConfiguredSecret proves a repository with no secret cannot be
// triggered by anyone: accepting would make every webhook URL a public sync button.
func TestVerifyRejectsNoConfiguredSecret(t *testing.T) {
	body := []byte("body")
	h := http.Header{}
	h.Set("X-Hub-Signature-256", githubSignature("", string(body)))

	v := NewSignatureVerifier()

	if err := v.Verify(h, body, ""); err == nil {
		t.Fatal("a repository with no secret accepted a webhook")
	}
}

// TestVerifyRejectsSHA1 proves the weaker algorithm is refused: SHA-1 is not a defensible
// choice for a new verification path.
func TestVerifyRejectsSHA1(t *testing.T) {
	body := []byte("body")
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha1=abcdef0123456789")

	v := NewSignatureVerifier()

	if err := v.Verify(h, body, redaction.NewSecret(testWebhookSecret)); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("sha1 was accepted: %v", err)
	}
}

// TestVerifyAcceptsGitLabTokenHeader covers GitLab's shared-secret form.
func TestVerifyAcceptsGitLabTokenHeader(t *testing.T) {
	h := http.Header{}
	h.Set("X-Gitlab-Token", testWebhookSecret)

	v := NewSignatureVerifier()

	if err := v.Verify(h, []byte("anything"), redaction.NewSecret(testWebhookSecret)); err != nil {
		t.Fatalf("a valid GitLab token was rejected: %v", err)
	}
}

// TestVerifyRejectsStaleTimestamp proves a captured request cannot be replayed after the
// window closes.
func TestVerifyRejectsStaleTimestamp(t *testing.T) {
	body := []byte("body")
	h := http.Header{}
	h.Set("X-Hub-Signature-256", timestampedSignature(testWebhookSecret, "1000000000", string(body)))
	h.Set("X-Timestamp", "1000000000") // 2001

	v := NewSignatureVerifier()

	if err := v.Verify(h, body, redaction.NewSecret(testWebhookSecret)); !errors.Is(err, ErrSignatureStale) {
		t.Fatalf("a stale timestamp was accepted: %v", err)
	}
}

// TestVerifyAcceptsFreshTimestamp is the counterweight: a current timestamp must pass.
func TestVerifyAcceptsFreshTimestamp(t *testing.T) {
	body := []byte("body")
	h := http.Header{}
	h.Set("X-Hub-Signature-256", timestampedSignature(testWebhookSecret, "1700000000", string(body)))
	h.Set("X-Timestamp", "1700000000")

	v := &SignatureVerifier{now: func() time.Time { return time.Unix(1700000000, 0).Add(time.Minute) }}

	if err := v.Verify(h, body, redaction.NewSecret(testWebhookSecret)); err != nil {
		t.Fatalf("a fresh timestamp was rejected: %v", err)
	}
}

// TestVerifyChecksSignatureBeforeTimestamp proves the order: an attacker without the
// secret must not be able to probe the replay window.
func TestVerifyChecksSignatureBeforeTimestamp(t *testing.T) {
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256=deadbeef")
	h.Set("X-Timestamp", "1000000000")

	v := NewSignatureVerifier()

	err := v.Verify(h, []byte("body"), redaction.NewSecret(testWebhookSecret))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("got %v, want the signature error to win over the stale timestamp", err)
	}
}

// TestBranchFromRef pins the ref parsing.
func TestBranchFromRef(t *testing.T) {
	cases := map[string]string{
		"refs/heads/main":        "main",
		"refs/heads/release/1.2": "release/1.2",
		"refs/tags/v1.0.0":       "",
		"":                       "",
		"main":                   "",
	}

	for ref, want := range cases {
		if got := branchFromRef(ref); got != want {
			t.Errorf("branchFromRef(%q) = %q, want %q", ref, got, want)
		}
	}
}

// TestParseWebhookPayloadReadsTheBranch covers both providers' push shapes.
func TestParseWebhookPayloadReadsTheBranch(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main","commits":[]}`)

	for _, provider := range []Provider{ProviderGitHub, ProviderGitLab, ProviderGeneric} {
		if got := ParseWebhookPayload(provider, body); got.Branch != "main" {
			t.Errorf("%s: branch = %q, want main", provider, got.Branch)
		}
	}

	// A body that does not parse yields no branch, which the receiver treats as
	// "sync regardless" rather than an error.
	if got := ParseWebhookPayload(ProviderGitHub, []byte("not json")); got.Branch != "" {
		t.Errorf("a malformed body produced branch %q, want empty", got.Branch)
	}
}

// TestProviderFromHeader pins the attribution hint.
func TestProviderFromHeader(t *testing.T) {
	cases := []struct {
		name   string
		header map[string]string
		want   Provider
	}{
		{"gitlab", map[string]string{"X-Gitlab-Token": "x"}, ProviderGitLab},
		{"github", map[string]string{"X-Hub-Signature-256": "x"}, ProviderGitHub},
		{"none", nil, ProviderGeneric},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tc.header {
				h.Set(k, v)
			}
			if got := ProviderFromHeader(h); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestVerifyRejectsRewrittenTimestamp is the guard for the replay window being forgeable.
//
// The timestamp used to be checked but not signed, so a captured request could be replayed
// indefinitely by sending a fresh X-Timestamp alongside the original body and signature.
// The timestamp is now part of the MAC: rewriting it without the secret breaks the
// signature.
func TestVerifyRejectsRewrittenTimestamp(t *testing.T) {
	body := []byte("body")

	// A genuine delivery, signed for a stale instant.
	genuine := http.Header{}
	genuine.Set("X-Hub-Signature-256", timestampedSignature(testWebhookSecret, "1000000000", string(body)))
	genuine.Set("X-Timestamp", "1000000000")

	// The attacker keeps the body and signature but claims it is fresh.
	replayed := http.Header{}
	replayed.Set("X-Hub-Signature-256", genuine.Get("X-Hub-Signature-256"))
	replayed.Set("X-Timestamp", "1700000000")

	v := &SignatureVerifier{now: func() time.Time { return time.Unix(1700000000, 0) }}

	err := v.Verify(replayed, body, redaction.NewSecret(testWebhookSecret))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("a rewritten timestamp was accepted: %v", err)
	}
}

// TestVerifyBodyOnlySignatureStillWorksForGitHub proves the fix did not break real GitHub
// deliveries, which send no X-Timestamp and sign the body alone.
func TestVerifyBodyOnlySignatureStillWorksForGitHub(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)

	h := http.Header{}
	h.Set("X-Hub-Signature-256", githubSignature(testWebhookSecret, string(body)))

	if err := NewSignatureVerifier().Verify(h, body, redaction.NewSecret(testWebhookSecret)); err != nil {
		t.Fatalf("a genuine GitHub body-only signature was rejected: %v", err)
	}
}

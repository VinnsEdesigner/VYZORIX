package provisioning

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

var (
	// ErrSignatureMissing is returned when a webhook request carries no signature
	// header. It is a 401, not a 400: the caller is unauthenticated, not malformed.
	ErrSignatureMissing = errors.New("webhook signature header is missing")
	// ErrSignatureInvalid is returned when the signature does not match the body.
	ErrSignatureInvalid = errors.New("webhook signature does not match")
	// ErrSignatureStale is returned when the signed timestamp is outside the replay
	// window. A captured request cannot be replayed indefinitely.
	ErrSignatureStale = errors.New("webhook timestamp is outside the accepted window")
)

// ReplayWindow is how far a signed timestamp may be from now.
//
// GitHub uses 300s for its own deliveries. Matching it means a self-hoster's provider
// and this server agree on what "fresh" means, and a replay is useless after five
// minutes regardless of whether the request was observed.
const ReplayWindow = 5 * time.Minute

// SignatureHeaders are the header names providers actually send.
//
// GitHub sends X-Hub-Signature-256 (sha256=...), GitLab sends X-Gitlab-Token (a shared
// secret, not an HMAC), and a generic sender sends X-Hub-Signature-256 too. The
// receiver accepts any of them rather than forcing a provider to be rewritten.
var SignatureHeaders = []string{
	"X-Hub-Signature-256",
	"X-Hub-Signature",
	"X-Gitlab-Token",
	"X-Signature-256",
}

// SignatureVerifier checks that a webhook request came from someone holding the
// repository's secret.
//
// Two properties matter. It compares with hmac.Equal, so a timing side channel cannot
// reveal the expected value byte by byte. And it binds the signature to the raw body,
// so a valid signature cannot be moved onto a different payload.
type SignatureVerifier struct {
	now func() time.Time
}

// NewSignatureVerifier builds a verifier. The clock is injectable so a test can pin it.
func NewSignatureVerifier() *SignatureVerifier {
	return &SignatureVerifier{now: time.Now}
}

// Verify checks a request against a repository's secret and returns nil when genuine.
//
// rawBody must be the bytes exactly as received. A provider signs what it sent, so
// re-encoding a parsed body would change the bytes and break every signature.
func (v *SignatureVerifier) Verify(h http.Header, rawBody []byte, secret redaction.Secret) error {
	if secret.IsZero() {
		// No secret configured means no webhook is expected. Refusing is the safe
		// answer: accepting an unverifiable push would let anyone trigger a sync.
		return fmt.Errorf("%w: no secret is configured for this repository", ErrSignatureInvalid)
	}

	header, value := findSignatureHeader(h)
	if header == "" {
		return ErrSignatureMissing
	}

	// GitLab sends the secret itself in X-Gitlab-Token, not an HMAC. Comparing it
	// directly is the documented contract for that header.
	if strings.EqualFold(header, "X-Gitlab-Token") {
		if !hmac.Equal([]byte(value), []byte(secret.Reveal())) {
			return ErrSignatureInvalid
		}

		return nil
	}

	// GitHub's sha256= prefix names the algorithm; an older sha1= is refused because
	// SHA-1 is not a defensible choice for a new verification path.
	algo, sig, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("%w: the header is not in algo=signature form", ErrSignatureInvalid)
	}
	if !strings.EqualFold(algo, "sha256") {
		return fmt.Errorf("%w: unsupported algorithm %q", ErrSignatureInvalid, algo)
	}

	// The timestamp, when the provider sends one, is part of the signed material rather
	// than a header checked beside it. A header outside the MAC could be rewritten on a
	// captured request, which would make the freshness window meaningless: an attacker
	// could replay any old delivery forever by sending a fresh X-Timestamp. GitHub sends no
	// timestamp and signs the body alone, so an absent header keeps the body-only MAC —
	// signing a header GitHub never sends would reject every genuine delivery.
	expected := signBody(secret, signedPayload(h.Get("X-Timestamp"), rawBody))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return ErrSignatureInvalid
	}

	// The timestamp is checked only after the signature is known good. Checking it
	// first would let an attacker probe the replay window without the secret.
	return v.checkFreshness(h)
}

// signedPayload is the exact bytes the HMAC covers.
func signedPayload(timestamp string, body []byte) []byte {
	if timestamp == "" {
		return body
	}

	out := make([]byte, 0, len(timestamp)+1+len(body))
	out = append(out, timestamp...)
	out = append(out, '\n')
	out = append(out, body...)

	return out
}

// checkFreshness rejects a request whose signed timestamp is outside the window.
//
// The timestamp is read from a header the provider signs as part of the body on some
// integrations, and from the body on others. Here it is an optional header: when it is
// absent, freshness cannot be judged and the signature alone is the guarantee — which
// is the same trade GitHub makes, since its signature covers no timestamp either.
func (v *SignatureVerifier) checkFreshness(h http.Header) error {
	raw := h.Get("X-Timestamp")
	if raw == "" {
		return nil
	}

	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: X-Timestamp is not a unix second", ErrSignatureStale)
	}

	sent := time.Unix(secs, 0)
	if delta := v.now().Sub(sent); delta > ReplayWindow || delta < -ReplayWindow {
		return fmt.Errorf("%w: %s off", ErrSignatureStale, delta.Round(time.Second))
	}

	return nil
}

// signBody computes the hex HMAC-SHA256 a provider would send for this body.
func signBody(secret redaction.Secret, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret.Reveal()))
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

// findSignatureHeader returns the first recognised signature header and its value.
func findSignatureHeader(h http.Header) (name, value string) {
	for _, candidate := range SignatureHeaders {
		if v := h.Get(candidate); v != "" {
			return candidate, v
		}
	}

	return "", ""
}

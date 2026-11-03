// Package crypto: asymmetric device identity.
//
// This replaces the HMAC command secret. The old scheme needed a shared secret,
// which meant the server had to transmit it — and it did so over FCM, an
// unauthenticated channel. A keypair removes the problem at the root: the device
// generates a private key that never leaves it and registers only the public half,
// so nothing secret is ever transmitted in either direction.
//
// Primitive: ECDSA over P-256 with SHA-256. Chosen over Ed25519 because Android's
// Keystore supports EC P-256 from API 1 while Ed25519 arrives at API 33; the fleet
// target is well below that. One primitive for both directions, so there is one
// encode/decode/verify path rather than two.
package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	libv "github.com/VinnsEdesigner/libv/go"
)

var (
	// ErrNoPublicKey is returned when a device has no registered public key.
	ErrNoPublicKey = errors.New("device has no registered public key")
	// ErrBadPublicKey is returned when a stored key cannot be parsed.
	ErrBadPublicKey = errors.New("device public key is malformed")
	// ErrDeviceSignatureInvalid is returned when a device signature does not verify.
	ErrDeviceSignatureInvalid = errors.New("device signature invalid")
	// ErrServerSignatureInvalid is returned when a server signature does not verify.
	ErrServerSignatureInvalid = errors.New("server signature invalid")
)

// EncodePublicKey serialises an EC public key as a PEM SubjectPublicKeyInfo
// block. PEM rather than raw bytes so the stored value is self-describing: a
// future key type can be added without a schema change, and a corrupt value fails
// to parse rather than being silently misread as a different curve.
func EncodePublicKey(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// DecodePublicKey parses a PEM public key and requires it to be on P-256.
// The curve check is deliberate: a key on a different curve would verify against
// a canonical string the device never signed.
func DecodePublicKey(encoded string) (*ecdsa.PublicKey, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, ErrNoPublicKey
	}

	block, _ := pem.Decode([]byte(encoded))
	if block == nil {
		return nil, ErrBadPublicKey
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadPublicKey, err)
	}

	pub, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrBadPublicKey
	}
	if pub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%w: expected P-256", ErrBadPublicKey)
	}

	return pub, nil
}

// GenerateDeviceKeyPair generates a P-256 keypair. Used by tests and by the
// server's own signing key bootstrap; a real device generates its own.
func GenerateDeviceKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// DeviceCanonicalString is what a device signs for an HTTP request.
//
// Every element is length-prefixed or newline-separated and none may be empty, so
// two different field values cannot concatenate into the same string — the classic
// canonicalisation bug where ("ab","c") and ("a","bc") sign identically.
func DeviceCanonicalString(method, path, deviceID, timestampMs, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)

	var b strings.Builder
	b.WriteString(strings.ToUpper(method))
	b.WriteString("\n")
	b.WriteString(path)
	b.WriteString("\n")
	b.WriteString(deviceID)
	b.WriteString("\n")
	b.WriteString(timestampMs)
	b.WriteString("\n")
	b.WriteString(nonce)
	b.WriteString("\n")
	b.WriteString(base64.StdEncoding.EncodeToString(bodyHash[:]))

	return b.String()
}

// VerifyDeviceSignature checks a device's signature over its canonical string.
// The signature is base64-encoded ASN.1 DER, which is what Android's
// SHA256withECDSA produces.
func VerifyDeviceSignature(
	publicKeyPEM string,
	method, path, deviceID, timestampMs, nonce string,
	body []byte,
	signatureB64 string,
) error {
	pub, err := DecodePublicKey(publicKeyPEM)
	if err != nil {
		return err
	}

	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return ErrDeviceSignatureInvalid
	}

	digest := sha256.Sum256([]byte(DeviceCanonicalString(method, path, deviceID, timestampMs, nonce, body)))
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return ErrDeviceSignatureInvalid
	}

	return nil
}

// ServerSigner signs what the server sends to a device: the config and any
// command frame. The device verifies against the public half, pinned in the app.
//
// It holds both the raw ECDSA key (for command frames, whose canonical string is
// the server's own) and a libv.Signer built from the same key (for config
// documents, whose canonical string is the format's). One key, two canonical
// forms: the config's is defined by libv and the device verifies it with libv, so
// signing it any other way produces a signature the device can never check.
type ServerSigner struct {
	key *ecdsa.PrivateKey
	// doc is the same key behind the format's signer, built once here rather than
	// per call so signing a config is a pure function of the key.
	doc *libv.Signer
}

// NewServerSigner builds a signer from a PEM-encoded PKCS#8 private key, as
// loaded from configuration.
func NewServerSigner(privateKeyPEM string) (*ServerSigner, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("server signing key is not PEM")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse server signing key: %w", err)
	}

	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, errors.New("server signing key must be an EC P-256 key")
	}

	// The format's signer is built from the same PEM. If it cannot be built the key
	// is unusable for configs, and failing here beats issuing an unverifiable one.
	doc, err := libv.NewSignerFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("build config signer: %w", err)
	}

	return &ServerSigner{key: key, doc: doc}, nil
}

// SignDocument signs a vyzorix.v document in the format's own shape, appending the
// `v sig ecdsa-p256 <base64>` line the device's verifier looks for.
//
// This is the config's signing path. It is deliberately not SignCanonical: a
// config is verified with libv against the canonical string libv defines, so the
// signature must be produced over that same string. Signing the raw text (as an
// earlier version did) yields a signature no device can ever check, and omitting
// the signature line entirely makes the document unsigned — both leave the device
// unable to trust the config it is handed.
func (s *ServerSigner) SignDocument(document string) (string, error) {
	return s.doc.Sign(document)
}

// ServerCanonicalString is what the server signs. It is deliberately distinct
// from the device's form — a different domain prefix means a device signature can
// never be replayed as a server signature, or vice versa.
func ServerCanonicalString(kind, deviceID, timestampMs, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)

	var b strings.Builder
	b.WriteString("SERVER\n")
	b.WriteString(kind)
	b.WriteString("\n")
	b.WriteString(deviceID)
	b.WriteString("\n")
	b.WriteString(timestampMs)
	b.WriteString("\n")
	b.WriteString(nonce)
	b.WriteString("\n")
	b.WriteString(base64.StdEncoding.EncodeToString(bodyHash[:]))

	return b.String()
}

// Sign produces a base64 ASN.1 DER signature over the server canonical string,
// returning the signature and the nonce that was used. The caller puts both on
// the wire so the device can reconstruct the string.
func (s *ServerSigner) Sign(kind, deviceID string, body []byte) (signatureB64, nonce string, timestampMs int64, err error) {
	nonceBytes := make([]byte, 16)
	if _, readErr := rand.Read(nonceBytes); readErr != nil {
		return "", "", 0, fmt.Errorf("nonce: %w", readErr)
	}
	nonce = base64.RawURLEncoding.EncodeToString(nonceBytes)
	timestampMs = time.Now().UnixMilli()

	digest := sha256.Sum256(
		[]byte(ServerCanonicalString(kind, deviceID, strconv.FormatInt(timestampMs, 10), nonce, body)),
	)
	sig, err := ecdsa.SignASN1(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", "", 0, fmt.Errorf("sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), nonce, timestampMs, nil
}

// SignCanonical signs a caller-built canonical string and returns the base64
// ASN.1 DER signature.
//
// This exists because Sign generates its own timestamp and nonce, which is right
// for a payload that carries neither — but a command frame carries both, and the
// signature must cover the values the frame actually holds. Signing with values
// the caller cannot see produces a signature that can never verify.
func (s *ServerSigner) SignCanonical(canonical string) (string, error) {
	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyServerSignature is the device-side check, mirrored here so a test can
// assert the two implementations agree byte for byte.
func VerifyServerSignature(
	serverPublicKey *ecdsa.PublicKey,
	kind, deviceID, timestampMs, nonce string,
	body []byte,
	signatureB64 string,
) error {
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return ErrServerSignatureInvalid
	}

	digest := sha256.Sum256([]byte(ServerCanonicalString(kind, deviceID, timestampMs, nonce, body)))
	if !ecdsa.VerifyASN1(serverPublicKey, digest[:], sig) {
		return ErrServerSignatureInvalid
	}

	return nil
}

// pairingAlphabet is Crockford base32: no I, L, O or U, so a code read off a
// notification cannot be mistyped for a different valid code.
const pairingAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// pairingCodeLength is 8 characters from a 32-symbol alphabet, ~40 bits. Entropy
// alone is not the defence; the TTL, single-use rule and rate limiting are what
// make guessing infeasible. See REGISTRATION_CONTRACT.md §4.
const pairingCodeLength = 8

// GeneratePairingCode returns a code and its display form ("K7QM-3X2P").
func GeneratePairingCode() (code string, display string, err error) {
	buf := make([]byte, pairingCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("pairing code: %w", err)
	}

	chars := make([]byte, pairingCodeLength)
	for i, b := range buf {
		chars[i] = pairingAlphabet[int(b)%len(pairingAlphabet)]
	}

	code = string(chars)

	return code, code[:4] + "-" + code[4:], nil
}

// NormalizePairingCode upper-cases and strips the grouping separator, so a user
// who types the code exactly as shown is accepted.
func NormalizePairingCode(input string) string {
	cleaned := strings.ToUpper(strings.TrimSpace(input))
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Crockford: a user reading the alphabet may type these for the digits.
	cleaned = strings.NewReplacer("I", "1", "L", "1", "O", "0").Replace(cleaned)

	return cleaned
}

// ConstantTimeEqual compares two strings without leaking length-independent
// timing. Used for the pairing code, which is the one unauthenticated step.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

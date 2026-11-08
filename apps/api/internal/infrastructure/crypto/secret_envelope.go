// Package crypto: the secret envelope.
//
// A secret is never carried in a signed config (VYZORIX_DEPLOY_CONTRACT.md §5.2,
// §9b). It travels on its own channel, RSA-OAEP-SHA256 to the device's public
// key, so a compromise that reaches the ciphertext store yields nothing for a
// device whose private key the attacker does not hold. This is the server half.
//
// RSA-OAEP and not ECDH: Android Keystore PURPOSE_AGREE_KEY is API 31 and the app
// supports API 26; PURPOSE_DECRYPT is API 23. The recipient key is a second
// Keystore entry, separate from the identity key, whose public half is public.
//
// Interop trap this file pins down, and the reason it uses SHA-1: Go's
// rsa.EncryptOAEP uses the supplied hash for both the OAEP digest and MGF1, so
// it cannot produce the "SHA-256 digest, SHA-1 MGF1" pair that Java's
// OAEPWithSHA-256AndMGF1Padding defaults to. More to the point, the Android
// Keystore refuses anything but MGF1-SHA1: on API 28 a decrypt with an explicit
// MGF1-SHA256 spec throws "Unsupported MGF1 digest: SHA-256. Only SHA-1
// supported", while OAEP-SHA1/MGF1-SHA1 succeeds. Since the device's key is
// non-exportable and lives in the Keystore, the Keystore's constraint decides
// the envelope: SHA-1 for both, which is what EncryptOAEP(sha1.New()) produces.
// SHA-1 in OAEP is the standard default and is not a practical weakness here —
// the 3072-bit modulus is the protection — but it is a constraint, not a
// preference, and it is recorded so nobody "upgrades" it back to SHA-256 and
// breaks every device. Verified on device, not inferred; see the spike note in
// VYZORIX_DEPLOY_CONTRACT.md §9b.3.
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	// SHA-1 is required by the Android Keystore's OAEP implementation, which
	// supports only MGF1-SHA1. See the package comment.
	"crypto/sha1" //nolint:gosec // the Keystore forces MGF1-SHA1; see above
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// SecretEnvelopeAlgorithm names the envelope in logs and in the API, so a
// mismatch between server and device is a string comparison rather than a
// mysterious decryption failure.
const SecretEnvelopeAlgorithm = "RSA-OAEP-SHA1"

// SecretKeyBits is the modulus size. 3072 rather than 2048: a secret that is
// meant to outlive the deploy that set it should not be on a key that a
// well-resourced adversary can factor on the timescale the device is in service.
const SecretKeyBits = 3072

var (
	// ErrNoRecipientKey is returned when a device has no secret key registered.
	ErrNoRecipientKey = errors.New("device has no secret key registered")
	// ErrBadRecipientKey is returned when a stored public key cannot be parsed.
	ErrBadRecipientKey = errors.New("device secret key is malformed")
	// ErrSecretDecrypt is returned when a ciphertext does not open.
	ErrSecretDecrypt = errors.New("secret ciphertext does not decrypt")
)

// GenerateSecretKeyPair generates the device's RSA keypair. A real device
// generates this in the Keystore and never exports it; this exists for tests and
// for the spike that proves the Go/Kotlin interop.
func GenerateSecretKeyPair() (*rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, SecretKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}

	return priv, nil
}

// EncodeRSAPublicKeyPEM serialises an RSA public key as PEM SubjectPublicKeyInfo,
// which is what the device sends at enroll and what the server stores.
func EncodeRSAPublicKeyPEM(pub *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshal secret public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// EncodeRSAPrivateKeyPEM serialises a private key as PKCS#8 PEM. Only tests and
// the spike call this: a device's private key does not leave the Keystore, and a
// server never holds one.
func EncodeRSAPrivateKeyPEM(priv *rsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", fmt.Errorf("marshal secret private key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

// DecodeRSAPublicKeyPEM parses a stored device public key.
func DecodeRSAPublicKeyPEM(encoded string) (*rsa.PublicKey, error) {
	if encoded == "" {
		return nil, ErrNoRecipientKey
	}

	block, _ := pem.Decode([]byte(encoded))
	if block == nil {
		return nil, ErrBadRecipientKey
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRecipientKey, err)
	}

	pub, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: not an RSA key", ErrBadRecipientKey)
	}

	return pub, nil
}

// DecodeRSAPrivateKeyPEM parses a PKCS#8 private key. Test-only, mirroring
// [EncodeRSAPrivateKeyPEM].
func DecodeRSAPrivateKeyPEM(encoded string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(encoded))
	if block == nil {
		return nil, ErrBadRecipientKey
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRecipientKey, err)
	}

	priv, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: not an RSA key", ErrBadRecipientKey)
	}

	return priv, nil
}

// WrapSecret encrypts a secret to a device's public key and returns the
// ciphertext as base64, which is the form the API serves and the device fetches.
//
// The parameters are the contract's: OAEP-SHA1 with a nil label. A label would
// have to match on both sides and buys nothing here, so it is nil rather than an
// empty string — the two are not the same to some providers.
func WrapSecret(recipientPublicKeyPEM string, plaintext []byte) (string, error) {
	pub, err := DecodeRSAPublicKeyPEM(recipientPublicKeyPEM)
	if err != nil {
		return "", err
	}

	if len(plaintext) == 0 {
		return "", errors.New("secret is empty")
	}

	// A 3072-bit modulus leaves ~318 bytes for OAEP-SHA1. Refusing over-long
	// plaintext here names the limit instead of letting EncryptOAEP return a
	// bare "message too long", which reads like a bug rather than a bound.
	if max := pub.Size() - 2*sha1.Size - 2; len(plaintext) > max {
		return "", fmt.Errorf("secret is %d bytes; the envelope holds at most %d", len(plaintext), max)
	}

	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, pub, plaintext, nil)
	if err != nil {
		return "", fmt.Errorf("wrap secret: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// UnwrapSecret opens a ciphertext with a private key. The server does not do
// this in production — only the device holds the private half — but it is the
// other side of [WrapSecret] and is what makes a Go-side round-trip test
// possible.
func UnwrapSecret(priv *rsa.PrivateKey, ciphertextB64 string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, ErrSecretDecrypt
	}

	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, priv, ciphertext, nil)
	if err != nil {
		// The underlying error distinguishes "wrong key" from "corrupt", but a
		// caller must not: reporting which one it was is an oracle. One error.
		return nil, ErrSecretDecrypt
	}

	return plaintext, nil
}

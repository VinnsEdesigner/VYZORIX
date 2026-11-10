package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// newSecretKey generates a keypair for a test. 2048 rather than 3072: the
// modulus size is asserted separately, and key generation is the slow part of
// this test file. The envelope's behaviour does not depend on the size.
func newSecretKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	return priv
}

// TestSecretEnvelopeRoundTrip is the property the envelope exists for: what the
// server wraps, the holder of the key opens, byte for byte.
func TestSecretEnvelopeRoundTrip(t *testing.T) {
	priv := newSecretKey(t)

	pubPEM, err := EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}

	secret := []byte("postgres://user:pw@db.internal:5432/app")

	ciphertext, err := WrapSecret(pubPEM, secret)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	got, err := UnwrapSecret(priv, ciphertext)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}

	if !bytes.Equal(got, secret) {
		t.Fatalf("round trip changed the secret: got %q, want %q", got, secret)
	}
}

// TestSecretEnvelopeWrongKeyFails is the second half of item 8: a ciphertext
// wrapped to key A must not open with key B. If it did, "encrypted to the
// device's key" would be a claim rather than a property.
func TestSecretEnvelopeWrongKeyFails(t *testing.T) {
	privA := newSecretKey(t)
	privB := newSecretKey(t)

	pubPEM, err := EncodeRSAPublicKeyPEM(&privA.PublicKey)
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}

	ciphertext, err := WrapSecret(pubPEM, []byte("s3cr3t"))
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	if _, err := UnwrapSecret(privB, ciphertext); !errors.Is(err, ErrSecretDecrypt) {
		t.Fatalf("unwrapping with the wrong key must fail with ErrSecretDecrypt, got %v", err)
	}
}

// TestSecretEnvelopeTamperFails proves the OAEP integrity check is doing its
// job. A flipped ciphertext byte must be refused, not returned as garbage —
// "a secret that decrypts to garbage is worse than one that refuses".
func TestSecretEnvelopeTamperFails(t *testing.T) {
	priv := newSecretKey(t)

	pubPEM, _ := EncodeRSAPublicKeyPEM(&priv.PublicKey)

	ciphertext, err := WrapSecret(pubPEM, []byte("s3cr3t"))
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	raw[len(raw)/2] ^= 0x01

	if _, err := UnwrapSecret(priv, base64.StdEncoding.EncodeToString(raw)); !errors.Is(err, ErrSecretDecrypt) {
		t.Fatalf("a tampered ciphertext must fail with ErrSecretDecrypt, got %v", err)
	}
}

// TestSecretEnvelopeRejectsBadRecipient covers the shapes a device key can
// arrive in before it is usable: absent, not PEM, and not RSA.
func TestSecretEnvelopeRejectsBadRecipient(t *testing.T) {
	// An EC key is a valid PEM block and the wrong key type.
	ecPriv, err := GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate ec key: %v", err)
	}

	ecPEM, err := EncodePublicKey(&ecPriv.PublicKey)
	if err != nil {
		t.Fatalf("encode ec key: %v", err)
	}

	cases := []struct {
		name string
		pem  string
		want error
	}{
		{"absent", "", ErrNoRecipientKey},
		{"not pem", "not a pem block", ErrBadRecipientKey},
		{"wrong key type", ecPEM, ErrBadRecipientKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := WrapSecret(tc.pem, []byte("x")); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

// TestSecretEnvelopeBoundsThePlaintext proves an over-long secret is refused
// with a message that names the limit, rather than RSA's bare "message too
// long". A bound an operator cannot read is a bound they will report as a bug.
func TestSecretEnvelopeBoundsThePlaintext(t *testing.T) {
	priv := newSecretKey(t)

	pubPEM, _ := EncodeRSAPublicKeyPEM(&priv.PublicKey)

	max := priv.PublicKey.Size() - 2*sha1.Size - 2

	if _, err := WrapSecret(pubPEM, make([]byte, max+1)); err == nil {
		t.Fatal("an over-long secret must be refused")
	} else if !strings.Contains(err.Error(), "at most") {
		t.Fatalf("the refusal must name the limit, got %q", err)
	}

	if _, err := WrapSecret(pubPEM, make([]byte, max)); err != nil {
		t.Fatalf("a secret at the limit must be accepted, got %v", err)
	}
}

// TestSecretEnvelopeRejectsEmpty guards the one input that would otherwise wrap
// successfully and decrypt to nothing, which is indistinguishable downstream
// from a secret that was never set.
func TestSecretEnvelopeRejectsEmpty(t *testing.T) {
	priv := newSecretKey(t)

	pubPEM, _ := EncodeRSAPublicKeyPEM(&priv.PublicKey)

	if _, err := WrapSecret(pubPEM, nil); err == nil {
		t.Fatal("an empty secret must be refused")
	}
}

// TestSecretKeyBits pins the size the contract states, so a later edit to the
// constant is a test failure rather than a silent weakening.
func TestSecretKeyBits(t *testing.T) {
	if SecretKeyBits != 3072 {
		t.Fatalf("the contract specifies RSA-3072, got %d", SecretKeyBits)
	}
}

// TestSecretEnvelopeAlgorithmName pins the string the device compares against,
// so a rename on one side is caught here rather than as an interop failure.
func TestSecretEnvelopeAlgorithmName(t *testing.T) {
	if SecretEnvelopeAlgorithm != "RSA-OAEP-SHA1" {
		t.Fatalf("unexpected algorithm name %q", SecretEnvelopeAlgorithm)
	}
}

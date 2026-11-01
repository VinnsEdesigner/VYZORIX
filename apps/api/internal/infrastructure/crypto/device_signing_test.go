package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strconv"
	"strings"
	"testing"
)

// TestPublicKeyRoundTrip proves a key survives encode/decode, and that a
// different curve is refused rather than silently accepted.
func TestPublicKeyRoundTrip(t *testing.T) {
	key, err := GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	encoded, err := EncodePublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := DecodePublicKey(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !decoded.Equal(&key.PublicKey) {
		t.Fatal("decoded key differs from the original")
	}
}

func TestDecodePublicKeyRefusesGarbage(t *testing.T) {
	for name, in := range map[string]string{
		"empty":     "",
		"not pem":   "hello",
		"bad bytes": "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n",
	} {
		if _, err := DecodePublicKey(in); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestDecodePublicKeyRefusesNonP256(t *testing.T) {
	// A P-384 key parses as a valid EC key but must be refused: a signature over a
	// canonical string the device never signed would otherwise verify.
	other, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate p384: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&other.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	encoded := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	if _, err := DecodePublicKey(encoded); err == nil {
		t.Fatal("a non-P-256 key must be refused")
	}
}

// signAsDevice mimics what the Android Keystore does: SHA256withECDSA, ASN.1 DER,
// base64. If the server verifies this, the two sides agree on the primitive.
func signAsDevice(t *testing.T, key *ecdsa.PrivateKey, canonical string) string {
	t.Helper()

	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	return base64.StdEncoding.EncodeToString(sig)
}

// TestDeviceSignatureVerifies is the core of the spike: a signature produced the
// way Android produces one must verify against the device's registered public key.
func TestDeviceSignatureVerifies(t *testing.T) {
	key, err := GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	pubPEM, err := EncodePublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	body := []byte(`{"deviceUuid":"d1"}`)
	canonical := DeviceCanonicalString("POST", "/v1/device/enroll", "d1", "1700000000000", "n1", body)
	sig := signAsDevice(t, key, canonical)

	if err := VerifyDeviceSignature(pubPEM, "POST", "/v1/device/enroll", "d1", "1700000000000", "n1", body, sig); err != nil {
		t.Fatalf("a valid device signature must verify: %v", err)
	}
}

func TestDeviceSignatureRefusesTampering(t *testing.T) {
	key, _ := GenerateDeviceKeyPair()
	pubPEM, _ := EncodePublicKey(&key.PublicKey)

	body := []byte(`{"deviceUuid":"d1"}`)
	canonical := DeviceCanonicalString("POST", "/v1/device/enroll", "d1", "1700000000000", "n1", body)
	sig := signAsDevice(t, key, canonical)

	cases := map[string]struct {
		method, path, deviceID, ts, nonce string
		body                              []byte
	}{
		"changed body":   {"POST", "/v1/device/enroll", "d1", "1700000000000", "n1", []byte(`{"deviceUuid":"d2"}`)},
		"changed method": {"GET", "/v1/device/enroll", "d1", "1700000000000", "n1", body},
		"changed path":   {"POST", "/v1/device/other", "d1", "1700000000000", "n1", body},
		"changed device": {"POST", "/v1/device/enroll", "dX", "1700000000000", "n1", body},
		"changed ts":     {"POST", "/v1/device/enroll", "d1", "1700000000001", "n1", body},
		"changed nonce":  {"POST", "/v1/device/enroll", "d1", "1700000000000", "n2", body},
	}

	for name, c := range cases {
		err := VerifyDeviceSignature(pubPEM, c.method, c.path, c.deviceID, c.ts, c.nonce, c.body, sig)
		if err == nil {
			t.Errorf("%s: tampering must fail verification", name)
		}
	}
}

// TestCanonicalStringIsUnambiguous proves two different field splits cannot
// produce the same canonical string — the classic concatenation bug.
func TestCanonicalStringIsUnambiguous(t *testing.T) {
	a := DeviceCanonicalString("POST", "/a", "bc", "1", "n", nil)
	b := DeviceCanonicalString("POST", "/ab", "c", "1", "n", nil)

	if a == b {
		t.Fatal("different fields must not collide in the canonical string")
	}
}

// TestServerSignatureVerifies proves the server's own sign/verify pair agrees.
func TestServerSignatureVerifies(t *testing.T) {
	key, err := GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	signer, err := NewServerSigner(privPEM)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	body := []byte(`{"version":1}`)
	sig, nonce, ts, err := signer.Sign("config", "d1", body)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	err = VerifyServerSignature(&key.PublicKey, "config", "d1", strconv.FormatInt(ts, 10), nonce, body, sig)
	if err != nil {
		t.Fatalf("a valid server signature must verify: %v", err)
	}

	// A config for a different device must not verify, or one device could apply
	// another's config.
	err = VerifyServerSignature(&key.PublicKey, "config", "d2", strconv.FormatInt(ts, 10), nonce, body, sig)
	if err == nil {
		t.Fatal("a signature bound to d1 must not verify for d2")
	}
}

// TestServerAndDeviceDomainsAreSeparate proves a device signature cannot be
// replayed as a server signature: the canonical strings differ by a domain prefix.
func TestServerAndDeviceDomainsAreSeparate(t *testing.T) {
	device := DeviceCanonicalString("POST", "/x", "d1", "1", "n", []byte("b"))
	server := ServerCanonicalString("POST", "d1", "1", "n", []byte("b"))

	if strings.Contains(server, device) || device == server {
		t.Fatal("the two canonical forms must not be interchangeable")
	}
	if !strings.HasPrefix(server, "SERVER\n") {
		t.Fatal("the server form must carry its domain prefix")
	}
}

// TestPairingCodeShape pins the alphabet, length and grouping.
func TestPairingCodeShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, display, err := GeneratePairingCode()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(code) != 8 {
			t.Fatalf("code %q has length %d, want 8", code, len(code))
		}
		if len(display) != 9 || display[4] != '-' {
			t.Fatalf("display %q is not grouped 4-4", display)
		}
		for _, r := range code {
			if !strings.ContainsRune(pairingAlphabet, r) {
				t.Fatalf("code %q contains %q, which is not in the alphabet", code, r)
			}
		}
		// Crockford excludes these precisely because they are misread.
		for _, bad := range []string{"I", "L", "O", "U"} {
			if strings.Contains(code, bad) {
				t.Fatalf("code %q must not contain %q", code, bad)
			}
		}
		seen[code] = true
	}

	// Not a randomness test, but 200 draws colliding would mean a broken source.
	if len(seen) < 190 {
		t.Fatalf("only %d distinct codes in 200 draws", len(seen))
	}
}

// TestNormalizePairingCode proves a user typing what they see is accepted, and
// that the ambiguous letters map to the digits they resemble.
func TestNormalizePairingCode(t *testing.T) {
	cases := map[string]string{
		"K7QM-3X2P":  "K7QM3X2P",
		"k7qm3x2p":   "K7QM3X2P",
		" k7qm 3x2p": "K7QM3X2P",
		"K7QMI":      "K7QM1",
		"K7QML":      "K7QM1",
		"K7QMO":      "K7QM0",
	}

	for in, want := range cases {
		if got := NormalizePairingCode(in); got != want {
			t.Errorf("NormalizePairingCode(%q) = %q, want %q", in, got, want)
		}
	}
}

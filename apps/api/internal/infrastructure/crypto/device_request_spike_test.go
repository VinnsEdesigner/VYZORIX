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
	"testing"
	"time"
)

// TestDeviceRequestRoundTrip is the server half of the handshake spike: a
// signature produced the way the device produces it must verify with the
// server's verifier, over the exact canonical string.
func TestDeviceRequestRoundTrip(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	method := "POST"
	path := "/v1/device/dev-1/command"
	deviceID := "dev-1"
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "n-1"
	body := []byte(`{"command":"SHELL_EXEC"}`)

	// Device side: sign the canonical string.
	bodyHash := sha256.Sum256(body)
	canonical := method + "\n" + path + "\n" + deviceID + "\n" + ts + "\n" + nonce + "\n" +
		base64.StdEncoding.EncodeToString(bodyHash[:])
	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Server side: verify.
	if err := VerifyDeviceSignature(pubPEM, method, path, deviceID, ts, nonce, body, sigB64); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// The server's own canonical builder must produce the same string.
	got := DeviceCanonicalString(method, path, deviceID, ts, nonce, body)
	if got != canonical {
		t.Fatalf("canonical mismatch:\n got %q\nwant %q", got, canonical)
	}
}

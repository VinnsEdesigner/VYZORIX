package websocket

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	config "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// connectFixture builds an upgrader with a nonce cache and a pinned device key,
// and returns a signer for that key.
func connectFixture(t *testing.T) (*StreamUpgrader, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	cfg := config.Config{Env: "production", EnforceHMAC: true}
	upgrader := NewStreamUpgrader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg,
		cryptohmac.Verifier{Nonces: cryptohmac.NewNonceCache(5 * time.Minute), Window: 5 * time.Minute},
		func(string) string { return pubPEM },
	)

	return upgrader, key
}

// signConnect produces the query the device sends, signing ConnectCanonical.
func signConnect(t *testing.T, key *ecdsa.PrivateKey, deviceID, ts, nonce string) string {
	t.Helper()

	canonical := "connect|" + deviceID + "|" + ts + "|" + nonce
	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(sig)
}

// buildConnectRequest builds a request carrying the connect query parameters.
func buildConnectRequest(t *testing.T, deviceID, ts, nonce, sig string) *http.Request {
	t.Helper()

	url := "/v1/device/" + deviceID + "/stream?sig_timestamp=" + ts +
		"&sig_nonce=" + nonce + "&sig_signature=" + sig

	return httptest.NewRequest("GET", url, nil)
}

func TestVerifyDeviceConnect_AcceptsAFreshSignature(t *testing.T) {
	upgrader, key := connectFixture(t)

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "n-1"
	sig := signConnect(t, key, "dev-1", ts, nonce)

	if err := upgrader.verifyDeviceConnect(buildConnectRequest(t, "dev-1", ts, nonce, sig), "dev-1"); err != nil {
		t.Fatalf("a fresh connect must be accepted: %v", err)
	}
}

func TestVerifyDeviceConnect_RejectsAStaleTimestamp(t *testing.T) {
	upgrader, key := connectFixture(t)

	ts := strconv.FormatInt(time.Now().Add(-time.Hour).UnixMilli(), 10)
	nonce := "n-2"
	sig := signConnect(t, key, "dev-1", ts, nonce)

	if err := upgrader.verifyDeviceConnect(buildConnectRequest(t, "dev-1", ts, nonce, sig), "dev-1"); err == nil {
		t.Fatal("a stale connect timestamp must be rejected")
	}
}

func TestVerifyDeviceConnect_RejectsAReplayedNonce(t *testing.T) {
	upgrader, key := connectFixture(t)

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "n-3"
	sig := signConnect(t, key, "dev-1", ts, nonce)

	if err := upgrader.verifyDeviceConnect(buildConnectRequest(t, "dev-1", ts, nonce, sig), "dev-1"); err != nil {
		t.Fatalf("the first connect must be accepted: %v", err)
	}
	if err := upgrader.verifyDeviceConnect(buildConnectRequest(t, "dev-1", ts, nonce, sig), "dev-1"); err == nil {
		t.Fatal("a replayed connect nonce must be rejected")
	}
}

func TestVerifyDeviceConnect_RejectsABadSignature(t *testing.T) {
	upgrader, key := connectFixture(t)

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "n-4"
	sig := signConnect(t, key, "dev-1", ts, nonce)
	// The signature was over a different device id.
	if err := upgrader.verifyDeviceConnect(buildConnectRequest(t, "dev-2", ts, nonce, sig), "dev-2"); err == nil {
		t.Fatal("a signature for another device must be rejected")
	}
}

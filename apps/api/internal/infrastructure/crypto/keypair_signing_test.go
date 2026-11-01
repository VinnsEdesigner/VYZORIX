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

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
)

// newTestServerSigner returns a signer plus its public PEM, which is what a
// device would have pinned.
func newTestServerSigner(t *testing.T) (*ServerSigner, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	signer, err := NewServerSigner(privPEM)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	pubPEM, err := EncodePublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("public pem: %v", err)
	}

	return signer, pubPEM
}

// TestCommandFrameSignsAndVerifiesWithAKey is the replacement for the HMAC round
// trip: the server signs with its private key, a verifier checks with the public
// half, and no shared secret is involved.
func TestCommandFrameSignsAndVerifiesWithAKey(t *testing.T) {
	signer, pubPEM := newTestServerSigner(t)

	frame := &command.CommandFrame{
		DispatchID: "d1",
		Command:    "SHELL_EXEC",
		Args:       []byte(`{"cmd":"uname -a"}`),
	}

	if err := SignCommandFrameWithKey(signer, frame, "dev-1"); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if frame.Signature == "" || frame.Nonce == "" {
		t.Fatal("signing must set the signature and nonce")
	}
	if frame.Timestamp == 0 {
		t.Fatal("signing must set the timestamp")
	}

	if !ValidateCommandSignatureWithKey(pubPEM, frame, "dev-1") {
		t.Fatal("a frame signed with the server key must verify")
	}
}

// TestCommandSignatureRefusesTampering covers the fields an attacker would change.
func TestCommandSignatureRefusesTampering(t *testing.T) {
	signer, pubPEM := newTestServerSigner(t)

	base := func() *command.CommandFrame {
		f := &command.CommandFrame{
			DispatchID: "d1",
			Command:    "SHELL_EXEC",
			Args:       []byte(`{"cmd":"id"}`),
		}
		if err := SignCommandFrameWithKey(signer, f, "dev-1"); err != nil {
			t.Fatalf("sign: %v", err)
		}

		return f
	}

	cases := map[string]func(*command.CommandFrame){
		"changed command":    func(f *command.CommandFrame) { f.Command = "FACTORY_RESET" },
		"changed args":       func(f *command.CommandFrame) { f.Args = []byte(`{"cmd":"rm -rf /"}`) },
		"changed dispatchId": func(f *command.CommandFrame) { f.DispatchID = "d2" },
		"changed timestamp":  func(f *command.CommandFrame) { f.Timestamp++ },
		"changed nonce":      func(f *command.CommandFrame) { f.Nonce = "tampered" },
		"dropped signature":  func(f *command.CommandFrame) { f.Signature = "" },
	}

	for name, mutate := range cases {
		f := base()
		mutate(f)
		if ValidateCommandSignatureWithKey(pubPEM, f, "dev-1") {
			t.Errorf("%s: a tampered frame must not verify", name)
		}
	}
}

// TestCommandSignatureIsBoundToTheDevice proves one device's frame cannot be
// replayed as another's.
func TestCommandSignatureIsBoundToTheDevice(t *testing.T) {
	signer, pubPEM := newTestServerSigner(t)

	frame := &command.CommandFrame{DispatchID: "d1", Command: "SHELL_EXEC"}
	if err := SignCommandFrameWithKey(signer, frame, "dev-1"); err != nil {
		t.Fatalf("sign: %v", err)
	}

	if ValidateCommandSignatureWithKey(pubPEM, frame, "dev-2") {
		t.Fatal("a frame bound to dev-1 must not verify for dev-2")
	}
}

// TestWrongKeyRefusesTheFrame proves a signature is not accepted under an
// unrelated key.
func TestWrongKeyRefusesTheFrame(t *testing.T) {
	signer, _ := newTestServerSigner(t)
	_, otherPub := newTestServerSigner(t)

	frame := &command.CommandFrame{DispatchID: "d1", Command: "SHELL_EXEC"}
	if err := SignCommandFrameWithKey(signer, frame, "dev-1"); err != nil {
		t.Fatalf("sign: %v", err)
	}

	if ValidateCommandSignatureWithKey(otherPub, frame, "dev-1") {
		t.Fatal("a frame must not verify under an unrelated key")
	}
}

// TestConnectSignatureVerifies proves the device's WebSocket handshake works with
// a keypair, which is what replaces ValidateConnectHMAC.
func TestConnectSignatureVerifies(t *testing.T) {
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	pubPEM, _ := EncodePublicKey(&deviceKey.PublicKey)

	deviceID := "dev-1"
	timestamp := strconv.FormatInt(1700000000, 10)
	nonce := "n1"

	// The device signs the canonical string with its own private key.
	canonical := ConnectCanonical(deviceID, timestamp, nonce)
	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, deviceKey, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	signature := base64.StdEncoding.EncodeToString(sig)

	if !VerifyConnectSignature(pubPEM, deviceID, timestamp, nonce, signature) {
		t.Fatal("a valid connect signature must verify")
	}

	// A different device id must not verify, or one device could open another's stream.
	if VerifyConnectSignature(pubPEM, "dev-2", timestamp, nonce, signature) {
		t.Fatal("a connect signature bound to dev-1 must not verify for dev-2")
	}
	if VerifyConnectSignature(pubPEM, deviceID, timestamp, "n2", signature) {
		t.Fatal("a changed nonce must not verify")
	}
}

// TestCommandAndConnectDomainsDiffer proves a command signature cannot be
// replayed as a connect signature.
func TestCommandAndConnectDomainsDiffer(t *testing.T) {
	if CommandSignKind == ConnectSignKind {
		t.Fatal("the command and connect domains must differ")
	}
}

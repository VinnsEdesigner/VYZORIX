package device

import (
	"context"
	"testing"

	libv "github.com/VinnsEdesigner/libv/go"
)

// TestDeliveredConfigIsVerifiableByTheFormat is the guard on the bug that made
// every delivered config unverifiable: the server signed the raw document text,
// and never appended a signature line at all, so the device's verifier — which
// checks the format's canonical string — could never accept it.
//
// The assertion is the device's own check: parse with libv, then Verify against
// the pinned public key.
func TestDeliveredConfigIsVerifiableByTheFormat(t *testing.T) {
	svc := newConfigService(t, true)
	signer := newConfigSigner(t)

	pubPEM, err := signer.PublicKeyPEM()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	pinned, err := libv.DecodePublicKey(pubPEM)
	if err != nil {
		t.Fatalf("decode pinned key: %v", err)
	}

	envelope, err := svc.ConfigForDevice(context.Background(), configSign(signer), "dev-1")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	doc, err := libv.Parse(envelope.Document)
	if err != nil {
		t.Fatalf("the delivered config must parse: %v\n%s", err, envelope.Document)
	}
	if !doc.Signed {
		t.Fatalf("the delivered config must carry a signature line:\n%s", envelope.Document)
	}

	if err := libv.Verify(doc, envelope.Document, pinned, nil); err != nil {
		t.Fatalf("the delivered config must verify against the pinned key: %v", err)
	}
}

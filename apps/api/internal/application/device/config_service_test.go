package device

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"strings"
	"testing"

	libv "github.com/VinnsEdesigner/libv/go"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
)

// newConfigService builds a Service over a real in-memory database with a claimed
// device, so the config is assembled from a record that actually round-trips.
func newConfigService(t *testing.T, claimed bool) *Service {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE devices (
		id TEXT PRIMARY KEY, firebase_install_id TEXT, fcm_token TEXT, app_version TEXT,
		device_class TEXT, online INTEGER NOT NULL DEFAULT 0,
		registered_at INTEGER NOT NULL DEFAULT 0, last_seen INTEGER NOT NULL DEFAULT 0,
		operator_id TEXT, organization_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP,
		device_name TEXT, manufacturer TEXT, model TEXT, os_version TEXT, security_patch TEXT,
			build_id TEXT, api_level INTEGER NOT NULL DEFAULT 0, supported_abis TEXT,
			ram_bytes INTEGER NOT NULL DEFAULT 0, storage_bytes INTEGER NOT NULL DEFAULT 0,
		deregistered_at INTEGER, deletion_scheduled_at INTEGER, fcm_token_refreshed_at INTEGER,
		tags TEXT, tier INTEGER NOT NULL DEFAULT 1, public_key TEXT, secret_public_key TEXT, pairing_code TEXT,
		pairing_expires_at INTEGER)`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	d := device.NewDevice("dev-1", "")
	d.Tier = device.TierRooted
	if claimed {
		d.OrganizationID = "org-1"
		d.DeviceName = "field-ops-07"
	}
	if err := storage.NewDeviceRepository(db).Create(context.Background(), d); err != nil {
		t.Fatalf("seed: %v", err)
	}

	return NewService(storage.NewDeviceRepository(db), nil, nil)
}

// configSign adapts a libv signer to the service's SignDocument, which is the
// same primitive the server's ServerSigner provides.
func configSign(signer *libv.Signer) SignDocument {
	return func(document string) (string, error) { return signer.Sign(document) }
}

func newConfigSigner(t *testing.T) *libv.Signer {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	signer, err := libv.NewSignerFromPEM(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	return signer
}

// TestConfigIsSignedAndVerifies is the round trip: a device must be able to check
// the config against the public key it has pinned.
func TestConfigIsSignedAndVerifies(t *testing.T) {
	svc := newConfigService(t, true)
	signer := newConfigSigner(t)
	pubPEM, err := signer.PublicKeyPEM()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}

	envelope, err := svc.ConfigForDevice(context.Background(), configSign(signer), "dev-1")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	doc, err := libv.Parse(envelope.Document)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, envelope.Document)
	}
	if !doc.Signed {
		t.Fatal("a config must be signed")
	}

	pub, err := libv.DecodePublicKey(pubPEM)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := libv.Verify(doc, envelope.Document, pub, nil); err != nil {
		t.Fatalf("a config must verify: %v", err)
	}
}

// TestConfigCarriesTheDeviceFacts proves the values come from the record rather
// than from defaults.
func TestConfigCarriesTheDeviceFacts(t *testing.T) {
	svc := newConfigService(t, true)
	envelope, err := svc.ConfigForDevice(context.Background(), configSign(newConfigSigner(t)), "dev-1")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	doc, err := libv.Parse(envelope.Document)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	deviceNode := doc.Root.Find("device")
	if len(deviceNode) != 1 || deviceNode[0].Tokens[1] != "dev-1" {
		t.Fatalf("device node = %+v", deviceNode)
	}

	org := doc.Root.Find("device.org")
	if len(org) != 1 || org[0].Tokens[1] != "org-1" {
		t.Fatalf("org node = %+v", org)
	}

	name := doc.Root.Find("device.name")
	if len(name) != 1 || name[0].Tokens[1] != "field-ops-07" {
		t.Fatalf("name node = %+v", name)
	}

	tier := doc.Root.Find("device.tier")
	if len(tier) != 1 || tier[0].Tokens[1] != "2" {
		t.Fatalf("tier node = %+v", tier)
	}
}

// TestConfigRefusesAnUnclaimedDevice proves a device with no organization cannot
// be configured: there is nothing to bind it to yet.
func TestConfigRefusesAnUnclaimedDevice(t *testing.T) {
	svc := newConfigService(t, false)

	if _, err := svc.ConfigForDevice(context.Background(), configSign(newConfigSigner(t)), "dev-1"); err == nil {
		t.Fatal("an unclaimed device must not receive a config")
	}
}

func TestConfigRequiresASigningKey(t *testing.T) {
	svc := newConfigService(t, true)

	if _, err := svc.ConfigForDevice(context.Background(), nil, "dev-1"); err == nil {
		t.Fatal("a config must not be issued unsigned")
	}
}

// TestConfigCarriesNoSecret is the invariant the whole design rests on: a config
// is transmitted, so it must never contain a key, a token or a password.
func TestConfigCarriesNoSecret(t *testing.T) {
	svc := newConfigService(t, true)
	envelope, err := svc.ConfigForDevice(context.Background(), configSign(newConfigSigner(t)), "dev-1")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if err := ValidateConfigHasNoSecrets(envelope.Document); err != nil {
		t.Fatalf("a real config must pass its own check: %v", err)
	}
}

// TestSecretCheckRejectsASecret proves the check actually fires, so the invariant
// above is not vacuous.
func TestSecretCheckRejectsASecret(t *testing.T) {
	cases := map[string]string{
		"token field":    "v device d1\nvv token abc123\n",
		"secret field":   "v device d1\nvv secret abc123\n",
		"password field": "v device d1\nvv password hunter2\n",
		"hmac field":     "v device d1\nvv hmac sha256 deadbeef\n",
		"key field":      "v device d1\nvv key -----BEGIN PRIVATE KEY-----\n",
		"nested secret":  "v device d1\nvv cloud\nvvv api_token xyz\n",
	}

	for name, doc := range cases {
		if err := ValidateConfigHasNoSecrets(doc); err == nil {
			t.Errorf("%s: a config carrying a secret must be refused", name)
		}
	}
}

// TestSecretCheckAllowsTheSignatureLine proves the algorithm name is not mistaken
// for a secret, or every signed config would be refused.
func TestSecretCheckAllowsTheSignatureLine(t *testing.T) {
	doc := "v device d1\nv sig ecdsa-p256 MEYCIQ==\n"

	if err := ValidateConfigHasNoSecrets(doc); err != nil {
		t.Fatalf("the signature line must not read as a secret: %v", err)
	}
}

// TestRenderedConfigIsParseableByTheFormat proves the server writes documents the
// format's own parser accepts — the cross-repo contract, exercised here.
func TestRenderedConfigIsParseableByTheFormat(t *testing.T) {
	svc := newConfigService(t, true)
	envelope, err := svc.ConfigForDevice(context.Background(), configSign(newConfigSigner(t)), "dev-1")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if !strings.Contains(envelope.Document, "v device dev-1") {
		t.Errorf("document missing the device line:\n%s", envelope.Document)
	}
	if !strings.Contains(envelope.Document, "vvv terminal true") {
		t.Errorf("document missing the feature flags:\n%s", envelope.Document)
	}

	// Parse, then look up a nested node, which proves the depths are right.
	doc, err := libv.Parse(envelope.Document)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, envelope.Document)
	}
	if got := doc.Root.Find("device.cloud.stream"); len(got) != 1 {
		t.Fatalf("stream node not found at the expected depth: %+v", got)
	}
}

// TestConfigEnablesApps is the flag §5.1 turns on: a registered device runs apps,
// so its config must say so, or the device-side gate stays shut and nothing a
// deploy installs ever starts.
func TestConfigEnablesApps(t *testing.T) {
	svc := newConfigService(t, true)
	envelope, err := svc.ConfigForDevice(context.Background(), configSign(newConfigSigner(t)), "dev-1")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if !strings.Contains(envelope.Document, "vvv apps true") {
		t.Errorf("config must enable apps:\n%s", envelope.Document)
	}
}

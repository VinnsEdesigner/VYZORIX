package deploy

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	libv "github.com/VinnsEdesigner/libv/go"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// twoAppManifest is the US-012 fixture: a platform, not a hardcoded feature.
//
// The `pwa` key must precede the `[[app.artifact]]` blocks: once a TOML reader is
// inside an array-of-tables element, a bare key belongs to that element. And the
// boolean form (`pwa = true`) and the table form (`[app.pwa]`) are mutually
// exclusive, because both define the key `pwa`.
const twoAppManifest = `
[device]
id  = "d_01H"
org = "org_01H"
name = "field-ops-07"

[[app]]
name = "vfd"
cmd  = "/home/apps/vfd/run.sh"
port = 8080
expose = "loopback"

  [app.pwa]
  theme_color = "#be123c"
  display = "standalone"

  [app.env]
  MI_PORT = "8080"

  [app.secrets]
  DATABASE_URL = ""

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/vyzor-arm64.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
  arch = "arm64-v8a"

  [[app.artifact]]
  name = "console"
  url = "https://example.test/console.tar.gz"
  sha256 = "41ab000000000000000000000000000000000000000000000000000000000000"
  kind = "webroot"

[[app]]
name = "wiki"
cmd  = "/home/apps/wiki/run.sh"
port = 9000
expose = "lan"
pwa = true

  [[app.artifact]]
  name = "site"
  url = "https://example.test/wiki.tar.gz"
  sha256 = "7b3d000000000000000000000000000000000000000000000000000000000000"
  kind = "webroot"
`

// signer returns a deterministic signer for the tests, plus its public key.
func signer(t *testing.T) (DocumentSigner, *ecdsa.PublicKey) {
	t.Helper()

	priv, err := crypto.GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	s, err := libv.NewSignerFromPEM(privateKeyPEM(t, priv))
	if err != nil {
		t.Fatalf("build signer: %v", err)
	}

	return s.Sign, &priv.PublicKey
}

func privateKeyPEM(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func injection() Injection {
	return Injection{
		MemberID:         "op_01H",
		DeviceID:         "d_01H",
		OrgID:            "org_01H",
		Name:             "field-ops-07",
		StreamPath:       "/v1/device/d_01H/stream",
		Version:          7,
		TierCap:          2,
		HeartbeatSeconds: 60,
		TerminalEnabled:  true,
	}
}

// compileFixture parses, compiles and signs the two-app manifest.
func compileFixture(t *testing.T) (string, *ecdsa.PublicKey) {
	t.Helper()

	m, err := ParseManifest([]byte(twoAppManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	sign, pub := signer(t)

	signed, err := CompileAndSign(m, injection(), sign)
	if err != nil {
		t.Fatalf("compile and sign: %v", err)
	}

	return signed, pub
}

// TestCompileAndVerifyRoundTrip is the trust model in one test: compile, sign,
// verify against the public key.
func TestCompileAndVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	signed, pub := compileFixture(t)

	doc, err := libv.Parse(signed)
	if err != nil {
		t.Fatalf("parse signed: %v", err)
	}

	if err := libv.Verify(doc, signed, pub, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// TestTamperedConfigFailsVerification flips one byte and expects refusal.
func TestTamperedConfigFailsVerification(t *testing.T) {
	t.Parallel()

	signed, pub := compileFixture(t)

	tampered := strings.Replace(signed, "port 8080", "port 8081", 1)
	if tampered == signed {
		t.Fatal("fixture did not contain the field the test tampers with")
	}

	doc, err := libv.Parse(tampered)
	if err != nil {
		t.Fatalf("parse tampered: %v", err)
	}

	if err := libv.Verify(doc, tampered, pub, nil); err == nil {
		t.Fatal("a tampered config must fail verification")
	}
}

// TestWrongKeyFailsVerification proves a config signed for one key does not
// verify under another.
func TestWrongKeyFailsVerification(t *testing.T) {
	t.Parallel()

	signed, _ := compileFixture(t)

	other, err := crypto.GenerateDeviceKeyPair()
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	doc, err := libv.Parse(signed)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := libv.Verify(doc, signed, &other.PublicKey, nil); err == nil {
		t.Fatal("a config must not verify under a key that did not sign it")
	}
}

// TestTwoAppsCompile proves the US-012 property: a second app compiles with no
// platform change, and both are present with their own fields.
func TestTwoAppsCompile(t *testing.T) {
	t.Parallel()

	signed, _ := compileFixture(t)

	doc, err := libv.Parse(signed)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	apps := doc.Root.Find("device.apps.app")
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	names := []string{apps[0].Tokens[1], apps[1].Tokens[1]}
	if names[0] != "vfd" || names[1] != "wiki" {
		t.Fatalf("app order must follow the manifest, got %v", names)
	}

	// The reader rule: fields are read relative to each app node, because an
	// absolute path flattens across apps.
	ports := appField(apps[0], "port")
	if ports != "8080" {
		t.Fatalf("vfd port: got %q", ports)
	}

	if p := appField(apps[1], "port"); p != "9000" {
		t.Fatalf("wiki port: got %q", p)
	}
}

// TestCompileIsDeterministic proves byte-identical output for the same input.
func TestCompileIsDeterministic(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(twoAppManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cfg, err := Compile(m, injection())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	first, err := RenderConfig(*cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	second, err := RenderConfig(*cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if first != second {
		t.Fatal("the same config must render byte-identically")
	}
}

// TestRandomizedEnvStillDeterministic proves the sorted env write, which is what
// makes a map deterministic.
func TestRandomizedEnvStillDeterministic(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(twoAppManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	m.Apps[0].Env = map[string]string{
		"ZED": "1", "ALPHA": "2", "MI_PORT": "8080", "BETA": "3", "OMEGA": "4",
	}

	cfg, err := Compile(m, injection())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	first, err := RenderConfig(*cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Rebuild the map with a different insertion order; the render must not move.
	m.Apps[0].Env = map[string]string{
		"OMEGA": "4", "BETA": "3", "MI_PORT": "8080", "ALPHA": "2", "ZED": "1",
	}

	cfg2, err := Compile(m, injection())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	second, err := RenderConfig(*cfg2)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if first != second {
		t.Fatal("env map order must not change the rendered document")
	}
}

// appField reads one child of an app node by key. It is the reader rule of
// contract §5.1 in one function: never use an absolute path across apps.
func appField(app *libv.Node, key string) string {
	for _, child := range app.Children {
		if child.Key() == key && len(child.Tokens) > 1 {
			return child.Tokens[1]
		}
	}

	return ""
}

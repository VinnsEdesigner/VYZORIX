package deploy

import (
	"errors"
	"strings"
	"testing"
)

// The refusals are the invariants; a regression in any is silent, so each is
// asserted by the message it produces, not merely by "an error".

// manifestWith returns a one-app manifest with the given [[app]] body.
func manifestWith(t *testing.T, appBlock string) []byte {
	t.Helper()

	return []byte("[device]\nid = \"d_01H\"\norg = \"org_01H\"\n\n[[app]]\n" + appBlock)
}

// TestUnknownKeyIsRefused proves a typo'd setting does not silently do nothing.
func TestUnknownKeyIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/home/apps/vfd/run.sh\"\nauto_start = true\n"))
	if err == nil {
		t.Fatal("an unknown key must be refused")
	}

	if !strings.Contains(err.Error(), "auto_start") {
		t.Fatalf("the refusal must name the unknown key, got %q", err)
	}
}

// TestMissingSha256IsRefused names the artifact, because an operator has to find it.
func TestMissingSha256IsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/e.tar.gz"
`))
	if err == nil {
		t.Fatal("an artifact without a sha256 must be refused")
	}

	if !strings.Contains(err.Error(), "sha256") || !strings.Contains(err.Error(), "engine") {
		t.Fatalf("the refusal must name the artifact and the field, got %q", err)
	}
}

// TestCmdOutsideAppDirIsRefused is the rule at compile time.
func TestCmdOutsideAppDirIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/bin/sh\"\n"))
	if err == nil {
		t.Fatal("a cmd outside the app directory must be refused")
	}

	if !strings.Contains(err.Error(), "must resolve inside") {
		t.Fatalf("the refusal must explain the directory rule, got %q", err)
	}
}

// TestCmdEscapeViaParentIsRefused proves the cleaned path is checked, so a `..`
// cannot slip past the prefix test.
func TestCmdEscapeViaParentIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/home/apps/vfd/../../bin/sh\"\n"))
	if err == nil {
		t.Fatal("a cmd that escapes via .. must be refused")
	}

	if !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("the refusal must name the escape, got %q", err)
	}
}

// TestCmdMetacharactersAreRefused covers the shell characters the rule bans.
func TestCmdMetacharactersAreRefused(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{
		"/home/apps/vfd/run.sh; rm -rf /",
		"/home/apps/vfd/run.sh && whoami",
		"/home/apps/vfd/$(id).sh",
		"/home/apps/vfd/run.sh | tee /tmp/x",
	} {
		_, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \""+bad+"\"\n"))
		if err == nil {
			t.Fatalf("cmd %q must be refused", bad)
		}
	}
}

// TestCloudExposeIsRefusedInV1 covers the deferred mode.
func TestCloudExposeIsRefusedInV1(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/home/apps/vfd/run.sh\"\nport = 8080\nexpose = \"cloud\"\n"))
	if err == nil {
		t.Fatal("expose = cloud must be refused in v1")
	}

	if !strings.Contains(err.Error(), "not supported in v1") {
		t.Fatalf("the refusal must say why, got %q", err)
	}
}

// TestNonLoopbackRequiresPort proves the port rule.
func TestNonLoopbackRequiresPort(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/home/apps/vfd/run.sh\"\nexpose = \"lan\"\n"))
	if err == nil {
		t.Fatal("expose = lan without a port must be refused")
	}

	if !strings.Contains(err.Error(), "port") {
		t.Fatalf("the refusal must name the missing port, got %q", err)
	}
}

// TestInvalidDisplayIsRefused covers the value that silently degrades to browser.
func TestInvalidDisplayIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [app.pwa]
  display = "fullscreen-ish"
`))
	if err == nil {
		t.Fatal("an invalid display value must be refused")
	}

	if !strings.Contains(err.Error(), "display") {
		t.Fatalf("the refusal must name display, got %q", err)
	}
}

// TestSecretValueInTableIsRefused proves [app.secrets] carries names only.
func TestSecretValueInTableIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [app.secrets]
  DATABASE_URL = "postgres://user:pw@db/app"
`))
	if err == nil {
		t.Fatal("a secret table with a value must be refused")
	}

	if !strings.Contains(err.Error(), "names only") {
		t.Fatalf("the refusal must explain the names-only rule, got %q", err)
	}
}

// TestConfigNamingSecretsIsAccepted is the other half: the scan must not refuse
// the schema's own `secret NAME` nodes.
func TestConfigNamingSecretsIsAccepted(t *testing.T) {
	t.Parallel()

	signed, _ := compileFixture(t)

	if !strings.Contains(signed, "secret DATABASE_URL") {
		t.Fatal("the config must name the app's secret")
	}
}

// TestConfigCarryingASecretIsRefused: a value that is a secret is still refused.
func TestConfigCarryingASecretIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"jwt in env":       "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		"hex run in env":   "9f2c1b3a4d5e6f708192a3b4c5d6e7f8091a2b3c4",
		"url with creds":   "postgres://user:password@db.internal:5432/app",
		"pem block in env": "-----BEGIN PRIVATE KEY-----",
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [app.env]
  CONFIG = "`+value+`"
`))
			if err != nil {
				t.Fatalf("the manifest must parse; the scan runs at compile time: %v", err)
			}

			sign, _ := signer(t)

			if _, err := CompileAndSign(m, injection(), sign); err == nil {
				t.Fatalf("a config carrying %s must be refused", name)
			}
		})
	}
}

// TestEnvKeyNamingASecretIsRefused covers the key side of the scan.
func TestEnvKeyNamingASecretIsRefused(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [app.env]
  API_TOKEN = "short"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	sign, _ := signer(t)

	if _, err := CompileAndSign(m, injection(), sign); err == nil {
		t.Fatal("an env key naming a secret must be refused")
	}
}

// TestDuplicateAppIsRefused proves names are unique within a file.
func TestDuplicateAppIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest([]byte(`
[device]
id = "d_01H"
org = "org_01H"

[[app]]
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

[[app]]
name = "vfd"
cmd = "/home/apps/vfd/run.sh"
`))
	if err == nil {
		t.Fatal("a duplicate app name must be refused")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("the refusal must say duplicate, got %q", err)
	}
}

// TestDuplicateArtifactPerArchIsRefused covers the per-arch uniqueness rule.
func TestDuplicateArtifactPerArchIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/a.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
  arch = "arm64-v8a"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/b.tar.gz"
  sha256 = "41ab000000000000000000000000000000000000000000000000000000000000"
  arch = "arm64-v8a"
`))
	if err == nil {
		t.Fatal("two artifacts with the same name and arch must be refused")
	}

	if !strings.Contains(err.Error(), "duplicate artifact") {
		t.Fatalf("the refusal must say duplicate artifact, got %q", err)
	}
}

// TestSameArtifactDifferentArchIsAccepted proves the per-arch form, which is how
// one binary ships to several ABIs.
func TestSameArtifactDifferentArchIsAccepted(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/arm64.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
  arch = "arm64-v8a"

  [[app.artifact]]
  name = "engine"
  url = "https://example.test/x86.tar.gz"
  sha256 = "41ab000000000000000000000000000000000000000000000000000000000000"
  arch = "x86_64"
`))
	if err != nil {
		t.Fatalf("the same artifact for two arches must be accepted: %v", err)
	}
}

// TestCredentialInArtifactURLIsRefused: the scan must reach nested values.
func TestCredentialInArtifactURLIsRefused(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [[app.artifact]]
  name = "engine"
  url = "https://user:s3cr3t@example.test/e.tar.gz"
  sha256 = "9f2c000000000000000000000000000000000000000000000000000000000000"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	sign, _ := signer(t)

	if _, err := CompileAndSign(m, injection(), sign); err == nil {
		t.Fatal("a credential in an artifact URL must be refused")
	}
}

// TestCredentialInPwaFieldIsRefused covers the other nested value.
func TestCredentialInPwaFieldIsRefused(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest(manifestWith(t, `
name = "vfd"
cmd = "/home/apps/vfd/run.sh"

  [app.pwa]
  description = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	sign, _ := signer(t)

	if _, err := CompileAndSign(m, injection(), sign); err == nil {
		t.Fatal("a credential in a PWA field must be refused")
	}
}

// TestCredentialInATopLevelValueIsRefused covers the depth the walk must reach:
// a credential in `device.name` is as much a secret as one in an env, and the
// scan must not stop at the app nodes.
func TestCredentialInATopLevelValueIsRefused(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(twoAppManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	sign, _ := signer(t)

	in := injection()
	in.Name = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"

	if _, err := CompileAndSign(m, in, sign); err == nil {
		t.Fatal("a credential in a top-level value must be refused")
	}
}

// TestArtifactDigestIsNotASecret guards the false positive: a bare-hex rule
// outside env would refuse every artifact in every manifest.
func TestArtifactDigestIsNotASecret(t *testing.T) {
	t.Parallel()

	signed, _ := compileFixture(t)

	if !strings.Contains(signed, "sha256 9f2c") {
		t.Fatal("the digest must survive the scan")
	}
}

// TestAutostartDefaultsToTrue: a Go zero value would otherwise invert it.
func TestAutostartDefaultsToTrue(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/home/apps/vfd/run.sh\"\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !m.Apps[0].Autostart {
		t.Fatal("autostart must default to true")
	}
}

// TestAutostartExplicitFalseIsHonoured: the default does not override the file.
func TestAutostartExplicitFalseIsHonoured(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest(manifestWith(t, "name = \"vfd\"\ncmd = \"/home/apps/vfd/run.sh\"\nautostart = false\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if m.Apps[0].Autostart {
		t.Fatal("autostart = false must be honoured")
	}
}

// TestMissingDeviceIDIsRefused proves the target is required.
func TestMissingDeviceIDIsRefused(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest([]byte("[device]\norg = \"org_01H\"\n\n[[app]]\nname = \"vfd\"\ncmd = \"/home/apps/vfd/run.sh\"\n"))
	if err == nil {
		t.Fatal("a manifest without device.id must be refused")
	}

	if !strings.Contains(err.Error(), "device.id") {
		t.Fatalf("the refusal must name device.id, got %q", err)
	}
}

// TestNoSignerIsRefused proves a compile cannot produce an unsigned config.
func TestNoSignerIsRefused(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(twoAppManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if _, err := CompileAndSign(m, injection(), nil); err == nil {
		t.Fatal("compiling with no signer must be refused")
	}
}

// TestUnsignedResultIsRefused proves SignConfig checks the signature line it got
// back, rather than trusting the signer. A document with no signature line would
// otherwise be stored and served as though it were signed.
func TestUnsignedResultIsRefused(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(twoAppManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	identity := func(document string) (string, error) { return document, nil }

	if _, err := CompileAndSign(m, injection(), identity); !errors.Is(err, ErrNotSigned) {
		t.Fatalf("a signer that returns the input unchanged must be refused, got %v", err)
	}
}

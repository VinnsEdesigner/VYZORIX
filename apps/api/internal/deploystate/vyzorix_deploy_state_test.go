package deploystate

import (
	"strings"
	"testing"
)

func TestCompareInSync(t *testing.T) {
	d := Compare(
		Expected{HasConfig: true, Version: 7, AppDigests: map[string]string{"vfd": "abc123"}},
		Reported{ReportedState: true, Version: 7, AppDigests: map[string]string{"vfd": "abc123"}},
	)

	if d.State != DriftNone {
		t.Fatalf("state = %q, want %q (%s)", d.State, DriftNone, d.Detail)
	}
}

func TestCompareBehind(t *testing.T) {
	// The case a failed apply produces: the device still holds the old version.
	d := Compare(
		Expected{HasConfig: true, Version: 7, AppDigests: map[string]string{"vfd": "abc"}},
		Reported{ReportedState: true, Version: 6, AppDigests: map[string]string{"vfd": "abc"}},
	)

	if d.State != DriftBehind {
		t.Fatalf("state = %q, want %q", d.State, DriftBehind)
	}
	if !strings.Contains(d.Detail, "7") || !strings.Contains(d.Detail, "6") {
		t.Fatalf("detail should name both versions, got %q", d.Detail)
	}
}

func TestCompareAhead(t *testing.T) {
	d := Compare(
		Expected{HasConfig: true, Version: 6, AppDigests: map[string]string{"vfd": "abc"}},
		Reported{ReportedState: true, Version: 7, AppDigests: map[string]string{"vfd": "abc"}},
	)

	if d.State != DriftAhead {
		t.Fatalf("state = %q, want %q", d.State, DriftAhead)
	}
}

func TestCompareAppChangedAtTheSameVersion(t *testing.T) {
	// The drift the version alone cannot see: same version, different bytes. An app
	// edited or replaced in place.
	d := Compare(
		Expected{HasConfig: true, Version: 7, AppDigests: map[string]string{"vfd": "aaaa1111"}},
		Reported{ReportedState: true, Version: 7, AppDigests: map[string]string{"vfd": "bbbb2222"}},
	)

	if d.State != DriftAppChanged {
		t.Fatalf("state = %q, want %q", d.State, DriftAppChanged)
	}
	if !strings.Contains(d.Detail, "vfd") {
		t.Fatalf("detail should name the app, got %q", d.Detail)
	}
}

func TestCompareAppMissing(t *testing.T) {
	d := Compare(
		Expected{HasConfig: true, Version: 7, AppDigests: map[string]string{"vfd": "aaaa1111"}},
		Reported{ReportedState: true, Version: 7, AppDigests: map[string]string{}},
	)

	if d.State != DriftAppChanged {
		t.Fatalf("state = %q, want %q", d.State, DriftAppChanged)
	}
	if !strings.Contains(d.Detail, "not installed") {
		t.Fatalf("detail should say it is not installed, got %q", d.Detail)
	}
}

// TestCompareUnknownIsNeverHealthy is the load-bearing one: a device that reports
// no state, or a server with no config, must never read as in-sync. A green tick
// that was not earned is worse than an admitted unknown.
func TestCompareUnknownIsNeverHealthy(t *testing.T) {
	cases := []struct {
		name     string
		expected Expected
		reported Reported
	}{
		{
			name:     "device reports nothing",
			expected: Expected{HasConfig: true, Version: 7},
			reported: Reported{ReportedState: false, Version: 0},
		},
		{
			name:     "server holds no config",
			expected: Expected{HasConfig: false},
			reported: Reported{ReportedState: true, Version: 7},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Compare(tc.expected, tc.reported)
			if d.State != DriftUnknown {
				t.Fatalf("state = %q, want %q", d.State, DriftUnknown)
			}
			if d.Detail == "" {
				t.Fatal("an unknown state must say why, or it reads as a bug")
			}
		})
	}
}

// TestAppStateDigestIsOrderIndependent pins the property that makes the digest
// usable: a map has no order, so the digest must not depend on one.
func TestAppStateDigestIsOrderIndependent(t *testing.T) {
	a := AppStateDigest(map[string]string{"engine": "11", "console": "22", "extractors": "33"})
	b := AppStateDigest(map[string]string{"extractors": "33", "engine": "11", "console": "22"})

	if a != b {
		t.Fatalf("digest depends on map order: %q vs %q", a, b)
	}
	if len(a) != 16 {
		t.Fatalf("digest = %q (len %d), want 16 hex characters", a, len(a))
	}
}

func TestAppStateDigestChangesWithContent(t *testing.T) {
	a := AppStateDigest(map[string]string{"engine": "1111"})
	b := AppStateDigest(map[string]string{"engine": "2222"})

	if a == b {
		t.Fatal("a different sha256 must produce a different digest")
	}
}

func TestAppStateDigestEmptyIsEmpty(t *testing.T) {
	if got := AppStateDigest(nil); got != "" {
		t.Fatalf("digest of no artifacts = %q, want empty", got)
	}
}

// TestAppStateDigestMatchesTheDeviceAlgorithm is the cross-language contract.
//
// The value below is produced by the *device's* algorithm
// (`InstalledStateStore.digest`, Vyzorix-android). If this test fails, the two
// sides have drifted and **every** device will read as drifted, so it is worth
// failing loudly rather than discovering it in production.
//
// The expected value is computed by the same construction the device uses:
//
//	body = "console=2222\nengine=1111"     (sorted by artifact name, newline-joined)
//	digest = hex(sha256(body))[:16]
//
// The device's test suite asserts the same literal from its side, so a change to
// either implementation breaks one of the two.
func TestAppStateDigestMatchesTheDeviceAlgorithm(t *testing.T) {
	got := AppStateDigest(map[string]string{"engine": "1111", "console": "2222"})
	const want = "47584980637a11e1"

	if got != want {
		t.Fatalf(
			"the digest has drifted from the device's algorithm: got %q, want %q.\n"+
				"Both sides must compute the same value or every device reads as drifted.\n"+
				"Body is 'console=2222\\nengine=1111', sha256, first 16 hex characters.",
			got, want,
		)
	}
}

// TestExpectedFromDocumentReadsAppsAndDigests walks a real compiled config.
func TestExpectedFromDocumentReadsAppsAndDigests(t *testing.T) {
	// The shape the server renders: depth is the number of leading `v` characters,
	// and a list is a repeated node. `sha256` is a child of the artifact, not of the
	// app — reading it by absolute path would return the first artifact's digest for
	// every artifact.
	document := strings.Join([]string{
		"' vyzorix.v — device configuration",
		"v device d_1",
		"vv org org_1",
		"vv apps",
		"vvv app vfd",
		"vvvv dir /home/apps/vfd",
		"vvvv cmd /home/apps/vfd/engine/run.sh",
		"vvvv artifact engine",
		"vvvvv sha256 1111",
		"vvvv artifact console",
		"vvvvv sha256 2222",
		"vvv app wiki",
		"vvvv dir /home/apps/wiki",
		"vvvv artifact site",
		"vvvvv sha256 3333",
		"v version 7",
	}, "\n")

	got := ExpectedFromDocument(document)

	if !got.HasConfig {
		t.Fatal("a document that parses must yield an expectation")
	}
	if got.Version != 7 {
		t.Fatalf("version = %d, want 7", got.Version)
	}
	if len(got.AppDigests) != 2 {
		t.Fatalf("apps = %v, want 2 (vfd, wiki)", got.AppDigests)
	}
	if _, ok := got.AppDigests["vfd"]; !ok {
		t.Fatalf("vfd missing from %v", got.AppDigests)
	}
	// vfd's digest must be over both its artifacts, and equal the digest of the same
	// pair computed directly — this is the check that the artifact walk does not
	// flatten across siblings.
	wantVfd := AppStateDigest(map[string]string{"engine": "1111", "console": "2222"})
	if got.AppDigests["vfd"] != wantVfd {
		t.Fatalf("vfd digest = %q, want %q", got.AppDigests["vfd"], wantVfd)
	}
	// And wiki's must be over *its* artifact alone, not vfd's.
	wantWiki := AppStateDigest(map[string]string{"site": "3333"})
	if got.AppDigests["wiki"] != wantWiki {
		t.Fatalf("wiki digest = %q, want %q — the artifact walk crossed apps", got.AppDigests["wiki"], wantWiki)
	}
}

func TestExpectedFromUnparsableDocument(t *testing.T) {
	// A depth jump is a structural error libv rejects, so the document yields no
	// expectation rather than a wrong one.
	got := ExpectedFromDocument("vv apps")

	if got.HasConfig {
		t.Fatal("a document that will not parse must not yield an expectation")
	}
}

// Package deploystate answers one question continuously: is the device running
// what the server expects?
//
// ## Why this exists
//
// Until now the only record of "what is running" was the server's own
// `applied_at`, written from a `device_ready` frame. A refused apply was still
// recorded as applied, so the console showed a deploy that had never landed, and
// nothing noticed — the device held the truth and never said it.
//
// The device now reports what it installed on every telemetry heartbeat
// (`configVersion` and a per-app digest). This package compares that to the config
// the server has stored, so drift is visible on every heartbeat rather than only
// when an operator happens to deploy.
//
// ## The comparison is one pure function
//
// `Compare` takes what is expected and what was reported and returns a `Drift`.
// It reads nothing and writes nothing, so it is testable without a database, a
// socket or a device — the same discipline the deploy authorization predicate
// follows.
package deploystate

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	libv "github.com/VinnsEdesigner/libv/go"
)

// Expected is what the server's stored config says the device should be running.
type Expected struct {
	// AppDigests is app name → the digest of the artifacts that app declares.
	AppDigests map[string]string
	// Version is the config version the device should hold.
	Version int
	// HasConfig reports whether the server holds any config at all. A device with no
	// expected config cannot be drifted.
	HasConfig bool
}

// Reported is what the device says it is running, from its telemetry heartbeat.
type Reported struct {
	AppDigests map[string]string
	Version    int
	// ReportedState is false when the frame carried no state at all (an older app,
	// or a build with no app storage). A device that does not report state cannot be
	// judged, so it is neither healthy nor drifted — it is unknown, and the console
	// must say so rather than showing a green tick it did not earn.
	ReportedState bool
}

// Drift is the verdict. It is deliberately a value, not an error: drift is a fact
// about the world, not a failure of the comparison.
type Drift struct {
	// State is the verdict's kind.
	State DriftState
	// Detail names what differs, for a human: "app vfd: expected 3f2a…, running 9c11…".
	Detail string
}

// DriftState is how the device's reported state relates to what the server expects.
type DriftState string

const (
	// DriftNone: the device is running exactly what the server expects.
	DriftNone DriftState = "in_sync"
	// DriftUnknown: the device reported no state, or the server holds no config, so
	// no comparison is possible. Never rendered as healthy.
	DriftUnknown DriftState = "unknown"
	// DriftBehind: the device holds an older config version than the server expects.
	// The common case, and the one a failed apply produces.
	DriftBehind DriftState = "behind"
	// DriftAhead: the device holds a newer version than the server knows. A rolled
	// back server, or a device from a future deploy — worth surfacing either way.
	DriftAhead DriftState = "ahead"
	// DriftAppChanged: the version matches but an app's artifacts do not. This is the
	// case the version alone cannot see: an app edited or replaced in place.
	DriftAppChanged DriftState = "app_changed"
)

// Compare reports how the device's reported state relates to the expected one.
//
// The version is checked first, because it is the coarser signal: a device behind
// is behind whatever its app digests say, and reporting "app_changed" for a device
// that is simply on the previous version would be a less useful answer. Only when
// the versions agree do the digests matter, and then only for apps present on both
// sides — an app the server no longer declares is a removal, which is a version
// change and will show as one.
func Compare(expected Expected, reported Reported) Drift {
	if !expected.HasConfig {
		return Drift{State: DriftUnknown, Detail: "no config is stored for this device"}
	}

	if !reported.ReportedState {
		return Drift{State: DriftUnknown, Detail: "the device does not report installed state"}
	}

	switch {
	case reported.Version < expected.Version:
		return Drift{State: DriftBehind, Detail: versionDetail(expected.Version, reported.Version)}
	case reported.Version > expected.Version:
		return Drift{State: DriftAhead, Detail: versionDetail(expected.Version, reported.Version)}
	}

	// Same version: the digests are the only thing that can differ now. Sorted, so
	// the detail names the first difference deterministically rather than whichever
	// map iteration happened to come first.
	names := make([]string, 0, len(expected.AppDigests))
	for name := range expected.AppDigests {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		want := expected.AppDigests[name]
		got, present := reported.AppDigests[name]
		if !present {
			return Drift{
				State:  DriftAppChanged,
				Detail: "app " + name + " is not installed (expected " + short(want) + ")",
			}
		}
		if got != want {
			return Drift{
				State:  DriftAppChanged,
				Detail: "app " + name + ": expected " + short(want) + ", running " + short(got),
			}
		}
	}

	return Drift{State: DriftNone, Detail: "version " + strconv.Itoa(expected.Version) + " is running"}
}

func versionDetail(expected, reported int) string {
	return "expected version " + strconv.Itoa(expected) + ", device holds " + strconv.Itoa(reported)
}

// short renders a digest for a human: the first 8 characters, which is enough to
// tell two apart in a message and short enough to read.
func short(digest string) string {
	if len(digest) <= 8 {
		return digest
	}

	return digest[:8] + "…"
}

// AppStateDigest is the digest of one app's artifacts, and it is a **wire
// contract**: the device computes the same value in
// `InstalledStateStore.digest` (Vyzorix-android). Both sides must produce the same
// string for the same artifacts, or every device reads as drifted.
//
// The shape is deliberately trivial to reimplement: each artifact as
// `name=sha256`, sorted by artifact name, joined by newlines, hashed with SHA-256
// and truncated to 16 hex characters. The app name is not in the body — it is the
// key this digest is filed under.
//
// Sorted, because the artifacts arrive as a map and a digest must not depend on
// iteration order. Truncated, because it travels on every heartbeat and 64 bits
// over a handful of artifacts is ample to notice a change.
func AppStateDigest(artifacts map[string]string) string {
	if len(artifacts) == 0 {
		return ""
	}

	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(artifacts[name])
	}

	sum := sha256.Sum256([]byte(b.String()))

	return hex.EncodeToString(sum[:8])
}

// ExpectedFromDocument reads the config the server stores (the signed vyzorix.v
// text) and extracts what it says the device should be running: the version and,
// per app, the digest of its artifacts.
//
// It parses with the format's own parser, so it sees the same nodes the device
// does. A document that will not parse yields no expectation rather than an error:
// a stored config the server cannot read is a problem, but it is not *drift*, and
// reporting it as drift would send an operator looking at the wrong thing.
//
// ## Why the apps are read by walking, not by path
//
// `vyzorix.v` has no list syntax: a repeated node is the list, and `Find`
// flattens across repeated parents. Reading an app's field by absolute path would
// cross every app and mix them up — the device-side reader carries the same
// warning. So the app nodes are iterated and each is read within its own subtree.
func ExpectedFromDocument(document string) Expected {
	doc, err := libv.Parse(document)
	if err != nil {
		return Expected{}
	}

	version := 0
	if n := doc.Root.Find("version"); len(n) > 0 && len(n[0].Tokens) > 1 {
		version = atoi(n[0].Tokens[1])
	}

	digests := map[string]string{}
	// `device.apps` then the `app` children of each apps node — the same path the
	// device's `AppConfigReader` uses. Find flattens across repeated parents, so a
	// bare Find("app") would also collect an app node anywhere else in the document
	// (the schema has one place for them; this keeps it that way).
	var appNodes []*libv.Node
	for _, apps := range doc.Root.Find("device.apps") {
		for _, child := range apps.Children {
			if child.Key() == "app" {
				appNodes = append(appNodes, child)
			}
		}
	}

	for _, app := range appNodes {
		name := ""
		if len(app.Tokens) > 1 {
			name = app.Tokens[1]
		}
		if name == "" {
			continue
		}

		artifacts := map[string]string{}
		for _, artifact := range children(app, "artifact") {
			artifactName := ""
			if len(artifact.Tokens) > 1 {
				artifactName = artifact.Tokens[1]
			}
			if artifactName == "" {
				continue
			}
			if sha := artifactValue(artifact, "sha256"); sha != "" {
				artifacts[artifactName] = sha
			}
		}
		// One digest per app, over all its artifacts. An app with no artifacts gets
		// none, and the device reports none for it too.
		if d := AppStateDigest(artifacts); d != "" {
			digests[name] = d
		}
	}

	return Expected{Version: version, AppDigests: digests, HasConfig: true}
}

// children returns a node's direct children whose key is [key].
//
// Direct children, not `Find`: Find flattens, so a flattened `sha256` would return
// the first artifact's digest for every artifact in the app.
func children(node *libv.Node, key string) []*libv.Node {
	var out []*libv.Node
	for _, child := range node.Children {
		if child.Key() == key {
			out = append(out, child)
		}
	}

	return out
}

// artifactValue reads one key's value from an artifact subtree.
//
// The artifact's own fields are its direct children, so this scans them rather
// than using Find: Find flattens, and a flattened `sha256` would return the first
// artifact's digest for every artifact in the app.
func artifactValue(artifact *libv.Node, key string) string {
	for _, child := range artifact.Children {
		if child.Key() == key && len(child.Tokens) > 1 {
			return child.Tokens[1]
		}
	}

	return ""
}

// atoi parses a non-negative decimal, returning 0 for anything else. A version
// that will not parse reads as "no version", which Compare treats as behind — the
// safe direction for a device.
func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}

	return n
}

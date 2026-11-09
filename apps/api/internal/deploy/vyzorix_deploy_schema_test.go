package deploy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileManifestSchema compiles the published schema exactly as a consumer (the
// CLI) does. Compilation validates the document against the metaschema, so a
// structurally invalid schema fails here rather than at a client.
func compileManifestSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	raw, err := ManifestSchema(SchemaVersion)
	if err != nil {
		t.Fatalf("ManifestSchema(%q): %v", SchemaVersion, err)
	}

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("the published schema is not valid JSON: %v", err)
	}

	compiler := jsonschema.NewCompiler()

	// The published document carries no $id; a consumer keys it by the URL it was
	// fetched from. The test does the same, using a server-shaped URL.
	const url = "http://localhost:3099/v1/deploy/schema/1"
	if err := compiler.AddResource(url, doc); err != nil {
		t.Fatalf("AddResource: %v", err)
	}

	compiled, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("the published schema does not compile: %v", err)
	}

	return compiled
}

// TestManifestSchema_CompilesAndAcceptsAValidManifest is the guard the map-field
// bug slipped past: nothing else compiles the published artifact, so an invalid
// `additionalProperties` reached the CLI. Compiling it here fails the build that
// would ship it.
func TestManifestSchema_CompilesAndAcceptsAValidManifest(t *testing.T) {
	compiled := compileManifestSchema(t)

	var doc map[string]any
	if err := toml.Unmarshal([]byte(twoAppManifest), &doc); err != nil {
		t.Fatalf("fixture is not valid TOML: %v", err)
	}

	if err := compiled.Validate(doc); err != nil {
		t.Fatalf("the published schema refused a valid manifest: %v", err)
	}
}

func TestManifestSchema_RefusesAnUnknownKey(t *testing.T) {
	compiled := compileManifestSchema(t)

	var doc map[string]any
	if err := toml.Unmarshal([]byte("[device]\nid = \"d\"\norg = \"o\"\nbogus = 1\n"), &doc); err != nil {
		t.Fatalf("fixture is not valid TOML: %v", err)
	}

	if err := compiled.Validate(doc); err == nil {
		t.Fatal("the published schema accepted an unknown key")
	}
}

func TestManifestSchema_UnknownVersionIsRefused(t *testing.T) {
	if _, err := ManifestSchema("999"); err == nil {
		t.Fatal("ManifestSchema accepted an unknown version")
	}
}

// TestManifestSchema_EnforcesTheServersRules is the parity guard: the published
// schema is what the CLI pre-validates against (FR-2), so every rule the server
// enforces must also be in the schema, or the CLI accepts a manifest the server
// then refuses. Each case is a manifest the server refuses; the schema must too.
func TestManifestSchema_EnforcesTheServersRules(t *testing.T) {
	compiled := compileManifestSchema(t)

	cases := map[string]string{
		"no apps":           "[device]\nid = \"d\"\norg = \"o\"\n",
		"too many apps":     "[device]\nid = \"d\"\norg = \"o\"\n" + strings.Repeat("[[app]]\nname=\"a\"\ncmd=\"/home/apps/a/run.sh\"\n[[app.artifact]]\nname=\"x\"\nurl=\"https://e/x\"\nsha256=\""+sha+"\"\n", 33),
		"empty device id":   "[device]\nid = \"\"\norg = \"o\"\n" + validApp,
		"empty device org":  "[device]\nid = \"d\"\norg = \"\"\n" + validApp,
		"bad app name":      "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"Bad_Name\"\ncmd = \"/home/apps/x/run.sh\"\n" + validArtifact,
		"cmd not absolute":  "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"run.sh\"\n" + validArtifact,
		"port out of range": "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\nport = 70000\n" + validArtifact,
		"bad expose":        "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\nexpose = \"public\"\n" + validArtifact,
		"secret value":      "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[app.secrets]\nDATABASE_URL = \"postgres://x\"\n" + validArtifact,
		"bad artifact name": "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[[app.artifact]]\nname = \"Bad\"\nurl = \"https://e/x\"\nsha256 = \"" + sha + "\"\n",
		"non-https url":     "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[[app.artifact]]\nname = \"x\"\nurl = \"http://e/x\"\nsha256 = \"" + sha + "\"\n",
		"bad sha256":        "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[[app.artifact]]\nname = \"x\"\nurl = \"https://e/x\"\nsha256 = \"nope\"\n",
		"bad arch":          "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[[app.artifact]]\nname = \"x\"\nurl = \"https://e/x\"\nsha256 = \"" + sha + "\"\narch = \"armv7\"\n",
		"bad kind":          "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[[app.artifact]]\nname = \"x\"\nurl = \"https://e/x\"\nsha256 = \"" + sha + "\"\nkind = \"archive\"\n",
		"bad pwa display":   "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[app.pwa]\ndisplay = \"immersive\"\n" + validArtifact,
	}

	for name, manifest := range cases {
		var doc map[string]any
		if err := toml.Unmarshal([]byte(manifest), &doc); err != nil {
			t.Fatalf("%s: fixture is not valid TOML: %v", name, err)
		}

		if err := compiled.Validate(doc); err == nil {
			t.Errorf("%s: the published schema accepted a manifest the server refuses", name)
		}
	}
}

// TestManifestSchema_AcceptsTheEdgeCasesTheServerAccepts pins the other half:
// a rule that refuses too much is as wrong as one that refuses too little (§5.2).
func TestManifestSchema_AcceptsTheEdgeCasesTheServerAccepts(t *testing.T) {
	compiled := compileManifestSchema(t)

	cases := map[string]string{
		"an app with no artifacts": "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n",
		"expose lan with a port":   "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\nport = 9000\nexpose = \"lan\"\n" + validArtifact,
		"an empty secrets table":   "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n[app.secrets]\nDATABASE_URL = \"\"\nS3_SECRET_KEY = \"\"\n" + validArtifact,
		"the boolean pwa form":     "[device]\nid = \"d\"\norg = \"o\"\n[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\npwa = true\n" + validArtifact,
	}

	for name, manifest := range cases {
		var doc map[string]any
		if err := toml.Unmarshal([]byte(manifest), &doc); err != nil {
			t.Fatalf("%s: fixture is not valid TOML: %v", name, err)
		}

		if err := compiled.Validate(doc); err != nil {
			t.Errorf("%s: the published schema refused a valid manifest: %v", name, err)
		}
	}
}

const (
	sha           = "9f2c000000000000000000000000000000000000000000000000000000000000"
	validArtifact = "\n[[app.artifact]]\nname = \"engine\"\nurl = \"https://e/engine.tar.gz\"\nsha256 = \"" + sha + "\"\n"
	validApp      = "[[app]]\nname = \"a\"\ncmd = \"/home/apps/a/run.sh\"\n" + validArtifact
)

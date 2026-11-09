package deploy

import (
	_ "embed"
	"fmt"
)

// SchemaVersion is the published manifest schema's version. It is the {version}
// segment of GET /v1/deploy/schema/{version} (FR-2). It moves when the manifest
// shape does; the CLI pins it, so a manifest the CLI validated is the manifest
// the server accepts.
const SchemaVersion = "1"

// schemaJSON is the generated JSON Schema for vyzorix.toml, embedded so the
// server serves the exact bytes the CLI validates against. It is produced from
// tooling/schema/deploy.cue by tooling/codegen/build_deploy_schema.py, which the
// pre-commit drift gate re-runs.
//
//go:embed deploy.schema.json
var schemaJSON []byte

// ManifestSchema returns the published JSON Schema for the requested version.
// Only the current version is served: an unknown version is a refusal, not a
// silent fallback to a schema that would accept a different shape.
func ManifestSchema(version string) ([]byte, error) {
	if version != SchemaVersion {
		return nil, &UnknownSchemaVersionError{Requested: version, Current: SchemaVersion}
	}

	return schemaJSON, nil
}

// UnknownSchemaVersionError names both versions so a client can see which one to
// pin. It maps to 404: the request is well-formed, the version simply does not
// exist.
type UnknownSchemaVersionError struct {
	Requested string
	Current   string
}

// Error implements error.
func (e *UnknownSchemaVersionError) Error() string {
	return fmt.Sprintf("deploy: schema version %q does not exist; current version is %s", e.Requested, e.Current)
}

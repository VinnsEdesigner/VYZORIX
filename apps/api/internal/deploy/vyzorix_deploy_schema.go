package deploy

import (
	"errors"

	vyzorixspec "github.com/VinnsEdesigner/vyzorix-spec"
)

// SchemaVersion is the published manifest schema's version. It is the {version}
// segment of GET /v1/deploy/schema/{version} (FR-2). The spec module owns both
// the value and the schema, so the server and the CLI cannot disagree on which
// version a manifest was validated against.
const SchemaVersion = vyzorixspec.SchemaVersion

// ManifestSchema returns the published JSON Schema for the requested version.
// Only the current version is served: an unknown version is a refusal, not a
// silent fallback to a schema that would accept a different shape.
func ManifestSchema(version string) ([]byte, error) {
	schema, err := vyzorixspec.Schema(version)
	if err != nil {
		var unknown *vyzorixspec.UnknownSchemaVersionError
		if errors.As(err, &unknown) {
			return nil, &UnknownSchemaVersionError{Requested: unknown.Requested, Current: unknown.Current}
		}

		return nil, err
	}

	return schema, nil
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
	return "deploy: schema version " + e.Requested + " does not exist; current version is " + e.Current
}

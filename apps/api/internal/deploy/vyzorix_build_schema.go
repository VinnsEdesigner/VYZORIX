package deploy

import (
	vyzorixspec "github.com/VinnsEdesigner/vyzorix-spec"
)

// BuildSchemaVersion is the published build-recipe schema's version, the {version}
// segment of GET /v1/deploy/build-schema/{version}.
const BuildSchemaVersion = vyzorixspec.BuildSchemaVersion

// BuildRecipeSchema returns the published JSON Schema for the requested version.
// An unknown version is a refusal, not a fallback to a different shape.
func BuildRecipeSchema(version string) ([]byte, error) {
	return vyzorixspec.BuildSchema(version)
}

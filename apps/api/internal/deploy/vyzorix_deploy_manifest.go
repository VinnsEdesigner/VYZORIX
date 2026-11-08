// Package deploy compiles a vyzorix.toml manifest into a signed vyzorix.v
// config. It is the server half of VYZORIX_DEPLOY_CONTRACT.md: the operator
// declares, this package refuses what it must, signs what remains, and the
// device verifies it alone.
package deploy

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Bounds on the manifest's own fields. The device trusts this document, so the
// limits are enforced here rather than left to whatever the compiler produces.
const (
	maxDeviceIDLen   = 128
	maxAppNameLen    = 32
	maxArtifactName  = 64
	sha256HexLen     = 64
	artifactNameRe   = `^[a-z0-9][a-z0-9-]*$`
	appNameRe        = `^[a-z0-9][a-z0-9-]*$`
	maxAppsPerDevice = 32
)

// ExposeMode is how an app's port is reachable. `cloud` is refused in v1
// (contract §10); it is modelled so the refusal names the value.
type ExposeMode string

const (
	// ExposeLoopback binds 127.0.0.1 only. The default, and the only mode that
	// needs no token.
	ExposeLoopback ExposeMode = "loopback"
	// ExposeLAN binds 0.0.0.0 and requires a token at the supervisor.
	ExposeLAN ExposeMode = "lan"
	// ExposeCloud is refused in v1: it needs the deferred wildcard-DNS/TLS proxy.
	ExposeCloud ExposeMode = "cloud"
)

// ArtifactKind distinguishes content that is merely unpacked from content that
// is served as a webroot.
type ArtifactKind string

const (
	// ArtifactDir is unpacked into the app directory. The default.
	ArtifactDir ArtifactKind = "dir"
	// ArtifactWebroot is unpacked and served by the app.
	ArtifactWebroot ArtifactKind = "webroot"
)

// DisplayMode is the PWA display value. Anything else is a hard error: an
// invalid value silently degrades to `browser` and the operator never learns
// why the app is not installable (contract §6).
type DisplayMode string

const (
	// DisplayStandalone is the generated default.
	DisplayStandalone DisplayMode = "standalone"
	// DisplayFullscreen hides all browser chrome.
	DisplayFullscreen DisplayMode = "fullscreen"
	// DisplayMinimalUI keeps a minimal set of controls.
	DisplayMinimalUI DisplayMode = "minimal-ui"
	// DisplayBrowser is a normal browser tab.
	DisplayBrowser DisplayMode = "browser"
)

// Icon is one entry of a generated web manifest's icon list. Icons are never
// generated; an operator supplies them or the list is empty (contract §6).
type Icon struct {
	Src   string `toml:"src"`
	Sizes string `toml:"sizes"`
	Type  string `toml:"type"`
}

// PWA is the per-app web-manifest declaration. In the manifest it is either a
// boolean or a table; both arrive here as a non-nil PWA, with Enabled carrying
// the boolean form.
type PWA struct {
	Name            string `toml:"name"`
	ShortName       string `toml:"short_name"`
	Description     string `toml:"description"`
	ThemeColor      string `toml:"theme_color"`
	BackgroundColor string `toml:"background_color"`
	Display         string `toml:"display"`
	Orientation     string `toml:"orientation"`
	StartURL        string `toml:"start_url"`
	Icons           []Icon `toml:"icons"`
	Enabled         bool   `toml:"-"`
}

// ArtifactSpec is one artifact reference. A URL is literal and the digest pins
// its bytes, so the transport can be untrusted (contract §5.3).
type ArtifactSpec struct {
	Name   string       `toml:"name"`
	URL    string       `toml:"url"`
	SHA256 string       `toml:"sha256"`
	Arch   string       `toml:"arch"`
	Kind   ArtifactKind `toml:"kind"`
	Size   int64        `toml:"size"`
}

// AppSpec is one [[app]] block.
//
// Autostart carries the contract's `true` default (§4.1). Go zero-values a bool
// to false, so a *bool is used to tell "absent" from "explicitly false", and
// ParseManifest resolves it before validation.
type AppSpec struct {
	Env        map[string]string `toml:"env"`
	SecretsT   map[string]string `toml:"secrets"`
	PWA        *PWA              `toml:"pwa"`
	AutostartT *bool             `toml:"autostart"`
	Name       string            `toml:"name"`
	Cmd        string            `toml:"cmd"`
	Expose     ExposeMode        `toml:"expose"`
	Artifacts  []ArtifactSpec    `toml:"artifact"`
	Secrets    []string          `toml:"-"`
	Port       int               `toml:"port"`
	Autostart  bool              `toml:"-"`
	Purge      bool              `toml:"purge"`
}

// DeviceSpec is the [device] block. Only id, org and an optional display name
// are the operator's; everything else is server-injected (contract §5.1).
type DeviceSpec struct {
	ID   string `toml:"id"`
	Org  string `toml:"org"`
	Name string `toml:"name"`
}

// Manifest is a parsed, validated vyzorix.toml.
type Manifest struct {
	Device DeviceSpec `toml:"device"`
	Apps   []AppSpec  `toml:"app"`
}

// ManifestError is a refusal with the reason and the field that caused it. The
// CLI prints Message verbatim, so it must read as a sentence to an operator.
type ManifestError struct {
	Field   string
	Message string
}

// Error implements error.
func (e *ManifestError) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

var (
	appNamePattern      = regexp.MustCompile(appNameRe)
	artifactNamePattern = regexp.MustCompile(artifactNameRe)
	sha256Pattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
	archPattern         = regexp.MustCompile(`^(arm64-v8a|armeabi-v7a|x86_64)$`)
)

// ParseManifest decodes a vyzorix.toml. An unknown key is a hard error: a typo'd
// setting must not silently do nothing (contract §4.1).
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest

	dec := toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields()
	dec.EnableUnmarshalerInterface()

	if err := dec.Decode(&m); err != nil {
		return nil, unknownKeyError(err)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// unknownKeyError turns the decoder's strict-mode error into one that names each
// unknown key. The raw message ("fields in the document are missing in the
// target struct") does not say which field, and a refusal an operator cannot
// locate is one they will work around.
func unknownKeyError(err error) error {
	var strict *toml.StrictMissingError
	if !errors.As(err, &strict) {
		return &ManifestError{Message: err.Error()}
	}

	names := make([]string, 0, len(strict.Errors))

	for i := range strict.Errors {
		names = append(names, strings.Join(strict.Errors[i].Key(), "."))
	}

	sortStrings(names)

	return &ManifestError{
		Message: "unknown key(s): " + strings.Join(names, ", "),
	}
}

// Validate refuses a manifest the compiler must not accept. It names the field,
// because a refusal an operator cannot locate is one they will work around.
func (m *Manifest) Validate() error {
	if err := m.Device.validate(); err != nil {
		return err
	}

	if len(m.Apps) == 0 {
		return &ManifestError{Field: "app", Message: "at least one [[app]] is required"}
	}

	if len(m.Apps) > maxAppsPerDevice {
		return &ManifestError{
			Field:   "app",
			Message: fmt.Sprintf("at most %d apps per device, got %d", maxAppsPerDevice, len(m.Apps)),
		}
	}

	seen := make(map[string]struct{}, len(m.Apps))

	for i := range m.Apps {
		app := &m.Apps[i]
		if err := app.validate(); err != nil {
			return err
		}

		if _, dup := seen[app.Name]; dup {
			return &ManifestError{Field: "app.name", Message: fmt.Sprintf("duplicate app %q", app.Name)}
		}

		seen[app.Name] = struct{}{}
	}

	return nil
}

// validate checks the [device] block. The id and org must be present; the
// server binds them to the caller and the file may not name an authoriser.
func (d *DeviceSpec) validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return &ManifestError{Field: "device.id", Message: "required"}
	}

	if len(d.ID) > maxDeviceIDLen {
		return &ManifestError{Field: "device.id", Message: "too long"}
	}

	if strings.TrimSpace(d.Org) == "" {
		return &ManifestError{Field: "device.org", Message: "required"}
	}

	return nil
}

// validate checks one [[app]] block, including its artifacts and PWA table.
func (a *AppSpec) validate() error {
	if a.Name == "" {
		return &ManifestError{Field: "app.name", Message: "required"}
	}

	if !appNamePattern.MatchString(a.Name) || len(a.Name) > maxAppNameLen {
		return &ManifestError{
			Field:   "app.name",
			Message: fmt.Sprintf("%q must match %s and be at most %d characters", a.Name, appNameRe, maxAppNameLen),
		}
	}

	if a.Cmd == "" {
		return &ManifestError{Field: "app." + a.Name + ".cmd", Message: "required"}
	}

	if err := validateCmd(a.Name, a.Cmd); err != nil {
		return err
	}

	if err := a.validateExpose(); err != nil {
		return err
	}

	if err := a.validateArtifacts(); err != nil {
		return err
	}

	if err := a.validateSecrets(); err != nil {
		return err
	}

	a.resolveDefaults()

	return a.validatePWA()
}

// resolveDefaults applies the contract's defaults that a Go zero value cannot
// express: autostart is true unless the file says otherwise.
func (a *AppSpec) resolveDefaults() {
	if a.AutostartT == nil {
		a.Autostart = true
	} else {
		a.Autostart = *a.AutostartT
	}
}

// validateExpose enforces the port rule and refuses cloud in v1.
func (a *AppSpec) validateExpose() error {
	switch a.Expose {
	case "":
		a.Expose = ExposeLoopback
	case ExposeLoopback, ExposeLAN:
	case ExposeCloud:
		return &ManifestError{
			Field:   "app." + a.Name + ".expose",
			Message: "cloud is not supported in v1; use loopback or lan",
		}
	default:
		return &ManifestError{
			Field:   "app." + a.Name + ".expose",
			Message: fmt.Sprintf("%q must be loopback, lan or cloud", a.Expose),
		}
	}

	if a.Expose != ExposeLoopback && a.Port == 0 {
		return &ManifestError{
			Field:   "app." + a.Name + ".port",
			Message: fmt.Sprintf("required when expose = %q", a.Expose),
		}
	}

	if a.Port < 0 || a.Port > 65535 {
		return &ManifestError{Field: "app." + a.Name + ".port", Message: "must be 1-65535"}
	}

	return nil
}

// validateArtifacts enforces the mandatory digest and per-arch uniqueness.
func (a *AppSpec) validateArtifacts() error {
	seen := make(map[string]struct{}, len(a.Artifacts))

	for i := range a.Artifacts {
		art := &a.Artifacts[i]

		if err := art.validate(a.Name); err != nil {
			return err
		}

		key := art.Name + "\x00" + art.Arch
		if _, dup := seen[key]; dup {
			return &ManifestError{
				Field:   "app." + a.Name + ".artifact.name",
				Message: fmt.Sprintf("duplicate artifact %q for arch %q", art.Name, art.Arch),
			}
		}

		seen[key] = struct{}{}
	}

	return nil
}

// validate checks one artifact. The digest is mandatory: without it the URL is
// trusted because it was signed, which is only true if the host is honest.
func (a *ArtifactSpec) validate(app string) error {
	field := "app." + app + ".artifact"

	if a.Name == "" {
		return &ManifestError{Field: field + ".name", Message: "required"}
	}

	if !artifactNamePattern.MatchString(a.Name) || len(a.Name) > maxArtifactName {
		return &ManifestError{
			Field:   field + ".name",
			Message: fmt.Sprintf("%q must match %s and be at most %d characters", a.Name, artifactNameRe, maxArtifactName),
		}
	}

	if a.URL == "" {
		return &ManifestError{Field: field + "." + a.Name + ".url", Message: "required"}
	}

	if !strings.HasPrefix(a.URL, "https://") {
		return &ManifestError{Field: field + "." + a.Name + ".url", Message: "must be https"}
	}

	if a.SHA256 == "" {
		return &ManifestError{
			Field:   field + "." + a.Name + ".sha256",
			Message: "required; every artifact must be pinned by digest",
		}
	}

	if !sha256Pattern.MatchString(a.SHA256) {
		return &ManifestError{
			Field:   field + "." + a.Name + ".sha256",
			Message: fmt.Sprintf("must be %d lowercase hex characters", sha256HexLen),
		}
	}

	if a.Arch != "" && !archPattern.MatchString(a.Arch) {
		return &ManifestError{
			Field:   field + "." + a.Name + ".arch",
			Message: "must be arm64-v8a, armeabi-v7a or x86_64",
		}
	}

	switch a.Kind {
	case "":
		a.Kind = ArtifactDir
	case ArtifactDir, ArtifactWebroot:
	default:
		return &ManifestError{
			Field:   field + "." + a.Name + ".kind",
			Message: fmt.Sprintf("%q must be dir or webroot", a.Kind),
		}
	}

	return nil
}

// validateSecrets normalises the [app.secrets] table into names. The values
// must be empty: the table's keys are names and a value here is a mistake.
func (a *AppSpec) validateSecrets() error {
	if len(a.SecretsT) == 0 {
		return nil
	}

	a.Secrets = make([]string, 0, len(a.SecretsT))

	for key, value := range a.SecretsT {
		if key == "" {
			return &ManifestError{Field: "app." + a.Name + ".secrets", Message: "empty secret name"}
		}

		if value != "" {
			return &ManifestError{
				Field:   "app." + a.Name + ".secrets." + key,
				Message: "secrets carry names only; the value must be empty",
			}
		}

		a.Secrets = append(a.Secrets, key)
	}

	sortStrings(a.Secrets)

	return nil
}

// validatePWA refuses an invalid display value. The absent and false cases are
// both "no manifest"; they are distinguished at render time, not here.
func (a *AppSpec) validatePWA() error {
	if a.PWA == nil || !a.PWA.Enabled {
		return nil
	}

	switch DisplayMode(a.PWA.Display) {
	case "":
		a.PWA.Display = string(DisplayStandalone)
	case DisplayStandalone, DisplayFullscreen, DisplayMinimalUI, DisplayBrowser:
	default:
		return &ManifestError{
			Field:   "app." + a.Name + ".pwa.display",
			Message: fmt.Sprintf("%q must be standalone, fullscreen, minimal-ui or browser", a.PWA.Display),
		}
	}

	return nil
}

// UnmarshalTOML accepts `pwa = true`, `pwa = false`, or a `[app.pwa]` table.
// The union is why this is custom: TOML has no sum type, and a bool field and a
// table field cannot share a name.
func (p *PWA) UnmarshalTOML(data []byte) error {
	trimmed := strings.TrimSpace(string(data))

	if trimmed == "true" || trimmed == "false" {
		p.Enabled = trimmed == "true"

		return nil
	}

	// A table. Decoded with the document's strictness, so a typo in [app.pwa]
	// is refused rather than ignored.
	type pwaTable PWA

	var table pwaTable

	dec := toml.NewDecoder(strings.NewReader(trimmed)).DisallowUnknownFields()
	if err := dec.Decode(&table); err != nil {
		return fmt.Errorf("app.pwa: %w", err)
	}

	table.Enabled = true
	*p = PWA(table)

	return nil
}

// sortStrings sorts in place with an insertion sort. It keeps this file free of
// an import for one call, and it is stable, which determinism wants.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// errNoSigner is returned when a compile is asked to sign with no signer.
var errNoSigner = errors.New("deploy: no signer configured")

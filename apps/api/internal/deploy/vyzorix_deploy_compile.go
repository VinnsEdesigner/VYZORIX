package deploy

import (
	"fmt"
	"path"
	"strings"
)

// The compiler: manifest -> config. It injects the server-owned fields and
// produces the field set the renderer writes. It neither signs nor scans for
// secrets; CompileAndSign does both around it.

// Injection is the server-owned data a compile adds. None of it may come from
// the file: a config that names its own authoriser proves nothing.
type Injection struct {
	MemberID         string
	DeviceID         string
	OrgID            string
	Name             string
	StreamPath       string
	Version          int
	TierCap          int
	HeartbeatSeconds int
	TerminalEnabled  bool
	USBEnabled       bool
}

// Compile turns a manifest into a config. The manifest must already have passed
// ParseManifest.
//
// The injection is checked for the values that make a config meaningful. A caller that
// forgot to set the version would render `version 0`, and the device would treat the
// document as older than anything it holds and ignore it — a silent no-op rather than an
// error. The device, org and member are the same kind of required server-owned fact.
func Compile(m *Manifest, in Injection) (*Config, error) {
	if err := validateInjection(in); err != nil {
		return nil, err
	}

	cfg := &Config{
		DeviceID:         in.DeviceID,
		OrgID:            in.OrgID,
		Name:             in.Name,
		MemberID:         in.MemberID,
		StreamPath:       in.StreamPath,
		Version:          in.Version,
		TierCap:          in.TierCap,
		HeartbeatSeconds: in.HeartbeatSeconds,
		TerminalEnabled:  in.TerminalEnabled,
		USBEnabled:       in.USBEnabled,
		Apps:             make([]AppConfig, 0, len(m.Apps)),
	}

	for i := range m.Apps {
		app, err := compileApp(&m.Apps[i])
		if err != nil {
			return nil, err
		}

		cfg.Apps = append(cfg.Apps, *app)
	}

	return cfg, nil
}

// validateInjection refuses an injection missing a server-owned value a config needs.
// The version is the important one: version 0 is older than any real deploy, so a device
// would keep what it has and the deploy would look like it worked.
func validateInjection(in Injection) error {
	switch {
	case in.DeviceID == "":
		return &ManifestError{Field: "device.id", Message: "the server did not supply a target device"}
	case in.OrgID == "":
		return &ManifestError{Field: "device.org", Message: "the server did not supply an organisation"}
	case in.MemberID == "":
		return &ManifestError{Field: "device.member", Message: "the server did not supply the authorising member"}
	case in.Version <= 0:
		return &ManifestError{Field: "version", Message: "the server did not supply a positive version"}
	}

	return nil
}

// compileApp builds one AppConfig. The digest rule is re-checked here so a
// caller that builds a Manifest without ParseManifest is still covered.
func compileApp(spec *AppSpec) (*AppConfig, error) {
	app := &AppConfig{
		Name:      spec.Name,
		Cmd:       spec.Cmd,
		Port:      spec.Port,
		Autostart: spec.Autostart,
		Expose:    spec.Expose,
		Purge:     spec.Purge,
		Env:       spec.Env,
		Secrets:   spec.Secrets,
		PWA:       spec.PWA,
		Artifacts: make([]ArtifactConfig, 0, len(spec.Artifacts)),
	}

	for i := range spec.Artifacts {
		art := &spec.Artifacts[i]
		if art.SHA256 == "" {
			return nil, &ManifestError{
				Field:   "app." + spec.Name + ".artifact." + art.Name,
				Message: "has no sha256; every artifact must be pinned by digest",
			}
		}

		app.Artifacts = append(app.Artifacts, ArtifactConfig{
			Name:   art.Name,
			URL:    art.URL,
			SHA256: art.SHA256,
			Arch:   art.Arch,
			Kind:   art.Kind,
			Size:   art.Size,
		})
	}

	return app, nil
}

// cmdMetacharacters are the shell characters the cmd rule refuses. The command
// runs under fork/exec with no shell, so this is belt-and-braces.
const cmdMetacharacters = ";|&$`><(){}*?~!\n\r"

// validateCmd applies the cmd rule: absolute, inside the app's own
// directory, free of shell metacharacters. The symlink check is the device's.
func validateCmd(appName, cmd string) error {
	field := "app." + appName + ".cmd"

	if !strings.HasPrefix(cmd, "/") {
		return &ManifestError{Field: field, Message: "must be an absolute path"}
	}

	if strings.ContainsAny(cmd, cmdMetacharacters) {
		return &ManifestError{Field: field, Message: "must not contain shell metacharacters"}
	}

	appDir := "/home/apps/" + appName + "/"
	if !strings.HasPrefix(cmd, appDir) {
		return &ManifestError{
			Field:   field,
			Message: fmt.Sprintf("must resolve inside %s; got %q", appDir, cmd),
		}
	}

	// A `..` segment could escape the directory without changing the prefix
	// check, so the cleaned path must still be inside it.
	cleaned := path.Clean(cmd)
	if !strings.HasPrefix(cleaned, appDir) {
		return &ManifestError{
			Field:   field,
			Message: fmt.Sprintf("escapes the app directory; got %q", cleaned),
		}
	}

	return nil
}

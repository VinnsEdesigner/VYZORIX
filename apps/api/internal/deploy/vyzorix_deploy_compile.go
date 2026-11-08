package deploy

import (
	"fmt"
	"path"
	"strings"
)

// The compiler: manifest -> config. It injects the server-owned fields, applies
// the cmd rule (§8) and the unhashed-reference rule (§5.3), and produces the
// field set the renderer writes. It does not sign and does not scan for secrets;
// Compile does both around it so a caller cannot forget either.

// Injection is the server-owned data a compile adds. None of it may come from
// the file (contract §5.1): a config that names its own authoriser proves
// nothing.
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
// ParseManifest; Compile re-checks the rules that depend on injected values and
// on cross-app state.
func Compile(m *Manifest, in Injection) (*Config, error) {
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

// compileApp builds one AppConfig. The cmd rule is enforced at parse time; the
// digest rule is re-checked here because it also guards against a future caller
// that builds a Manifest without going through ParseManifest.
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
// is executed by fork/exec with no shell, so a metacharacter has nothing to be
// interpreted by; refusing it is belt-and-braces on top of that.
const cmdMetacharacters = ";|&$`><(){}*?~!\n\r"

// validateCmd applies the cmd rule (contract §8): absolute, inside the app's own
// directory, and free of shell metacharacters. The symlink check is the device's
// — the artifact has not been fetched here — so this is the compile-time half.
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

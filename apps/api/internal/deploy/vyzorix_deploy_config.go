package deploy

// The compiled config: what is signed, delivered and stored. It is the
// vyzorix.v shape of REGISTRATION_CONTRACT.md §6, extended with the apps the
// manifest declares (VYZORIX_DEPLOY_CONTRACT.md §5.1). The .toml never leaves
// the operator's machine; this does.

// Config is the whole signed document's field set.
type Config struct {
	MemberID         string
	DeviceID         string
	OrgID            string
	Name             string
	StreamPath       string
	Apps             []AppConfig
	Version          int
	TierCap          int
	HeartbeatSeconds int
	TerminalEnabled  bool
	USBEnabled       bool
}

// AppConfig is one app in the compiled config.
type AppConfig struct {
	Env       map[string]string
	PWA       *PWA
	Name      string
	Cmd       string
	Expose    ExposeMode
	Artifacts []ArtifactConfig
	Secrets   []string
	Port      int
	Autostart bool
	Purge     bool
}

// ArtifactConfig is one artifact reference in the compiled config.
type ArtifactConfig struct {
	Name   string
	URL    string
	SHA256 string
	Arch   string
	Kind   ArtifactKind
	Size   int64
}

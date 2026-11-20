package provisioning

import (
	"errors"
	"fmt"
	"strings"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/alert"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	toml "github.com/pelletier/go-toml/v2"
)

// ResourceKind is what a provisioned file declares. It is decided by the directory the
// file lives in, not by a field inside it: the layout is the contract.
//
// The type lives in the domain package, because the resource row carries it and the storage
// layer must not import application code. These aliases keep the app-side names unchanged.
type ResourceKind = domain.ResourceKind

const (
	// KindDevice is a per-device deploy manifest under devices/<device-id>/.
	KindDevice = domain.KindDevice
	// KindAlerts is an organisation's alert rules, orgs/<org>/alerts.toml.
	KindAlerts = domain.KindAlerts
	// KindContactPoints is an organisation's notification targets.
	KindContactPoints = domain.KindContactPoints
	// KindAnnotations is an organisation's annotations.
	KindAnnotations = domain.KindAnnotations
	// KindGroups is an organisation's device groups and their membership.
	KindGroups = domain.KindGroups
)

// OrgFileNames maps the file name to the kind it declares, inside an orgs/<org>/ directory.
var OrgFileNames = map[string]ResourceKind{
	"alerts.toml":         KindAlerts,
	"contact-points.toml": KindContactPoints,
	"annotations.toml":    KindAnnotations,
	"groups.toml":         KindGroups,
}

// ErrUnknownKind is returned for a file under a known anchor whose name declares no kind.
var ErrUnknownKind = errors.New("file declares no known provisioning kind")

// ClassifyPath decides a resource's kind from its repository path.
//
// The two anchors are devices/ and orgs/. A file under neither is an error naming the
// file, not a silent skip: a user who put a manifest in the wrong place should be told,
// not left wondering why nothing happened. The device kind is returned with the device id
// and the org kind with the organisation name, because both are what the applier keys on.
func ClassifyPath(rel string) (kind ResourceKind, target string, err error) {
	parts := strings.Split(rel, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("%s is not under devices/<device-id>/ or orgs/<org>/: %w", rel, ErrUnknownKind)
	}

	switch parts[0] {
	case "devices":
		return KindDevice, parts[1], nil
	case "orgs":
		fileKind, ok := OrgFileNames[parts[len(parts)-1]]
		if !ok {
			return "", "", fmt.Errorf("%s: %s is not a known org file: %w", rel, parts[len(parts)-1], ErrUnknownKind)
		}

		return fileKind, parts[1], nil
	default:
		return "", "", fmt.Errorf("%s is not under devices/<device-id>/ or orgs/<org>/: %w", rel, ErrUnknownKind)
	}
}

// AlertRule is one [[rule]] in an org's alerts.toml.
//
// The field names mirror the alert domain's own enums, so a file cannot name a metric or
// condition the evaluator does not implement. webhook_url is deliberately absent: a
// secret-bearing URL belongs in a contact point, which stores it protected.
type AlertRule struct {
	Name      string  `toml:"name"`
	Metric    string  `toml:"metric"`
	Condition string  `toml:"condition"`
	For       string  `toml:"for"`
	Notify    string  `toml:"notify"`
	Threshold float64 `toml:"threshold"`
	Enabled   bool    `toml:"enabled"`
}

// AlertFile is an org's alerts.toml.
type AlertFile struct {
	Rules []AlertRule `toml:"rule"`
}

// ParseAlertFile decodes and validates an alerts.toml. An unknown key is a hard error, so a
// typo'd setting cannot silently do nothing.
func ParseAlertFile(data []byte) (*AlertFile, error) {
	var f AlertFile

	dec := toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, err
	}

	if err := f.Validate(); err != nil {
		return nil, err
	}

	return &f, nil
}

// Validate refuses a file the applier must not accept, naming the rule and field.
//
// The metric and condition are checked against the alert domain's own enums: a file that
// names a metric the evaluator does not implement would otherwise save a rule that never
// fires, which looks configured and is not.
func (f *AlertFile) Validate() error {
	seen := make(map[string]struct{}, len(f.Rules))

	for i := range f.Rules {
		r := &f.Rules[i]

		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("rule %d: name is required", i+1)
		}
		if _, dup := seen[r.Name]; dup {
			return fmt.Errorf("rule %q is declared twice", r.Name)
		}
		seen[r.Name] = struct{}{}

		if !alert.Metric(r.Metric).Valid() {
			return fmt.Errorf("rule %q: %q is not a known metric", r.Name, r.Metric)
		}
		if !alert.Condition(r.Condition).Valid() {
			return fmt.Errorf("rule %q: %q is not a known condition", r.Name, r.Condition)
		}
	}

	return nil
}

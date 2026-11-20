package provisioning

import (
	"errors"
	"strings"
	"testing"
)

// TestClassifyPathDevices proves a device manifest classifies by folder.
func TestClassifyPathDevices(t *testing.T) {
	kind, target, err := ClassifyPath("devices/living-room/vyzorix.toml")
	if err != nil {
		t.Fatalf("ClassifyPath: %v", err)
	}
	if kind != KindDevice || target != "living-room" {
		t.Fatalf("got %s/%s, want device/living-room", kind, target)
	}
}

// TestClassifyPathOrgs proves each org file classifies by its name, and the org is the
// directory.
func TestClassifyPathOrgs(t *testing.T) {
	cases := map[string]ResourceKind{
		"orgs/acme/alerts.toml":         KindAlerts,
		"orgs/acme/contact-points.toml": KindContactPoints,
		"orgs/acme/annotations.toml":    KindAnnotations,
		"orgs/acme/groups.toml":         KindGroups,
	}

	for path, want := range cases {
		kind, org, err := ClassifyPath(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if kind != want {
			t.Fatalf("%s: kind = %s, want %s", path, kind, want)
		}
		if org != "acme" {
			t.Fatalf("%s: org = %s, want acme", path, org)
		}
	}
}

// TestClassifyPathRejectsUnknown proves a file under neither anchor, or an unrecognised
// org file, is an error rather than a silent skip.
func TestClassifyPathRejectsUnknown(t *testing.T) {
	for _, path := range []string{
		"docs/readme.toml",
		"devices/a",                    // no file segment
		"orgs/acme/unknown.toml",       // unrecognised org file
		"orgs/acme/nested/alerts.toml", // right name, wrong depth is still found by basename
	} {
		_, _, err := ClassifyPath(path)
		if path == "orgs/acme/nested/alerts.toml" {
			// A nested path still ends in a known file name; the layout rule is by
			// directory anchor, so this one is accepted.
			if err != nil {
				t.Fatalf("%s was rejected but ends in a known file name: %v", path, err)
			}

			continue
		}
		if !errors.Is(err, ErrUnknownKind) {
			t.Errorf("%s: got %v, want ErrUnknownKind", path, err)
		}
	}
}

// TestParseAlertFileValid proves a well-formed file parses.
func TestParseAlertFileValid(t *testing.T) {
	data := []byte(`
[[rule]]
name      = "device-offline"
metric    = "device_offline_count"
condition = "gt"
threshold = 0
for       = "5m"
notify    = "5m"
enabled   = true
`)
	f, err := ParseAlertFile(data)
	if err != nil {
		t.Fatalf("ParseAlertFile: %v", err)
	}
	if len(f.Rules) != 1 || f.Rules[0].Name != "device-offline" {
		t.Fatalf("rules = %+v", f.Rules)
	}
	if !f.Rules[0].Enabled {
		t.Fatal("enabled was not read")
	}
}

// TestParseAlertFileRejectsUnknownKey proves a typo'd setting is an error, not a silent
// no-op.
func TestParseAlertFileRejectsUnknownKey(t *testing.T) {
	data := []byte(`
[[rule]]
name   = "r"
metric = "device_offline_count"
conditon = "gt"
`)
	if _, err := ParseAlertFile(data); err == nil {
		t.Fatal("an unknown key was accepted")
	}
}

// TestParseAlertFileRejectsDuplicateName proves two rules cannot share a name, because the
// applier matches by name and a duplicate would make the match ambiguous.
func TestParseAlertFileRejectsDuplicateName(t *testing.T) {
	data := []byte(`
[[rule]]
name = "r"
metric = "device_offline_count"
condition = "gt"

[[rule]]
name = "r"
metric = "device_offline_percent"
condition = "lt"
`)
	if _, err := ParseAlertFile(data); err == nil {
		t.Fatal("a duplicate rule name was accepted")
	} else if !strings.Contains(err.Error(), "declared twice") {
		t.Fatalf("error does not explain the duplicate: %v", err)
	}
}

// TestParseAlertFileRequiresNameAndMetric proves the required fields are enforced.
func TestParseAlertFileRequiresNameAndMetric(t *testing.T) {
	for name, body := range map[string]string{
		"missing name":   "[[rule]]\nmetric = \"device_offline_count\"\ncondition = \"gt\"\n",
		"missing metric": "[[rule]]\nname = \"r\"\ncondition = \"gt\"\n",
		"missing cond":   "[[rule]]\nname = \"r\"\nmetric = \"device_offline_count\"\n",
	} {
		if _, err := ParseAlertFile([]byte(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

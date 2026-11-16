package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestIsValid(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"", true}, // empty clears the preference
		{"dark", true},
		{"light", true},
		{"rose-500", true},
		{"burgundy", true},
		{"matrix", true},
		{"tron", true},
		{"synthwave", true},
		{"zen", true},
		{"system", true},
		{"nope", false},
		{"DARK", false}, // ids are case-sensitive
	}
	for _, c := range cases {
		if got := IsValid(c.id); got != c.want {
			t.Errorf("IsValid(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestIDsMatchFrontendRegistry(t *testing.T) {
	// The frontend registry is the source of truth for the catalogue; this
	// asserts the Go allowlist agrees, so adding a theme to one side without the
	// other fails here rather than silently rejecting a valid id at runtime.
	path := filepath.Join("..", "..", "..", "..", "VyzoriX_web", "src", "test", "lib", "theme-test-helpers.ts")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("frontend theme id list not readable at %s: %v", path, err)
	}

	re := regexp.MustCompile(`(?s)THEMES_UNDER_TEST\s*=\s*\[(.*?)\]`)
	m := re.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("could not find THEMES_UNDER_TEST in %s", path)
	}
	var fe []string
	for _, line := range strings.Split(string(m[1]), "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "',")
		if line != "" {
			fe = append(fe, line)
		}
	}

	be := IDs()
	sort.Strings(fe)
	sort.Strings(be)
	if strings.Join(fe, ",") != strings.Join(be, ",") {
		t.Fatalf("theme id drift:\n frontend: %v\n backend:  %v", fe, be)
	}
}

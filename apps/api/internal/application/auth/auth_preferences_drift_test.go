package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestPreferenceKeysMatchFrontendSyncList asserts the server allowlist and the
// client's synced-key list agree.
//
// The client sends snake_case keys in a flat map and the server stores them, so
// the two lists are one contract split across languages. The client list is the
// source of truth (the client is what writes); this reads it and compares. Adding
// a synced preference on one side without the other fails here rather than
// silently rejecting a valid key (or accepting a dead one) at runtime.
func TestPreferenceKeysMatchFrontendSyncList(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "VyzoriX_web", "src", "test", "lib", "preferences-test-helpers.ts")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("frontend preference key list not readable at %s: %v", path, err)
	}

	re := regexp.MustCompile(`(?s)PREFERENCE_KEYS_UNDER_TEST\s*=\s*\[(.*?)\]`)
	m := re.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("could not find PREFERENCE_KEYS_UNDER_TEST in %s", path)
	}

	var frontend []string
	for _, line := range strings.Split(string(m[1]), "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "',")
		if line != "" {
			frontend = append(frontend, line)
		}
	}

	// Compare the client's synced keys against the schema. The schema also holds
	// keys the client does not sync yet (theme, timezone, language and the
	// notification prefs), so the assertion is one-directional: every client key
	// must be known to the server. The reverse would flag those legitimately
	// server-only keys.
	sort.Strings(frontend)
	var unknown []string
	for _, key := range frontend {
		if !IsKnownPreferenceKey(key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		t.Fatalf("client syncs preference keys the server does not accept: %v", unknown)
	}

	// And the reverse guard for the keys the client owns: if a key is dropped
	// from the client list but stays in the schema, that is drift too. Restrict
	// this to keys that are neither core nor notification so a deliberate
	// server-only key does not fail the test.
	serverOnly := []string{
		"theme", "timezone", "date_format", "week_start", "language",
		"refresh_interval", "notification_filter",
		"notify_email", "notify_in_app", "notify_push", "notify_severity_floor",
		"step_up_actions", "analytics_consent", "mask_sensitive", "remember_last_org",
		"high_contrast", "underline_links", "always_show_focus",
	}
	inClient := make(map[string]bool, len(frontend))
	for _, key := range frontend {
		inClient[key] = true
	}
	var missing []string
	for key := range preferenceSchema {
		if inClient[key] || containsString(serverOnly, key) {
			continue
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("server preference keys are neither synced by the client nor declared server-only: %v", missing)
	}
}

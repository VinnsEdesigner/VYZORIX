package audit

import (
	"strings"
	"testing"
)

const auditProbeSecret = "libsql-token-value-must-not-persist"

// TestAuditEntryIsSanitizedBeforeStorage is the guard for the audit funnel. Metadata is
// free-form, so a caller recording why an action failed can pass a driver error carrying
// a live credential — and the audit table is read back through an API.
func TestAuditEntryIsSanitizedBeforeStorage(t *testing.T) {
	entry := &Entry{
		Action:       ActionLoginSuccess,
		Result:       ResultSuccess,
		Metadata:     map[string]string{"reason": "connect failed", "db_token": auditProbeSecret},
		OldValue:     `{"token":"` + auditProbeSecret + `"}`,
		NewValue:     "authToken=" + auditProbeSecret,
		UserAgent:    "curl (token=" + auditProbeSecret + ")",
		ResourceType: "device",
	}

	out := sanitizeEntry(entry)
	if out == nil {
		t.Fatal("sanitizeEntry returned nil")
	}

	fields := map[string]string{
		"metadata":   strings.Join(mapValues(out.Metadata), " "),
		"old_value":  out.OldValue,
		"new_value":  out.NewValue,
		"user_agent": out.UserAgent,
	}

	for name, value := range fields {
		if strings.Contains(value, auditProbeSecret) {
			t.Errorf("%s leaked the credential: %q", name, value)
		}
	}

	// A safe field survives, so the entry stays useful for investigation.
	if out.Metadata["reason"] != "connect failed" {
		t.Errorf("a safe metadata value was altered: %q", out.Metadata["reason"])
	}
}

// TestSanitizeEntryDoesNotMutateTheCaller proves the caller's entry is untouched: a
// silent rewrite would surprise code that still holds it.
func TestSanitizeEntryDoesNotMutateTheCaller(t *testing.T) {
	original := &Entry{
		Action:   ActionLoginSuccess,
		Metadata: map[string]string{"db_token": auditProbeSecret},
	}

	_ = sanitizeEntry(original)

	if original.Metadata["db_token"] != auditProbeSecret {
		t.Fatal("sanitizeEntry mutated the caller's entry")
	}
}

// TestSanitizeEntryNilIsNil keeps the helper total.
func TestSanitizeEntryNilIsNil(t *testing.T) {
	if sanitizeEntry(nil) != nil {
		t.Fatal("sanitizeEntry(nil) must return nil")
	}
}

// mapValues flattens a string map's values for a leak check.
func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}

	return out
}

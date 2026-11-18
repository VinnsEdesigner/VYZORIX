package auth

import (
	"errors"
	"testing"
)

func TestValidatePreference_AcceptsKnownValues(t *testing.T) {
	cases := []struct {
		key   string
		value any
	}{
		{"theme", "tron"},
		{"theme", ""}, // empty clears
		{"density", "compact"},
		{"text_scale", "large"},
		{"reduce_motion", true},
		{"time_format", "12h"},
		{"table_page_size", float64(100)},
		{"idle_timeout_minutes", float64(0)},
		{"landing_page", "devices"},
		{"step_up_actions", []any{"api-keys", "sessions"}},
		{"notify_severity_floor", "critical"},
		{"refresh_interval", float64(30)},
	}
	for _, c := range cases {
		if err := validatePreference(c.key, c.value); err != nil {
			t.Errorf("validatePreference(%q, %v) = %v, want nil", c.key, c.value, err)
		}
	}
}

func TestValidatePreference_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value any
	}{
		{"unknown key", "not_a_preference", "x"},
		{"bad enum", "density", "gigantic"},
		{"wrong type for bool", "reduce_motion", "yes"},
		{"wrong type for int", "table_page_size", "big"},
		{"non-integer int", "table_page_size", 50.5},
		{"int outside the set", "table_page_size", float64(7)},
		{"out of range int", "refresh_interval", float64(0)},
		{"list with a bad member", "step_up_actions", []any{"api-keys", "nope"}},
		{"list of non-strings", "step_up_actions", []any{1, 2}},
		{"string where list expected", "step_up_actions", "api-keys"},
	}
	for _, c := range cases {
		err := validatePreference(c.key, c.value)
		if err == nil {
			t.Errorf("%s: validatePreference(%q, %v) = nil, want error", c.name, c.key, c.value)
			continue
		}
		if !errors.Is(err, ErrValidationError) {
			t.Errorf("%s: error %v does not wrap ErrValidationError", c.name, err)
		}
	}
}

func TestClearsPreference(t *testing.T) {
	if !clearsPreference(nil) {
		t.Error("nil should clear")
	}
	if !clearsPreference("") {
		t.Error("empty string should clear")
	}
	if clearsPreference("dark") {
		t.Error("a real string should not clear")
	}
	if clearsPreference(false) {
		t.Error("false should not clear (it is a real value)")
	}
	if clearsPreference([]any{"api-keys"}) {
		t.Error("a list should not clear")
	}
}

func TestIsKnownPreferenceKey(t *testing.T) {
	if !IsKnownPreferenceKey("density") {
		t.Error("density should be known")
	}
	if IsKnownPreferenceKey("nope") {
		t.Error("nope should be unknown")
	}
}

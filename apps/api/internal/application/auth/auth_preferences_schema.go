package auth

import (
	"fmt"
	"math"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/theme"
)

// preferenceKind is the value type a preference accepts.
type preferenceKind int

const (
	preferenceString preferenceKind = iota
	preferenceBool
	preferenceInt
	preferenceStringList
)

// preferenceSpec describes one accepted preference key.
//
//   - allowed / allowedInts, when non-empty, are the exhaustive value set.
//   - min / max bound an int when allowedInts is empty.
//
// A spec with no constraints beyond the kind accepts any value of that kind.
type preferenceSpec struct {
	kind        preferenceKind
	allowed     []string
	allowedInts []int
	min         int
	max         int
}

// preferenceSchema is the allowlist of user preference keys the API accepts.
//
// This is deliberate: the client sends a flat map and the server stores it as
// JSON, so without an allowlist any authenticated caller could persist
// arbitrary keys and values. The allowlist keeps the stored document bounded and
// typed, and rejects a typo at the edge instead of silently storing it.
//
// Adding a preference means adding it here and to the client store; the two are
// checked against each other by a test so drift fails CI rather than at runtime.
var preferenceSchema = map[string]preferenceSpec{
	"theme":       {kind: preferenceString, allowed: theme.IDs()},
	"timezone":    {kind: preferenceString},
	"date_format": {kind: preferenceString},
	"week_start":  {kind: preferenceString, allowed: []string{"monday", "sunday", "saturday"}},
	"language":    {kind: preferenceString, allowed: []string{"en", "fr"}},

	"density":             {kind: preferenceString, allowed: []string{"compact", "comfortable", "spacious"}},
	"text_scale":          {kind: preferenceString, allowed: []string{"small", "default", "large"}},
	"reduce_motion":       {kind: preferenceBool},
	"time_format":         {kind: preferenceString, allowed: []string{"12h", "24h"}},
	"relative_timestamps": {kind: preferenceBool},
	"byte_units":          {kind: preferenceString, allowed: []string{"binary", "decimal"}},
	"thousands_separator": {kind: preferenceBool},

	"landing_page":        {kind: preferenceString, allowed: []string{"last", "home", "devices", "logs"}},
	"default_time_range":  {kind: preferenceString, allowed: []string{"15m", "1h", "6h", "24h", "7d"}},
	"table_page_size":     {kind: preferenceInt, allowedInts: []int{25, 50, 100}},
	"live_updates":        {kind: preferenceBool},
	"confirm_destructive": {kind: preferenceBool},
	"step_up_actions":     {kind: preferenceStringList, allowed: []string{"api-keys", "sessions", "device-wipe", "secret-rotate"}},

	"notify_email":          {kind: preferenceBool},
	"notify_in_app":         {kind: preferenceBool},
	"notify_push":           {kind: preferenceBool},
	"notify_severity_floor": {kind: preferenceString, allowed: []string{"info", "warning", "critical"}},

	"analytics_consent":    {kind: preferenceBool},
	"mask_sensitive":       {kind: preferenceBool},
	"idle_timeout_minutes": {kind: preferenceInt, allowedInts: []int{0, 15, 30, 60}},
	"remember_last_org":    {kind: preferenceBool},

	"high_contrast":     {kind: preferenceBool},
	"underline_links":   {kind: preferenceBool},
	"always_show_focus": {kind: preferenceBool},

	"refresh_interval":    {kind: preferenceInt, min: 1, max: 86400},
	"notification_filter": {kind: preferenceString},
}

// IsKnownPreferenceKey reports whether key is accepted by the API.
func IsKnownPreferenceKey(key string) bool {
	_, ok := preferenceSchema[key]
	return ok
}

// validatePreference rejects an unknown key or a value of the wrong kind or
// outside the accepted set. An empty string is a valid value for a string key:
// it means "clear this preference".
func validatePreference(key string, value any) error {
	spec, ok := preferenceSchema[key]
	if !ok {
		return fmt.Errorf("%w: unknown preference %q", ErrValidationError, key)
	}

	switch spec.kind {
	case preferenceString:
		str, isString := value.(string)
		if !isString {
			return fmt.Errorf("%w: preference %q must be a string", ErrValidationError, key)
		}
		if str == "" {
			return nil
		}
		if len(spec.allowed) > 0 && !containsString(spec.allowed, str) {
			return fmt.Errorf("%w: preference %q has an unsupported value", ErrValidationError, key)
		}
	case preferenceBool:
		if _, isBool := value.(bool); !isBool {
			return fmt.Errorf("%w: preference %q must be a boolean", ErrValidationError, key)
		}
	case preferenceInt:
		n, isInt := asInt(value)
		if !isInt {
			return fmt.Errorf("%w: preference %q must be an integer", ErrValidationError, key)
		}
		if len(spec.allowedInts) > 0 {
			if !containsInt(spec.allowedInts, n) {
				return fmt.Errorf("%w: preference %q has an unsupported value", ErrValidationError, key)
			}
		} else if n < spec.min || n > spec.max {
			return fmt.Errorf("%w: preference %q is out of range", ErrValidationError, key)
		}
	case preferenceStringList:
		list, isList := value.([]any)
		if !isList {
			return fmt.Errorf("%w: preference %q must be a list", ErrValidationError, key)
		}
		for _, item := range list {
			str, isString := item.(string)
			if !isString {
				return fmt.Errorf("%w: preference %q must contain strings", ErrValidationError, key)
			}
			if len(spec.allowed) > 0 && !containsString(spec.allowed, str) {
				return fmt.Errorf("%w: preference %q has an unsupported value", ErrValidationError, key)
			}
		}
	}

	return nil
}

// clearsPreference reports whether a value means "unset this key". A nil value
// always clears; an empty string clears a string key. Other values set the key.
// (An interface holding a slice is not comparable to "", so the type is checked
// before comparing.)
func clearsPreference(value any) bool {
	if value == nil {
		return true
	}
	if str, isString := value.(string); isString {
		return str == ""
	}
	return false
}

// asInt coerces the integer kinds a caller may use. JSON decoding yields
// float64; Go callers and tests pass int or int64. A non-integral float is
// rejected so 50.5 does not silently truncate to 50.
func asInt(value any) (int, bool) {
	switch n := value.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func containsString(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

func containsInt(haystack []int, needle int) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// Package theme holds the set of UI themes the server will accept.
//
// The theme catalogue lives in the frontend (apps/VyzoriX_web/src/lib/theme)
// because that is where the styles are. The server keeps its own copy so it can
// reject an unknown id without trusting the client.
//
// A test (theme_test.go) asserts the two id sets agree, so adding a theme means
// touching both, and drift fails CI rather than silently rejecting a valid id.
package theme

// ID is a theme identifier.
type ID string

// DefaultID is applied when neither the user nor the organisation has set a
// theme. Kept in step with the frontend DEFAULT_THEME_ID.
const DefaultID ID = "dark"

// SystemID is not a theme: it selects light/dark from the OS preference. The
// server accepts it so the preference round-trips, but stores no theme block.
const SystemID ID = "system"

// All lists every selectable theme id. Order matches the frontend registry.
var All = []ID{
	"dark",
	"light",
	"rose-500",
	"burgundy",
	"matrix",
	"tron",
	"synthwave",
	"zen",
}

var valid = func() map[ID]struct{} {
	m := make(map[ID]struct{}, len(All)+1)
	for _, id := range All {
		m[id] = struct{}{}
	}
	m[SystemID] = struct{}{}
	return m
}()

// IsValid reports whether id names a selectable theme. The empty string is
// valid: it means "clear the preference and fall through to the next tier".
func IsValid(id string) bool {
	if id == "" {
		return true
	}
	_, ok := valid[ID(id)]
	return ok
}

// IDs returns the selectable theme ids as strings, for tests and API metadata.
func IDs() []string {
	out := make([]string, len(All))
	for i, id := range All {
		out[i] = string(id)
	}
	return out
}

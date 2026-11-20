package provisioning

import (
	"fmt"
	"path"
	"strings"
)

// Scope is a repository's include/exclude filter over resource paths.
//
// It is a value, not a pointer, so "no filter" is the zero Scope and InScope reports
// true for everything — a repository that declares nothing syncs everything under its
// path, which is the behaviour every existing repository already has.
type Scope struct {
	Include []string
	Exclude []string
}

// NewScope builds a Scope from the stored patterns.
func NewScope(include, exclude []string) Scope {
	return Scope{Include: include, Exclude: exclude}
}

// InScope reports whether a resource path is inside the repository's declared scope.
//
// Exclude wins over Include: a path matching both is out of scope. The safe reading of
// an ambiguous rule is "do not touch it", because acting on a file the operator meant
// to exclude is the failure that matters.
func (s Scope) InScope(resourcePath string) bool {
	p := normaliseScopePath(resourcePath)

	if matchesAny(s.Exclude, p) {
		return false
	}
	if len(s.Include) == 0 {
		return true
	}

	return matchesAny(s.Include, p)
}

// matchesAny reports whether p matches any of the patterns.
func matchesAny(patterns []string, p string) bool {
	for _, pattern := range patterns {
		if matchScopePattern(pattern, p) {
			return true
		}
	}

	return false
}

// matchScopePattern matches one pattern against a path.
//
// It is path.Match, plus one rule a user will expect and path.Match does not give: a
// pattern ending in "/*" means "everything under that directory", not "one level under
// it". "orgs/*" is how a person writes "all of orgs/", and silently excluding nothing
// because the tree is deeper than one level is the kind of quiet no-op this filter must
// not have.
func matchScopePattern(pattern, p string) bool {
	if ok, err := path.Match(pattern, p); err == nil && ok {
		return true
	}

	// "dir/*" (or "dir/*/" shapes) means the whole subtree.
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok {
		return p == prefix || strings.HasPrefix(p, prefix+"/")
	}

	return false
}

// normaliseScopePath strips a leading slash so a pattern is written against
// "devices/a/vyzorix.toml" rather than "/devices/a/vyzorix.toml".
func normaliseScopePath(p string) string {
	return strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(p)), "/")
}

// ValidateScope refuses a malformed pattern. It runs on save, so a bad glob is a 400 on
// the repository rather than a sync that silently matches nothing.
func ValidateScope(include, exclude []string) error {
	for _, group := range []struct {
		field    string
		patterns []string
	}{
		{"include", include},
		{"exclude", exclude},
	} {
		for _, pattern := range group.patterns {
			if strings.TrimSpace(pattern) == "" {
				return fmt.Errorf("%s pattern must not be empty", group.field)
			}
			// path.Match reports a bad pattern with ErrBadPattern when it has to scan it.
			if _, err := path.Match(pattern, "probe"); err != nil {
				return fmt.Errorf("%s pattern %q is malformed: %v", group.field, pattern, err)
			}
		}
	}

	return nil
}

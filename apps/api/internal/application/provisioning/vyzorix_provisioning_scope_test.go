package provisioning

import "testing"

// TestScopeEmptyMeansEverything proves a repository that declares no filter syncs
// everything, which is the behaviour every existing repository already relies on.
func TestScopeEmptyMeansEverything(t *testing.T) {
	var s Scope

	for _, p := range []string{"devices/a/vyzorix.toml", "devices/b/vyzorix.v", "orgs/acme/alerts.toml"} {
		if !s.InScope(p) {
			t.Fatalf("%q was excluded by an empty scope", p)
		}
	}
}

// TestScopeIncludeNarrows proves an include list restricts to what it matches.
func TestScopeIncludeNarrows(t *testing.T) {
	s := NewScope([]string{"devices/*/vyzorix.toml"}, nil)

	if !s.InScope("devices/a/vyzorix.toml") {
		t.Fatal("a matching path was excluded")
	}
	if s.InScope("devices/a/vyzorix.v") {
		t.Fatal("a non-matching path was included")
	}
	if s.InScope("orgs/acme/alerts.toml") {
		t.Fatal("a path outside every include pattern was included")
	}
}

// TestScopeExcludeWins proves an ambiguous rule excludes. Acting on a file the operator
// meant to exclude is the failure that matters, so the tie breaks toward "do not touch".
func TestScopeExcludeWins(t *testing.T) {
	s := NewScope([]string{"devices/*/*"}, []string{"devices/staging/*"})

	if s.InScope("devices/staging/vyzorix.toml") {
		t.Fatal("a path matching both include and exclude was included")
	}
	if !s.InScope("devices/prod/vyzorix.toml") {
		t.Fatal("a path matching only include was excluded")
	}
}

// TestScopeTrailingWildcardMeansSubtree pins the one rule path.Match does not give: a
// pattern ending in "/*" means everything under that directory, not one level under it.
// "orgs/*" is how a person writes "all of orgs/", and excluding nothing because the tree
// is deeper than one level would be a silent no-op.
func TestScopeTrailingWildcardMeansSubtree(t *testing.T) {
	s := NewScope(nil, []string{"orgs/*"})

	if s.InScope("orgs/org-1/alerts.toml") {
		t.Fatal("`orgs/*` did not cover a file one level under orgs/")
	}
	if s.InScope("orgs/org-1/nested/deep.toml") {
		t.Fatal("`orgs/*` did not cover a file deeper in the orgs/ tree")
	}
	if !s.InScope("devices/a/vyzorix.toml") {
		t.Fatal("`orgs/*` excluded a path outside orgs/")
	}

	// A mid-pattern wildcard still follows path.Match: `*` is one segment. Used as an
	// exclude, a match means "out of scope", so the assertions read inverted.
	mid := NewScope(nil, []string{"devices/*/vyzorix.toml"})
	if mid.InScope("devices/a/vyzorix.toml") {
		t.Fatal("a mid-pattern `*` did not match one segment")
	}
	if !mid.InScope("devices/a/nested/vyzorix.toml") {
		t.Fatal("a mid-pattern `*` crossed `/`")
	}
}

// TestScopeNormalisesLeadingSlash proves a stored path with a leading slash matches the
// same patterns as one without, so the two cannot behave differently.
func TestScopeNormalisesLeadingSlash(t *testing.T) {
	s := NewScope([]string{"devices/a/vyzorix.toml"}, nil)

	if !s.InScope("/devices/a/vyzorix.toml") {
		t.Fatal("a leading slash changed the match")
	}
}

// TestValidateScopeRejectsBadPatterns proves a malformed glob is refused on save rather
// than silently matching nothing at sync time.
func TestValidateScopeRejectsBadPatterns(t *testing.T) {
	if err := ValidateScope([]string{"devices/["}, nil); err == nil {
		t.Fatal("a malformed include pattern was accepted")
	}
	if err := ValidateScope(nil, []string{"devices/["}); err == nil {
		t.Fatal("a malformed exclude pattern was accepted")
	}
	if err := ValidateScope([]string{"  "}, nil); err == nil {
		t.Fatal("a blank pattern was accepted")
	}
	if err := ValidateScope([]string{"devices/*"}, []string{"orgs/*"}); err != nil {
		t.Fatalf("valid patterns were refused: %v", err)
	}
}

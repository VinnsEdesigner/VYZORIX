package provisioning

import (
	"errors"
	"testing"
)

// TestBrowseJoinRefusesEscapes pins the classification: a path that leaves the checkout is
// ErrPathOutsideRepo, which the handler answers 400. Reusing the pusher's safeJoin made it
// ErrPushFailed, and the browse handler has no case for that, so a refusal became a 500 —
// telling the caller the server broke when it had done exactly what it should.
func TestBrowseJoinRefusesEscapes(t *testing.T) {
	root := t.TempDir()

	escapes := []string{
		"../secret",
		"../../etc/passwd",
		"devices/../../etc/passwd",
		"/../etc/passwd",
	}

	for _, rel := range escapes {
		t.Run(rel, func(t *testing.T) {
			if _, err := browseJoin(root, rel); !errors.Is(err, ErrPathOutsideRepo) {
				t.Fatalf("browseJoin(%q) err = %v, want ErrPathOutsideRepo", rel, err)
			}
		})
	}
}

// A contained path resolves inside the root, so the normal case is not refused.
func TestBrowseJoinAcceptsContainedPaths(t *testing.T) {
	root := t.TempDir()

	for _, rel := range []string{"devices/dev-1/vyzorix.toml", "orgs/acme/alerts.toml", "README.md"} {
		full, err := browseJoin(root, rel)
		if err != nil {
			t.Fatalf("browseJoin(%q) = %v, want no error", rel, err)
		}
		if full == "" {
			t.Fatalf("browseJoin(%q) returned an empty path", rel)
		}
	}
}

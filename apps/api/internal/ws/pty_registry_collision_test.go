package hub

import (
	"errors"
	"testing"
)

// TestPTYRegistryRejectsDuplicateSessionID is the cross-operator isolation
// guard: session IDs are client-supplied, so two operators (or one operator
// replaying a request) can send the same ID. If Open overwrote the existing
// entry, the first session would leak (never closed, still counted against the
// device cap) and the second operator would inherit the first's session.
func TestPTYRegistryRejectsDuplicateSessionID(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})

	first, err := r.Open("sess-1", "dev-1", "op-1", "org-1", false)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// A different operator reusing the same ID must be refused.
	_, err = r.Open("sess-1", "dev-2", "op-2", "org-1", false)
	if !errors.Is(err, ErrPTYSessionExists) {
		t.Fatalf("duplicate Open err = %v, want ErrPTYSessionExists", err)
	}

	// The original session must be untouched.
	got, err := r.Get("sess-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.OperatorID != "op-1" || got.DeviceID != "dev-1" {
		t.Fatalf("session = %+v, want the original op-1/dev-1", got)
	}
	if got != first {
		t.Fatal("the original session pointer must be preserved")
	}

	// And the failed open must not have consumed a device slot.
	if n := r.CountForDevice("dev-2"); n != 0 {
		t.Fatalf("dev-2 sessions = %d, want 0 after a rejected open", n)
	}
	if n := r.Count(); n != 1 {
		t.Fatalf("total sessions = %d, want 1", n)
	}
}

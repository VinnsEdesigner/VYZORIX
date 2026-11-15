package hub

import (
	"errors"
	"testing"
	"time"
)

func TestPTYRegistryOpenAndClose(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})

	s, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s.Credit != DefaultPTYCreditWindow {
		t.Fatalf("credit = %d, want the full window %d", s.Credit, DefaultPTYCreditWindow)
	}
	if r.Count() != 1 || r.CountForDevice("dev-1") != 1 {
		t.Fatalf("counts = %d/%d, want 1/1", r.Count(), r.CountForDevice("dev-1"))
	}

	if _, err := r.Close("11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if r.Count() != 0 || r.CountForDevice("dev-1") != 0 {
		t.Fatal("counts must drop to zero after close")
	}
}

func TestPTYRegistryEnforcesPerDeviceCap(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{MaxSessionsPerDevice: 2})

	for i, id := range []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"} {
		if _, err := r.Open(id, "dev-1", "op-1", "org-1", false); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}

	// The third must be refused, but another device is unaffected.
	if _, err := r.Open("33333333-3333-4333-8333-333333333333", "dev-1", "op-1", "org-1", false); !errors.Is(err, ErrPTYTooManySessions) {
		t.Fatalf("err = %v, want ErrPTYTooManySessions", err)
	}
	if _, err := r.Open("44444444-4444-4444-8444-444444444444", "dev-2", "op-1", "org-1", false); err != nil {
		t.Fatalf("a different device must not be capped by dev-1: %v", err)
	}
}

func TestPTYRegistryCreditWindow(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{CreditWindow: 3})

	if _, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := r.TakeCredit("11111111-1111-4111-8111-111111111111"); err != nil {
			t.Fatalf("take %d: %v", i, err)
		}
	}

	// Exhausted: the relay uses this to drop output rather than buffer forever.
	if err := r.TakeCredit("11111111-1111-4111-8111-111111111111"); !errors.Is(err, ErrPTYNoCredit) {
		t.Fatalf("err = %v, want ErrPTYNoCredit", err)
	}

	// An ack restores credit...
	r.GrantCredit("11111111-1111-4111-8111-111111111111", 1)
	if err := r.TakeCredit("11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatalf("take after grant: %v", err)
	}

	// ...but never beyond the window, so a buggy user cannot inflate it.
	r.GrantCredit("11111111-1111-4111-8111-111111111111", 1000)
	s, _ := r.Get("11111111-1111-4111-8111-111111111111")
	if s.Credit > 3 {
		t.Fatalf("credit = %d, want it capped at the window (3)", s.Credit)
	}
}

func TestPTYRegistryCreditForUnknownSession(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})

	if err := r.TakeCredit("dddddddd-1111-4111-8111-111111111111"); !errors.Is(err, ErrPTYSessionNotFound) {
		t.Fatalf("err = %v, want ErrPTYSessionNotFound", err)
	}
	// Granting to an unknown session is a no-op, not a panic: the session may
	// have closed between the device sending data and the ack arriving.
	r.GrantCredit("dddddddd-1111-4111-8111-111111111111", 5)
}

func TestPTYRegistryCloseByDevice(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})

	_, _ = r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false)
	_, _ = r.Open("22222222-2222-4222-8222-222222222222", "dev-1", "op-2", "org-1", false)
	_, _ = r.Open("33333333-3333-4333-8333-333333333333", "dev-2", "op-1", "org-1", false)

	closed := r.CloseByDevice("dev-1")
	if len(closed) != 2 {
		t.Fatalf("closed = %d, want 2", len(closed))
	}
	if r.CountForDevice("dev-1") != 0 {
		t.Fatal("dev-1 must have no sessions left")
	}
	if r.CountForDevice("dev-2") != 1 {
		t.Fatal("dev-2's session must be untouched")
	}
}

func TestPTYRegistryExpiry(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	r := NewPTYRegistry(PTYRegistryConfig{
		SessionTTL: time.Minute,
		Now:        func() time.Time { return now },
	})

	_, _ = r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false)

	// Advance past the TTL without touching the session.
	now = now.Add(2 * time.Minute)

	expired := r.Expired()
	if len(expired) != 1 || expired[0].SessionID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("expired = %v, want s1", expired)
	}

	// Activity keeps a session alive.
	now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	_, _ = r.Open("22222222-2222-4222-8222-222222222222", "dev-2", "op-1", "org-1", false)
	now = now.Add(30 * time.Second)
	r.Touch("22222222-2222-4222-8222-222222222222")
	now = now.Add(30 * time.Second)

	for _, s := range r.Expired() {
		if s.SessionID == "22222222-2222-4222-8222-222222222222" {
			t.Fatal("a touched session must not be reported as expired")
		}
	}
}

func TestPTYRegistryCloseIsIdempotent(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})
	_, _ = r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false)

	if _, err := r.Close("11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if _, err := r.Close("11111111-1111-4111-8111-111111111111"); !errors.Is(err, ErrPTYSessionNotFound) {
		t.Fatalf("second close err = %v, want ErrPTYSessionNotFound", err)
	}
}

func TestPTYRegistryReopenSameUserRebinds(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})

	first, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Spend some credit so the re-open's reset is observable.
	if err := r.TakeCredit(first.SessionID); err != nil {
		t.Fatalf("TakeCredit: %v", err)
	}

	// The same user re-opening the same id is a reattach, not an error.
	again, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false)
	if err != nil {
		t.Fatalf("re-open must rebind, got: %v", err)
	}
	if again != first {
		t.Fatal("re-open must return the existing session, not a new one")
	}
	if r.Count() != 1 || r.CountForDevice("dev-1") != 1 {
		t.Fatalf("counts = %d/%d, want 1/1 (no second session)",
			r.Count(), r.CountForDevice("dev-1"))
	}
	if again.Credit != DefaultPTYCreditWindow {
		t.Fatalf("credit = %d, want a reset window %d", again.Credit, DefaultPTYCreditWindow)
	}
}

func TestPTYRegistryReopenDifferentUserIsRefused(t *testing.T) {
	// The session id is client-supplied, so a different user re-opening a live
	// id must be refused: allowing it would let one user adopt another's shell
	// by guessing the id.
	r := NewPTYRegistry(PTYRegistryConfig{})

	if _, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open: %v", err)
	}

	_, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-2", "org-1", false)
	if !errors.Is(err, ErrPTYSessionExists) {
		t.Fatalf("err = %v, want ErrPTYSessionExists", err)
	}
}

func TestPTYRegistryForDeviceReturnsOnlyItsSessions(t *testing.T) {
	r := NewPTYRegistry(PTYRegistryConfig{})

	if _, err := r.Open("11111111-1111-4111-8111-111111111111", "dev-1", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open s1: %v", err)
	}
	if _, err := r.Open("22222222-2222-4222-8222-222222222222", "dev-2", "op-1", "org-1", false); err != nil {
		t.Fatalf("Open s2: %v", err)
	}

	got := r.ForDevice("dev-1")
	if len(got) != 1 || got[0].SessionID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("ForDevice(dev-1) = %v, want only its own session", got)
	}
}

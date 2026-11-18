package user

import (
	"testing"
)

// TestStrictMailboxValidator_RejectsNonDeliverable proves the notification-address
// validator refuses an address whose domain cannot receive mail, which a plain
// format check would accept.
func TestStrictMailboxValidator_RejectsNonDeliverable(t *testing.T) {
	validator := StrictMailboxValidator()

	cases := []struct {
		name  string
		email string
		valid bool
	}{
		{"missing @", "not-an-email", false},
		{"no tld", "user@localhost", false},
		{"empty", "", false},
		{"nonexistent domain", "someone@this-domain-does-not-exist-zzz9.invalid", false},
	}
	for _, c := range cases {
		result := ValidateEmailFull(c.email, validator)
		if result.Valid != c.valid {
			t.Errorf("%s: ValidateEmailFull(%q).Valid = %v, want %v (errors: %v)", c.name, c.email, result.Valid, c.valid, result.Errors)
		}
	}
}

// TestStrictMailboxValidator_RejectsDisposable proves disposable domains are refused.
func TestStrictMailboxValidator_RejectsDisposable(t *testing.T) {
	result := ValidateEmailFull("someone@mailinator.com", StrictMailboxValidator())
	if result.Valid {
		t.Error("a disposable domain should be rejected")
	}
}

package organization

import (
	"errors"
	"fmt"
)

// ErrInvalidOrganizationID is returned when an organization ID is invalid.
var ErrInvalidOrganizationID = errors.New("invalid organization ID")

// ErrInvalidMemberID is returned when a member ID is invalid.
var ErrInvalidMemberID = errors.New("invalid member ID")

// ErrInvalidUserID is returned when a user ID is invalid.
var ErrInvalidUserID = errors.New("invalid operator ID")

// UserID is a value object representing a user's unique identifier.
type UserID struct {
	value string
}

// NewUserID creates a new UserID from a string value.
// Returns ErrInvalidUserID if the value is empty.
func NewUserID(value string) (UserID, error) {
	if value == "" {
		return UserID{}, ErrInvalidUserID
	}
	return UserID{value: value}, nil
}

// MustNewUserID creates a new UserID from a string value.
// Panics if the value is empty.
func MustNewUserID(value string) UserID {
	id, err := NewUserID(value)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string value of the UserID.
func (id UserID) String() string {
	return id.value
}

// IsZero returns true if the UserID is the zero value.
func (id UserID) IsZero() bool {
	return id.value == ""
}

// Equals returns true if two UserIDs are equal.
func (id UserID) Equals(other UserID) bool {
	return id.value == other.value
}

// Validate returns an error if the ID is invalid.
func (id UserID) Validate() error {
	if id.IsZero() {
		return ErrInvalidUserID
	}
	return nil
}

// Format implements fmt.Formatter for pretty printing.
func (id UserID) Format(s fmt.State, verb rune) {
	format := "%s"
	if verb == 'v' && s.Flag('#') {
		format = "%q"
	}
	_, _ = fmt.Fprintf(s, format, id.value)
}

// OrganizationID is a value object representing an organization's unique identifier.
type OrganizationID struct {
	value string
}

// NewOrganizationID creates a new OrganizationID from a string value.
// Returns ErrInvalidOrganizationID if the value is empty.
func NewOrganizationID(value string) (OrganizationID, error) {
	if value == "" {
		return OrganizationID{}, ErrInvalidOrganizationID
	}
	return OrganizationID{value: value}, nil
}

// MustNewOrganizationID creates a new OrganizationID from a string value.
// Panics if the value is empty.
func MustNewOrganizationID(value string) OrganizationID {
	id, err := NewOrganizationID(value)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string value of the OrganizationID.
func (id OrganizationID) String() string {
	return id.value
}

// IsZero returns true if the OrganizationID is the zero value.
func (id OrganizationID) IsZero() bool {
	return id.value == ""
}

// Equals returns true if two OrganizationIDs are equal.
func (id OrganizationID) Equals(other OrganizationID) bool {
	return id.value == other.value
}

// Validate returns an error if the ID is invalid.
func (id OrganizationID) Validate() error {
	if id.IsZero() {
		return ErrInvalidOrganizationID
	}
	return nil
}

// Format implements fmt.Formatter for pretty printing.
func (id OrganizationID) Format(s fmt.State, verb rune) {
	format := "%s"
	if verb == 'v' && s.Flag('#') {
		format = "%q"
	}
	_, _ = fmt.Fprintf(s, format, id.value)
}

// MemberID is a value object representing a member's unique identifier.
type MemberID struct {
	value string
}

// NewMemberID creates a new MemberID from a string value.
// Returns ErrInvalidMemberID if the value is empty.
func NewMemberID(value string) (MemberID, error) {
	if value == "" {
		return MemberID{}, ErrInvalidMemberID
	}
	return MemberID{value: value}, nil
}

// MustNewMemberID creates a new MemberID from a string value.
// Panics if the value is empty.
func MustNewMemberID(value string) MemberID {
	id, err := NewMemberID(value)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string value of the MemberID.
func (id MemberID) String() string {
	return id.value
}

// IsZero returns true if the MemberID is the zero value.
func (id MemberID) IsZero() bool {
	return id.value == ""
}

// Equals returns true if two MemberIDs are equal.
func (id MemberID) Equals(other MemberID) bool {
	return id.value == other.value
}

// Validate returns an error if the ID is invalid.
func (id MemberID) Validate() error {
	if id.IsZero() {
		return ErrInvalidMemberID
	}
	return nil
}

// Format implements fmt.Formatter for pretty printing.
func (id MemberID) Format(s fmt.State, verb rune) {
	format := "%s"
	if verb == 'v' && s.Flag('#') {
		format = "%q"
	}
	_, _ = fmt.Fprintf(s, format, id.value)
}

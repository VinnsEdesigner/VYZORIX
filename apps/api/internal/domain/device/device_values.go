package device

import (
	"errors"
	"fmt"
)

// ErrInvalidDeviceID is returned when a device ID is invalid.
var ErrInvalidDeviceID = errors.New("invalid device ID")

// DeviceID is a value object representing a device's unique identifier.
type DeviceID struct {
	value string
}

// NewDeviceID creates a new DeviceID from a string value.
// Returns ErrInvalidDeviceID if the value is empty.
func NewDeviceID(value string) (DeviceID, error) {
	if value == "" {
		return DeviceID{}, ErrInvalidDeviceID
	}
	return DeviceID{value: value}, nil
}

// String returns the string value of the DeviceID.
func (id DeviceID) String() string {
	return id.value
}

// IsZero returns true if the DeviceID is the zero value.
func (id DeviceID) IsZero() bool {
	return id.value == ""
}

// Equals returns true if two DeviceIDs are equal.
func (id DeviceID) Equals(other DeviceID) bool {
	return id.value == other.value
}

// MarshalText implements encoding.TextMarshaler for database storage.
func (id DeviceID) MarshalText() ([]byte, error) {
	return []byte(id.value), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for database retrieval.
func (id *DeviceID) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*id = DeviceID{}
		return nil
	}
	newID, err := NewDeviceID(string(text))
	if err != nil {
		return err
	}
	*id = newID
	return nil
}

// UserID is a value object representing a user's unique identifier.
type UserID struct {
	value string
}

// NewUserID creates a new UserID from a string value.
// Returns an error if the value is empty.
func NewUserID(value string) (UserID, error) {
	if value == "" {
		return UserID{}, errors.New("invalid operator ID")
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
func (id DeviceID) Validate() error {
	if id.IsZero() {
		return ErrInvalidDeviceID
	}
	return nil
}

// Format implements fmt.Formatter for pretty printing.
func (id DeviceID) Format(s fmt.State, verb rune) {
	format := "%s"
	if verb == 'v' && s.Flag('#') {
		format = "%q"
	}
	_, _ = fmt.Fprintf(s, format, id.value)
}

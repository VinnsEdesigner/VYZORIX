package storage

import (
	"time"
)

// sqlTime scans a TIMESTAMP column that may hold any of the shapes SQLite hands
// back: an int64 of Unix milliseconds, a string, or a time.Time.
//
// The schema declares these columns TIMESTAMP, but what the driver returns depends
// on what was written — the repositories write time.Time in some places and Unix
// millis in others, and a value can arrive as either. Scanning straight into
// time.Time fails at runtime with "unsupported Scan, storing driver.Value type
// string into type *time.Time", which is how a device enroll and every command
// result were failing. This delegates to parseTimestamp, which already handles
// every form, so a scan site cannot get the type wrong.
type sqlTime struct {
	dst *time.Time
}

func (t sqlTime) Scan(value any) error {
	*t.dst = parseTimestamp(value)

	return nil
}

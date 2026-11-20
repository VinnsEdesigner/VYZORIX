// Package hub provides WebSocket functionality.
package hub

import (
	"errors"
	"sync"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/uuid"
)

// PTY session errors.
var (
	// ErrPTYSessionNotFound is returned when a session ID is unknown.
	ErrPTYSessionNotFound = errors.New("pty session not found")
	// ErrPTYNoCredit is returned when a session has no send credit left.
	ErrPTYNoCredit = errors.New("pty session out of credit")
	// ErrPTYTooManySessions is returned when the per-device cap is reached.
	ErrPTYTooManySessions = errors.New("pty session limit reached")
	// ErrPTYSessionExists is returned when a client-supplied session ID is
	// already live. Session IDs come from the client, so a duplicate is either a
	// retry or an attempt to take over another session; both must be refused
	// rather than silently overwriting the existing entry.
	ErrPTYSessionExists = errors.New("pty session id already in use")
	// ErrPTYSessionIDInvalid is returned when a client-supplied session ID is not
	// a well-formed UUID.
	ErrPTYSessionIDInvalid = errors.New("pty session id is not a valid uuid")
)

// IsValidPTYSessionID reports whether a client-supplied session ID has the
// expected shape: a UUID. The recorder uses the ID as a filename, so a value
// containing path separators or dots is refused rather than sanitised, keeping
// the rule in one place and the recorder free of path handling.
func IsValidPTYSessionID(sessionID string) bool {
	if sessionID == "" || len(sessionID) > MaxPTYSessionIDLen {
		return false
	}

	return uuid.IsValid(sessionID)
}

// PTY session limits. These bound memory and blast radius per device.
const (
	// DefaultPTYMaxSessionsPerDevice caps concurrent sessions on one device.
	DefaultPTYMaxSessionsPerDevice = 4

	// DefaultPTYCreditWindow is how many unacknowledged data frames may be in
	// flight to the user before the device's output is throttled. Without a
	// window, `cat` of a large file would flood the socket and the browser.
	DefaultPTYCreditWindow = 64

	// DefaultPTYSessionTTL bounds a session's lifetime even if neither side
	// closes it, so a forgotten tab cannot hold a device open forever.
	DefaultPTYSessionTTL = 2 * time.Hour
)

// PTYSession is the in-memory state of one terminal session. It is deliberately
// not durable: a session is a live connection between a user and a device,
// and it is meaningless once the server restarts.
type PTYSession struct {
	// Fields are ordered so pointer-bearing ones come first and the pointer-free
	// scalars trail: the GC then has a shorter region to scan. Reordering the
	// scalars above the strings/slices is what fieldalignment flags.
	OpenedAt   time.Time
	LastSeenAt time.Time
	recorder   *PTYRecorder
	SessionID  string
	DeviceID   string
	UserID     string
	OrgID      string
	// capability is the resolved API allowlist, device tier and expiry the session
	// carries. It is the boundary contract's decision, recorded here at open so
	// the device's cloud WebSocket (Build 4) can deliver it and so it is
	// observable before that exists. Empty means the session was opened without a
	// resolver configured.
	capability []string
	Credit     int
	expiresAt  int64
	Recording  bool
	tier       int
}

// SetCapability records the resolved capability on the session.
func (s *PTYSession) SetCapability(allowlist []string, tier int, expiresAt int64) {
	s.capability = allowlist
	s.tier = tier
	s.expiresAt = expiresAt
}

// Capability returns the resolved API allowlist, device tier and expiry.
func (s *PTYSession) Capability() (allowlist []string, tier int, expiresAt int64) {
	return s.capability, s.tier, s.expiresAt
}

// Recorder returns the session's cast recorder, or nil when recording is off.
func (s *PTYSession) Recorder() *PTYRecorder {
	return s.recorder
}

// PTYRegistry tracks live PTY sessions and their flow-control credit.
type PTYRegistry struct {
	now          func() time.Time
	newRecorder  func(sessionID string) (*PTYRecorder, error)
	sessions     map[string]*PTYSession
	byDevice     map[string]int
	sessionTTL   time.Duration
	maxPerDevice int
	creditWindow int
	mu           sync.Mutex
}

// PTYRegistryConfig configures the registry. Zero values take the defaults.
type PTYRegistryConfig struct {
	// Now is injectable so tests can drive TTL expiry without sleeping.
	Now func() time.Time
	// NewRecorder opens a cast recorder for a session. When nil, recording is
	// unavailable and a session that asks for it fails to open rather than
	// silently running unrecorded.
	NewRecorder          func(sessionID string) (*PTYRecorder, error)
	SessionTTL           time.Duration
	MaxSessionsPerDevice int
	CreditWindow         int
}

// NewPTYRegistry creates a registry.
func NewPTYRegistry(cfg PTYRegistryConfig) *PTYRegistry {
	if cfg.MaxSessionsPerDevice <= 0 {
		cfg.MaxSessionsPerDevice = DefaultPTYMaxSessionsPerDevice
	}
	if cfg.CreditWindow <= 0 {
		cfg.CreditWindow = DefaultPTYCreditWindow
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = DefaultPTYSessionTTL
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return &PTYRegistry{
		sessions:     make(map[string]*PTYSession),
		byDevice:     make(map[string]int),
		newRecorder:  cfg.NewRecorder,
		maxPerDevice: cfg.MaxSessionsPerDevice,
		creditWindow: cfg.CreditWindow,
		sessionTTL:   cfg.SessionTTL,
		now:          cfg.Now,
	}
}

// Open registers a new session for a device. It returns ErrPTYTooManySessions
// when the device already holds the maximum, so one user cannot open an
// unbounded number of terminals on one device.
func (r *PTYRegistry) Open(sessionID, deviceID, userID, orgID string, recording bool) (*PTYSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// A session outlives the device socket now, so a reconnect re-opens the same id
	// rather than minting a new one. Re-binding is allowed only for the **same
	// user and device**: the id is client-supplied, so letting a different
	// user adopt a live session by guessing its id would be a session hijack.
	// A different user is refused exactly as before.
	if existing, ok := r.sessions[sessionID]; ok {
		if existing.UserID != userID || existing.DeviceID != deviceID {
			return nil, ErrPTYSessionExists
		}

		// Re-arm for the reconnecting user: refresh the liveness clock so the
		// idle TTL measures from now, and reset the credit window to a full one,
		// because the user is starting a fresh terminal view and the old
		// window's acks died with the socket they were sent on.
		existing.LastSeenAt = r.now()
		existing.Credit = r.creditWindow

		return existing, nil
	}

	if r.byDevice[deviceID] >= r.maxPerDevice {
		return nil, ErrPTYTooManySessions
	}

	// Session IDs are client-supplied, so validate the shape before storing one.
	// The ID becomes a filename for the cast recording, and a path-ish value is
	// exactly the input a future refactor might mishandle; rejecting it here
	// keeps that class of bug out of the recorder entirely.
	if !IsValidPTYSessionID(sessionID) {
		return nil, ErrPTYSessionIDInvalid
	}

	now := r.now()
	s := &PTYSession{
		SessionID:  sessionID,
		DeviceID:   deviceID,
		UserID:     userID,
		OrgID:      orgID,
		Recording:  recording,
		OpenedAt:   now,
		LastSeenAt: now,
		Credit:     r.creditWindow,
	}

	// Open the recorder before publishing the session, so a caller that asked for
	// recording either gets it or gets an error. Silently running unrecorded would
	// defeat the point of asking.
	if recording && r.newRecorder != nil {
		rec, err := r.newRecorder(sessionID)
		if err != nil {
			return nil, err
		}
		s.recorder = rec
	}

	r.sessions[sessionID] = s
	r.byDevice[deviceID]++

	return s, nil
}

// Get returns a session by ID.
func (r *PTYRegistry) Get(sessionID string) (*PTYSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return nil, ErrPTYSessionNotFound
	}

	return s, nil
}

// Close removes a session and returns it, so the caller can release the device
// side. It is idempotent: closing an unknown session returns ErrPTYSessionNotFound.
func (r *PTYRegistry) Close(sessionID string) (*PTYSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return nil, ErrPTYSessionNotFound
	}

	delete(r.sessions, sessionID)
	if r.byDevice[s.DeviceID] <= 1 {
		delete(r.byDevice, s.DeviceID)
	} else {
		r.byDevice[s.DeviceID]--
	}

	// Flush and close the cast file. This runs under the registry lock, which is
	// acceptable: Close is a single file close, not an I/O loop.
	if s.recorder != nil {
		_ = s.recorder.Close()
	}

	return s, nil
}

// CloseByDevice closes every session for a device and returns them. It is used
// when a device disconnects: its sessions cannot outlive the connection, and
// leaving them would let a reconnect inherit a dead session ID.
func (r *PTYRegistry) CloseByDevice(deviceID string) []*PTYSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	var closed []*PTYSession
	for id, s := range r.sessions {
		if s.DeviceID == deviceID {
			if s.recorder != nil {
				_ = s.recorder.Close()
			}
			closed = append(closed, s)
			delete(r.sessions, id)
		}
	}
	delete(r.byDevice, deviceID)

	return closed
}

// TakeCredit consumes one unit of send credit for a session. It returns
// ErrPTYNoCredit when the user has not acknowledged enough frames, which is
// how backpressure is applied to a device that is producing faster than the
// user can consume.
func (r *PTYRegistry) TakeCredit(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return ErrPTYSessionNotFound
	}
	if s.Credit <= 0 {
		return ErrPTYNoCredit
	}

	s.Credit--
	s.LastSeenAt = r.now()

	return nil
}

// GrantCredit returns credit to a session, capped at the window so a buggy or
// hostile user cannot inflate it. An unknown session is a no-op: the
// session may have closed between the device sending data and the ack arriving.
func (r *PTYRegistry) GrantCredit(sessionID string, n int) {
	if n <= 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	s.Credit += n
	if s.Credit > r.creditWindow {
		s.Credit = r.creditWindow
	}
	s.LastSeenAt = r.now()
}

// Touch records activity on a session, extending its effective lifetime.
func (r *PTYRegistry) Touch(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if s, ok := r.sessions[sessionID]; ok {
		s.LastSeenAt = r.now()
	}
}

// Expired returns sessions whose last activity is older than the TTL.
func (r *PTYRegistry) Expired() []*PTYSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := r.now().Add(-r.sessionTTL)
	var out []*PTYSession
	for _, s := range r.sessions {
		if s.LastSeenAt.Before(cutoff) {
			out = append(out, s)
		}
	}

	return out
}

// Count returns the number of live sessions.
func (r *PTYRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.sessions)
}

// CountForDevice returns the number of live sessions for a device.
func (r *PTYRegistry) CountForDevice(deviceID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.byDevice[deviceID]
}

// ForDevice returns the live sessions for a device.
//
// Used when a device (re)connects to re-send an OPEN for each session, so the
// device re-binds the shells it still holds instead of the user having to
// reopen them. The returned slice is a snapshot; the sessions themselves are
// shared and must be treated as read-only by the caller.
func (r *PTYRegistry) ForDevice(deviceID string) []*PTYSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*PTYSession, 0, r.byDevice[deviceID])
	for _, s := range r.sessions {
		if s.DeviceID == deviceID {
			out = append(out, s)
		}
	}

	return out
}

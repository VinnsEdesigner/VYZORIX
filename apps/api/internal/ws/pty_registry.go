// Package hub provides WebSocket functionality.
package hub

import (
	"errors"
	"sync"
	"time"
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
)

// PTY session limits. These bound memory and blast radius per device.
const (
	// DefaultPTYMaxSessionsPerDevice caps concurrent sessions on one device.
	DefaultPTYMaxSessionsPerDevice = 4

	// DefaultPTYCreditWindow is how many unacknowledged data frames may be in
	// flight to the operator before the device's output is throttled. Without a
	// window, `cat` of a large file would flood the socket and the browser.
	DefaultPTYCreditWindow = 64

	// DefaultPTYSessionTTL bounds a session's lifetime even if neither side
	// closes it, so a forgotten tab cannot hold a device open forever.
	DefaultPTYSessionTTL = 2 * time.Hour
)

// PTYSession is the in-memory state of one terminal session. It is deliberately
// not durable: a session is a live connection between an operator and a device,
// and it is meaningless once the server restarts.
type PTYSession struct {
	OpenedAt   time.Time
	LastSeenAt time.Time
	recorder   *PTYRecorder
	SessionID  string
	DeviceID   string
	OperatorID string
	OrgID      string
	Recording  bool
	Credit     int
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
// when the device already holds the maximum, so one operator cannot open an
// unbounded number of terminals on one device.
func (r *PTYRegistry) Open(sessionID, deviceID, operatorID, orgID string, recording bool) (*PTYSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byDevice[deviceID] >= r.maxPerDevice {
		return nil, ErrPTYTooManySessions
	}

	// Session IDs are client-supplied. Refusing a duplicate prevents two failure
	// modes: overwriting a live entry (leaking the first session, which would
	// never be closed and would keep counting against the device cap) and letting
	// one operator adopt another's session by guessing its ID.
	if _, exists := r.sessions[sessionID]; exists {
		return nil, ErrPTYSessionExists
	}

	now := r.now()
	s := &PTYSession{
		SessionID:  sessionID,
		DeviceID:   deviceID,
		OperatorID: operatorID,
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
// ErrPTYNoCredit when the operator has not acknowledged enough frames, which is
// how backpressure is applied to a device that is producing faster than the
// operator can consume.
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
// hostile operator cannot inflate it. An unknown session is a no-op: the
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

package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a command is not found.
var ErrNotFound = errors.New("command not found")

// ErrInvalidCommandTransition is returned when an invalid command status transition is attempted.
var ErrInvalidCommandTransition = errors.New("invalid command status transition")

// Status represents the status of a command.
type Status string

const (
	// StatusPending indicates the command is waiting to be processed.
	StatusPending Status = "pending"
	// StatusDelivered indicates the command has been delivered to the device.
	StatusDelivered Status = "delivered"
	// StatusCompleted indicates the command has been completed.
	StatusCompleted Status = "completed"
	// StatusFailed indicates the command has failed.
	StatusFailed Status = "failed"
	// StatusCancelled indicates the command was cancelled.
	StatusCancelled Status = "cancelled"
)

// CommandStatusTransitions defines valid state transitions for command lifecycle.
// The map key is the current state, and the value is the set of allowed next states.
var CommandStatusTransitions = map[Status]map[Status]bool{
	StatusPending: {
		StatusDelivered: true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusDelivered: {
		StatusCompleted: true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusCompleted: {}, // Terminal state.
	StatusFailed:    {}, // Terminal state.
	StatusCancelled: {}, // Terminal state.
}

// CanTransitionTo returns true if the status can transition to the target status.
func (s Status) CanTransitionTo(target Status) bool {
	allowed, exists := CommandStatusTransitions[s]
	if !exists {
		return false
	}
	return allowed[target]
}

// TransitionTo transitions the status to a new state.
// Returns ErrInvalidCommandTransition if the transition is not allowed.
func (s *Status) TransitionTo(target Status) error {
	if !s.CanTransitionTo(target) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidCommandTransition, *s, target)
	}
	*s = target
	return nil
}

// IsPending checks if the command is pending.
func (s Status) IsPending() bool {
	return s == StatusPending
}

// IsDelivered checks if the command has been delivered.
func (s Status) IsDelivered() bool {
	return s == StatusDelivered
}

// IsCompleted checks if the command has been completed.
func (s Status) IsCompleted() bool {
	return s == StatusCompleted
}

// IsFailed checks if the command has failed.
func (s Status) IsFailed() bool {
	return s == StatusFailed
}

// IsCancelled checks if the command was cancelled.
func (s Status) IsCancelled() bool {
	return s == StatusCancelled
}

// IsTerminal checks if the status is a terminal state.
func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

// IsActive checks if the command is in an active state (pending or delivered).
func (s Status) IsActive() bool {
	return s == StatusPending || s == StatusDelivered
}

// Command type constants.
const (
	// TypeWakeUpUpdater is sent to wake a device to check for updates.
	TypeWakeUpUpdater = "WAKE_UP_UPDATER"
	// TypeCheckUpdate is sent to trigger an update check on a device.
	TypeCheckUpdate = "CHECK_UPDATE"

	// TypeShellExec runs a single command on the device and returns its output.
	// It is stateless: one command in, one result out, no session. Use the PTY
	// session flow (see the operator terminal endpoint) for an interactive shell.
	//
	// There is deliberately no server-side handler for this type, and adding one
	// would be a design error. The server is a command plane, not an executor:
	// its job for every command type is the same four steps — authorize against
	// the risk profile, sign the frame with the device secret, deliver it over
	// the device WebSocket (or FCM wake), and record the result the device
	// reports back. Executing the command is the device agent's job, on the
	// device, under its own sandbox. A server-side handler would mean the server
	// ran shell commands, which is neither possible (it has no access to the
	// device) nor desirable (it would bypass the device's own privilege model).
	//
	// So this constant exists to be *classified* and *dispatched*, not executed:
	// risk_catalog.go gives it a tier so the risk gate knows how to authorize it,
	// and the command plane carries it to the device like any other type. The
	// device agent is what runs the command and reports the outcome, which then
	// flows back through the command_result path. Until that agent implements it,
	// a device that receives SHELL_EXEC should report a failed result rather than
	// ignore it, so the caller learns the command was not executed.
	TypeShellExec = "SHELL_EXEC"
	// TypePTYSessionOpen requests an interactive terminal session on the device.
	TypePTYSessionOpen = "PTY_SESSION_OPEN"
	// TypeUsbAttach grants a USB session on the device's OTG bus.
	TypeUsbAttach = "USB_ATTACH"
	// TypeAppStart starts a supervised application on the device.
	TypeAppStart = "APP_START"
	// TypeAppStop stops a supervised application on the device.
	TypeAppStop = "APP_STOP"
)

// Command represents a command to be sent to a device.
type Command struct {
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeliveredAt   *int64
	CompletedAt   *int64
	ExpiresAt     *time.Time
	NextRetryAt   *time.Time
	DispatchID    string
	Command       string
	Status        Status
	FailureReason string
	Result        string
	DeviceID      string
	ID            string
	Args          []byte
	RetryCount    int
	MaxRetries    int
}

// IsExpired returns true if the command has expired based on its TTL.
func (c *Command) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false // No TTL set, never expires.
	}
	return time.Now().After(*c.ExpiresAt)
}

// CommandFrame is the internal representation of a command for the WebSocket hub.
type CommandFrame struct {
	DeliveryConfirmation chan<- bool     `json:"-"`
	Type                 string          `json:"type"`
	DispatchID           string          `json:"dispatchId"`
	Command              string          `json:"command"`
	Nonce                string          `json:"nonce"`
	Signature            string          `json:"signature,omitempty"`
	Args                 json.RawMessage `json:"args,omitempty"`
	Timestamp            int64           `json:"timestamp"`
}

// IsPending returns true if the command is pending.
func (c *Command) IsPending() bool {
	return c.Status.IsPending()
}

// IsDelivered returns true if the command has been delivered.
func (c *Command) IsDelivered() bool {
	return c.Status.IsDelivered()
}

// IsCompleted returns true if the command has been completed.
func (c *Command) IsCompleted() bool {
	return c.Status.IsCompleted()
}

// IsFailed returns true if the command has failed.
func (c *Command) IsFailed() bool {
	return c.Status.IsFailed()
}

// IsFinished returns true if the command is in a terminal state.
func (c *Command) IsFinished() bool {
	return c.Status.IsTerminal()
}

// DeliveredAtTime returns the DeliveredAt as a time.Time.
func (c *Command) DeliveredAtTime() *time.Time {
	if c.DeliveredAt == nil {
		return nil
	}

	t := time.UnixMilli(*c.DeliveredAt)

	return &t
}

// CompletedAtTime returns the CompletedAt as a time.Time.
func (c *Command) CompletedAtTime() *time.Time {
	if c.CompletedAt == nil {
		return nil
	}

	t := time.UnixMilli(*c.CompletedAt)

	return &t
}

// SetArgs sets the arguments from a struct (JSON encoded).
func (c *Command) SetArgs(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	c.Args = data

	return nil
}

// GetArgs unmarshals the arguments into a struct.
func (c *Command) GetArgs(v interface{}) error {
	return json.Unmarshal(c.Args, v)
}

// CanRetry returns true if the command can be retried.
func (c *Command) CanRetry() bool {
	return c.MaxRetries == 0 || c.RetryCount < c.MaxRetries
}

// ApplyResult records a device-reported terminal outcome. It enforces the
// lifecycle transition (delivered/pending -> completed/failed) so a late or
// duplicated device report cannot resurrect a command that already finished,
// and it stores the device output separately from the failure reason.
//
// A successful report clears any earlier failure reason; a failed report clears
// any earlier result. Both are idempotent: re-applying the same terminal status
// returns (false, nil) rather than an error, so a retried delivery confirmation
// is safe. It returns true only when the status actually changed.
func (c *Command) ApplyResult(success bool, output string, at time.Time) (bool, error) {
	target := StatusCompleted
	if !success {
		target = StatusFailed
	}

	if c.Status == target {
		return false, nil
	}
	if c.Status.IsTerminal() {
		return false, fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidCommandTransition, c.Status, target)
	}

	c.Status = target
	ms := at.UnixMilli()
	c.CompletedAt = &ms
	c.UpdatedAt = at

	if success {
		c.Result = output
		c.FailureReason = ""
	} else {
		c.FailureReason = output
	}

	return true, nil
}

// ShouldRetryNow returns true if enough time has passed since the last attempt.
func (c *Command) ShouldRetryNow() bool {
	if c.NextRetryAt == nil {
		return true // First attempt or no backoff set.
	}
	return time.Now().After(*c.NextRetryAt)
}

// IncrementRetry sets up the command for the next retry attempt.
// Returns false if max retries exceeded.
func (c *Command) IncrementRetry(baseDelay time.Duration) bool {
	c.RetryCount++
	if c.MaxRetries > 0 && c.RetryCount >= c.MaxRetries {
		return false
	}
	// Exponential backoff: baseDelay * 2^(retryCount-1).
	delay := baseDelay * time.Duration(1<<(c.RetryCount-1))
	next := time.Now().Add(delay)
	c.NextRetryAt = &next
	return true
}

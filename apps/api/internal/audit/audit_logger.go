// Package audit provides audit logging functionality for security events.
package audit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/uuid"
)

// AuditLogger defines the interface for audit logging operations.
type AuditLogger interface {
	APIKeyCreated(ctx context.Context, userID, keyID, keyName, keyPrefix, scope, ipAddress, userAgent string)
	APIKeyUpdated(ctx context.Context, userID, keyID, keyName, changes, ipAddress, userAgent string)
	APIKeyRevoked(ctx context.Context, userID, keyID, keyName, ipAddress, userAgent string)
	APIKeyRotated(ctx context.Context, userID, keyID, keyName, ipAddress, userAgent string)
	APIKeyFailed(ctx context.Context, userID, keyPrefix, ipAddress, userAgent, reason string)
}

const (
	// auditLogBufferSize is the size of the audit event channel buffer.
	// Events beyond this buffer will block until the writer processes them.
	auditLogBufferSize = 10000
)

// Logger handles logging of security events to the audit repository.

// fire-and-forget goroutines per event, ensuring events are not dropped on shutdown.

type Logger struct {
	separateRepo interface {
		Log(context.Context, *Entry) error
	}
	repo   *Repository
	log    *slog.Logger
	done   chan struct{}
	events chan *Entry
	wg     sync.WaitGroup
}

// Compile-time check that Logger implements AuditLogger.
var _ AuditLogger = (*Logger)(nil)

// NoOpLogger is a no-operation audit logger for testing and environments.
// without an audit store. It satisfies AuditLogger and the command-handler's.
// auditLogger interface with zero-cost no-op methods.
type NoOpLogger struct{}

// NewNoOpLogger returns a ready-to-use NoOpLogger.
func NewNoOpLogger() *NoOpLogger { return &NoOpLogger{} }

func (n *NoOpLogger) APIKeyCreated(ctx context.Context, userID, keyID, keyName, keyPrefix, scope, ipAddress, userAgent string) {
}
func (n *NoOpLogger) APIKeyUpdated(ctx context.Context, userID, keyID, keyName, changes, ipAddress, userAgent string) {
}
func (n *NoOpLogger) APIKeyRevoked(ctx context.Context, userID, keyID, keyName, ipAddress, userAgent string) {
}
func (n *NoOpLogger) APIKeyRotated(ctx context.Context, userID, keyID, keyName, ipAddress, userAgent string) {
}
func (n *NoOpLogger) APIKeyFailed(ctx context.Context, userID, keyPrefix, ipAddress, userAgent, reason string) {
}

// CommandExecuted is a no-op for the command execution audit event.
func (n *NoOpLogger) CommandExecuted(ctx context.Context, e CommandExecutedEvent) {}

// PTYSession is a no-op for the terminal session audit event.
func (n *NoOpLogger) PTYSession(_ context.Context, _ PTYSessionEvent) {}

// NoOpAuditRepo is a no-operation audit repository for when no separate DB is configured.
type NoOpAuditRepo struct{}

func (n *NoOpAuditRepo) Log(ctx context.Context, entry *Entry) error { return nil }

// Compile-time check that NoOpAuditRepo implements the interface.
var _ interface {
	Log(context.Context, *Entry) error
} = (*NoOpAuditRepo)(nil)

// LoggerConfig holds configuration for the audit logger.
type LoggerConfig struct {
	SeparateDBPath string
	RetentionDays  int
	Enabled        bool
	SeparateDB     bool
}

// DefaultLoggerConfig returns the default audit logger configuration.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Enabled:       true,
		RetentionDays: 90,
		SeparateDB:    false, // Default to co-located for backward compatibility.
	}
}

// NewLogger creates a new audit logger with a single writer goroutine.

func NewLogger(repo *Repository, log *slog.Logger, cfg LoggerConfig, separateRepo interface {
	Log(context.Context, *Entry) error
}) *Logger {
	l := &Logger{
		repo:         repo,
		log:          log,
		done:         make(chan struct{}),
		events:       make(chan *Entry, auditLogBufferSize),
		separateRepo: separateRepo,
	}

	// Start single writer goroutine that drains the channel.
	l.wg.Add(1)
	go l.writer()

	return l
}

// writer is the single goroutine that processes audit events from the channel.
// and writes them to the repository.
func (l *Logger) writer() {
	defer l.wg.Done()

	for {
		select {
		case entry := <-l.events:
			if entry == nil {
				// Channel closed, flush remaining events.
				for entry := range l.events {
					l.writeEntry(entry)
				}
				return
			}
			l.writeEntry(entry)
		case <-l.done:
			// Shutdown requested, flush remaining events.
			close(l.events)
			for entry := range l.events {
				l.writeEntry(entry)
			}
			return
		}
	}
}

// writeEntry writes a single audit entry to the repository.

func (l *Logger) writeEntry(entry *Entry) {
	// Write to primary repository.
	if err := l.repo.Log(context.Background(), entry); err != nil {
		l.log.Error("failed to write audit log",
			slog.String("action", string(entry.Action)),
			slog.String("error", err.Error()))
	}

	if l.separateRepo != nil {
		if err := l.separateRepo.Log(context.Background(), entry); err != nil {
			l.log.Error("failed to write audit log to separate DB",
				slog.String("action", string(entry.Action)),
				slog.String("error", err.Error()))
		}
	}
}

// LogEvent logs a security event to the audit repository via buffered channel.

// writer goroutine, ensuring proper shutdown drainage.
// CountForUser returns how many audit entries the user has, i.e. how many
// audited actions they have taken. A read-only convenience over the repository
// so a profile summary can show an activity count without the caller reaching
// for the repository directly. A nil repo (no audit store configured) counts 0.
func (l *Logger) CountForUser(ctx context.Context, userID string) (int, error) {
	if l.repo == nil {
		return 0, nil
	}

	return l.repo.Count(ctx, Query{UserID: userID})
}

func (l *Logger) LogEvent(ctx context.Context, entry *Entry) {
	if entry.ID == "" {
		entry.ID = uuid.New()
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	select {
	case l.events <- entry:
		// Event queued successfully.
	case <-l.done:
		// Logger is shutting down, log directly (may block briefly).
		l.writeEntry(entry)
	default:
		// Buffer full - log and warn that buffer is saturated.
		l.log.Warn("audit log buffer full, logging synchronously",
			slog.String("action", string(entry.Action)))
		l.writeEntry(entry)
	}
}

// Shutdown gracefully shuts down the audit logger, ensuring all pending events.
// are written before returning. Call this during application shutdown.

func (l *Logger) Shutdown(ctx context.Context) error {
	close(l.done)

	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// LoginSuccess logs a successful login event.
func (l *Logger) LoginSuccess(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionLoginSuccess,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
	})
}

// LoginFailed logs a failed login attempt.
func (l *Logger) LoginFailed(ctx context.Context, userID, ipAddress, userAgent, reason string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionLoginFailed,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultFailure,
		Metadata:   map[string]string{"reason": reason},
	})
}

// Logout logs a logout event.
func (l *Logger) Logout(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionLogout,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
	})
}

// Register logs a registration event.
func (l *Logger) Register(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionRegister,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
	})
}

// PasswordChange logs a password change event.
func (l *Logger) PasswordChange(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionPasswordChange,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
	})
}

// AccountLocked logs an account lockout event.
func (l *Logger) AccountLocked(ctx context.Context, userID, ipAddress string, attempts int) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionAccountLocked,
		IPAddress:  ipAddress,
		Result:     ResultBlocked,
		Metadata:   map[string]string{"attempts": fmt.Sprintf("%d", attempts)},
	})
}

// SessionRevoked logs a session revocation event.
func (l *Logger) SessionRevoked(ctx context.Context, userID, ipAddress, userAgent, reason string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionSessionRevoked,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
		Metadata:   map[string]string{"reason": reason},
	})
}

// CSRFFailure logs a CSRF validation failure.
func (l *Logger) CSRFFailure(ctx context.Context, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		Action:    ActionCSRFFailure,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    ResultBlocked,
	})
}

// SigningFailure logs a request signing validation failure.
func (l *Logger) SigningFailure(ctx context.Context, ipAddress, userAgent, reason string) {
	l.LogEvent(ctx, &Entry{
		Action:    ActionSigningFailure,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    ResultBlocked,
		Metadata:  map[string]string{"reason": reason},
	})
}

// RateLimitExceeded logs a rate limit exceeded event.
func (l *Logger) RateLimitExceeded(ctx context.Context, ipAddress, userAgent, endpoint string) {
	l.LogEvent(ctx, &Entry{
		Action:       ActionRateLimitExceeded,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		ResourceType: "endpoint",
		ResourceID:   endpoint,
		Result:       ResultBlocked,
	})
}

// APIClientCreated logs an API client creation event.
func (l *Logger) APIClientCreated(ctx context.Context, userID, clientID, ipAddress string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIClientCreated,
		ResourceType: "api_client",
		ResourceID:   clientID,
		IPAddress:    ipAddress,
		Result:       ResultSuccess,
	})
}

// APIClientRevoked logs an API client revocation event.
func (l *Logger) APIClientRevoked(ctx context.Context, userID, clientID, ipAddress string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIClientRevoked,
		ResourceType: "api_client",
		ResourceID:   clientID,
		IPAddress:    ipAddress,
		Result:       ResultSuccess,
	})
}

// APIClientSecretRotated logs an API client secret rotation event.
func (l *Logger) APIClientSecretRotated(ctx context.Context, userID, clientID, ipAddress string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIClientSecretRotated,
		ResourceType: "api_client",
		ResourceID:   clientID,
		IPAddress:    ipAddress,
		Result:       ResultSuccess,
	})
}

// SigningKeyRotated logs a signing key rotation event.
func (l *Logger) SigningKeyRotated(ctx context.Context, userID, clientID, ipAddress string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionSigningKeyRotated,
		ResourceType: "signing_key",
		ResourceID:   clientID,
		IPAddress:    ipAddress,
		Result:       ResultSuccess,
	})
}

// AdminAction logs an admin action event.
func (l *Logger) AdminAction(ctx context.Context, userID, action, resourceType, resourceID, ipAddress string, metadata map[string]string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAdminAction,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    ipAddress,
		Result:       ResultSuccess,
		Metadata:     metadata,
	})
}

// UpdatePushed logs an update push event.
func (l *Logger) UpdatePushed(ctx context.Context, userID, pushID, version string, deviceCount int, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionUpdatePushed,
		ResourceType: "update_push",
		ResourceID:   pushID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Result:       ResultSuccess,
		Metadata: map[string]string{
			"version":      version,
			"device_count": fmt.Sprintf("%d", deviceCount),
		},
	})
}

// UpdateCancelled logs an update cancellation event.
func (l *Logger) UpdateCancelled(ctx context.Context, userID, pushID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionUpdateCancelled,
		ResourceType: "update_push",
		ResourceID:   pushID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Result:       ResultSuccess,
	})
}

// UpdateSyncStarted logs an update sync start event.
func (l *Logger) UpdateSyncStarted(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionUpdateSyncStarted,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
	})
}

// UpdateSyncFailed logs an update sync failure event.
func (l *Logger) UpdateSyncFailed(ctx context.Context, userID, ipAddress, userAgent, reason string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionUpdateSyncFailed,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultFailure,
		Metadata:   map[string]string{"reason": reason},
	})
}

// SettingsChanged logs a settings change event.
func (l *Logger) SettingsChanged(ctx context.Context, userID, section, action, ipAddress, userAgent string, metadata map[string]string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionSettingsChanged,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
		Metadata: map[string]string{
			"section": section,
			"action":  action,
		},
	})
}

// WebhookTest logs a webhook test event.
func (l *Logger) WebhookTest(ctx context.Context, userID, url, ipAddress, userAgent string, success bool, responseTimeMs int64) {
	result := ResultSuccess
	if !success {
		result = ResultFailure
	}
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionWebhookTest,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     result,
		Metadata: map[string]string{
			"webhook_url":      url,
			"response_time_ms": fmt.Sprintf("%d", responseTimeMs),
		},
	})
}

// WebhookSecretRotated logs a webhook secret rotation event.
func (l *Logger) WebhookSecretRotated(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionWebhookSecretRotated,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultSuccess,
	})
}

// MFAVerifyAttempt logs an MFA verification attempt.
// 2: Added for audit compliance.
func (l *Logger) MFAVerifyAttempt(ctx context.Context, userID, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionMFAVerifyAttempt,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultPending,
	})
}

// MFAVerifySuccess logs a successful MFA verification.
// 2: Added for audit compliance.
func (l *Logger) MFAVerifySuccess(ctx context.Context, userID, sessionID, ipAddress string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionMFAVerifySuccess,
		IPAddress:  ipAddress,
		Result:     ResultSuccess,
		Metadata:   map[string]string{"session_id": sessionID},
	})
}

// MFAVerifyFailed logs a failed MFA verification attempt.
// 2: Added for audit compliance.
func (l *Logger) MFAVerifyFailed(ctx context.Context, userID, ipAddress string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionMFAVerifyFailed,
		IPAddress:  ipAddress,
		Result:     ResultFailure,
	})
}

// APIKeyCreated logs an API key creation event.
func (l *Logger) APIKeyCreated(ctx context.Context, userID, keyID, keyName, keyPrefix, scope, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIKeyCreated,
		ResourceType: "api_key",
		ResourceID:   keyID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Result:       ResultSuccess,
		Metadata: map[string]string{
			"key_name":   keyName,
			"key_prefix": keyPrefix,
			"scope":      scope,
		},
	})
}

// APIKeyUpdated logs an API key update event.
func (l *Logger) APIKeyUpdated(ctx context.Context, userID, keyID, keyName, changes, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIKeyUpdated,
		ResourceType: "api_key",
		ResourceID:   keyID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Result:       ResultSuccess,
		Metadata: map[string]string{
			"key_name": keyName,
			"changes":  changes,
		},
	})
}

// APIKeyRevoked logs an API key revocation event.
func (l *Logger) APIKeyRevoked(ctx context.Context, userID, keyID, keyName, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIKeyRevoked,
		ResourceType: "api_key",
		ResourceID:   keyID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Result:       ResultSuccess,
		Metadata: map[string]string{
			"key_name": keyName,
		},
	})
}

// APIKeyRotated logs an API key rotation event.
func (l *Logger) APIKeyRotated(ctx context.Context, userID, keyID, keyName, ipAddress, userAgent string) {
	l.LogEvent(ctx, &Entry{
		UserID:   userID,
		Action:       ActionAPIKeyRotated,
		ResourceType: "api_key",
		ResourceID:   keyID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Result:       ResultSuccess,
		Metadata: map[string]string{
			"key_name": keyName,
		},
	})
}

// APIKeyFailed logs a failed API key authentication attempt.
func (l *Logger) APIKeyFailed(ctx context.Context, userID, keyPrefix, ipAddress, userAgent, reason string) {
	l.LogEvent(ctx, &Entry{
		UserID: userID,
		Action:     ActionAPIKeyFailed,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     ResultFailure,
		Metadata: map[string]string{
			"key_prefix": keyPrefix,
			"reason":     reason,
		},
	})
}

// CommandExecuted logs a device command execution attempt together with its.
// risk classification and the trace id of the originating request. Used by the.
// command execution path to leave an auditable trail for every command —.
// allowed, confirmation-gated, or blocked — so risky operations are traceable.
func (l *Logger) CommandExecuted(ctx context.Context, e CommandExecutedEvent) {
	metadata := map[string]string{
		"command":     e.Command,
		"device_id":   e.DeviceID,
		"dispatch_id": e.DispatchID,
	}
	if e.Reason != "" {
		metadata["reason"] = e.Reason
	}

	l.LogEvent(ctx, &Entry{
		UserID:   e.UserID,
		Action:       ActionCommandExecuted,
		ResourceType: "device",
		ResourceID:   e.DeviceID,
		IPAddress:    e.IPAddress,
		UserAgent:    e.UserAgent,
		Result:       e.Result,
		TraceID:      e.TraceID,
		RiskTier:     e.RiskTier,
		ActorType:    e.ActorType,
		ActorEmail:   e.ActorEmail,
		OldValue:     e.OldValue,
		NewValue:     e.NewValue,
		Metadata:     metadata,
	})
}

// CommandExecutedEvent carries the fields needed to audit a command execution.
// It is a value type so handlers assemble it without allocations when there is.
// nothing to record.
type CommandExecutedEvent struct {
	UserID string
	DeviceID   string
	Command    string
	DispatchID string
	IPAddress  string
	UserAgent  string
	TraceID    string
	RiskTier   string
	Result     Result
	Reason     string
	ActorType  string
	ActorEmail string
	OldValue   string
	NewValue   string
}

// PTYSession logs a terminal session lifecycle event: opened, closed, or denied.
// Opening a shell is the most privileged operation the platform exposes, so it is
// audited separately from command execution and records who opened it, on which
// device, and why it ended.
func (l *Logger) PTYSession(ctx context.Context, e PTYSessionEvent) {
	metadata := map[string]string{
		"session_id": e.SessionID,
		"device_id":  e.DeviceID,
	}
	if e.Reason != "" {
		metadata["reason"] = e.Reason
	}
	if e.Recording {
		metadata["recording"] = "true"
	}

	action := ActionPTYSessionOpened
	if !e.Opened {
		action = ActionPTYSessionClosed
	}

	l.LogEvent(ctx, &Entry{
		UserID:   e.UserID,
		Action:       action,
		ResourceType: "device",
		ResourceID:   e.DeviceID,
		IPAddress:    e.IPAddress,
		UserAgent:    e.UserAgent,
		Result:       e.Result,
		// A terminal is always treated as critical-tier for audit purposes: it is
		// unbounded code execution, whether or not the caller reached it through
		// the confirmation flow.
		RiskTier: "critical",
		Metadata: metadata,
	})
}

// PTYSessionEvent carries the fields needed to audit a terminal session. It is a
// value type so handlers assemble it without allocating when there is nothing to
// record.
type PTYSessionEvent struct {
	SessionID  string
	DeviceID   string
	UserID string
	Reason     string
	IPAddress  string
	UserAgent  string
	Result     Result
	// Opened distinguishes an open from a close for the same struct.
	Opened bool
	// Recording reports whether the session is being recorded.
	Recording bool
}

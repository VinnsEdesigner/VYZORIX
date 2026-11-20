// Package logging provides structured logging with PII redaction.
package logging

import (
	"context"
	"log/slog"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
	"os"
	"strings"
)

// NewAuditFileLogger creates a dedicated logger that writes audit events to a separate file.
// Audit logs are always JSON formatted for machine parsing and are never redacted.
// (unlike application logs which may redact PII).
func NewAuditFileLogger(path string) *slog.Logger {
	if path == "" {
		path = "audit.log"
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		// Fall back to stdout if file cannot be opened.
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// LoggerConfig holds logging configuration.
type LoggerConfig struct {
	Level string
	// ExtraRedactKeys are additional key fragments to mask, on top of the shared
	// sensitive-key rule. It is additive on purpose: a caller can widen coverage but
	// cannot narrow it, so no configuration can silently stop a credential being masked.
	ExtraRedactKeys []string
	RedactPII       bool
}

// DefaultLoggerConfig returns the default logging configuration.
//
// No extra keys: the shared rule in redaction.IsSensitiveKey already covers passwords,
// secrets, tokens, API keys, credentials, private keys, sessions, cookies, DSNs and
// webhook URLs. Duplicating that list here is what let the copies drift apart.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:     "info",
		RedactPII: true,
	}
}

// NewLogger creates a new structured logger with PII redaction.
func NewLogger(cfg LoggerConfig) *slog.Logger {
	var level slog.Level

	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if os.Getenv("ENV") == "production" || os.Getenv("NODE_ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(&redactingHandler{
		Handler:   handler,
		extraKeys: cfg.ExtraRedactKeys,
		enabled:   cfg.RedactPII,
	})
}

// redactingHandler wraps a slog handler to redact PII from log output.
type redactingHandler struct {
	slog.Handler
	extraKeys []string
	enabled   bool
}

// Handle implements slog.Handler, redacting PII before logging.
func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.enabled {
		r = h.redactRecord(r)
	}

	return h.Handler.Handle(ctx, r)
}

// redactRecord returns a record whose attributes are redacted.
//
// It must RETURN the record. slog.Record is a value type, so appending to a copy and
// then logging the original leaves every raw attribute in place — redaction that appears
// wired but does nothing. The rebuild starts from a fresh record because slog offers no
// way to remove or replace an attribute in place.
func (h *redactingHandler) redactRecord(r slog.Record) slog.Record {
	redactor := redaction.DefaultRedactor

	out := slog.NewRecord(r.Time, r.Level, redactor.Redact(r.Message), r.PC)

	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(h.redactAttr(a, redactor))

		return true
	})

	return out
}

// extraKey reports whether a key matches one of the caller's additional fragments.
func (h *redactingHandler) extraKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range h.extraKeys {
		if strings.Contains(lower, strings.ToLower(fragment)) {
			return true
		}
	}

	return false
}

// redactAttr redacts one attribute: by key name first, then by value content.
func (h *redactingHandler) redactAttr(a slog.Attr, redactor *redaction.Redactor) slog.Attr {
	// A value that knows how to mask itself wins: it is the strongest signal.
	if _, ok := a.Value.Any().(interface{ LogValue() slog.Value }); ok {
		return a
	}

	// The shared rule covers every credential family in one place; the handler's own
	// list is an additive extra, so the two cannot disagree about what a secret is.
	if redaction.IsSensitiveKey(a.Key) || h.extraKey(a.Key) {
		return slog.String(a.Key, redaction.Marker)
	}

	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, redactor.Redact(a.Value.String()))
	case slog.KindGroup:
		group := a.Value.Group()
		redacted := make([]any, 0, len(group)*2)
		for _, sub := range group {
			sub = h.redactAttr(sub, redactor)
			redacted = append(redacted, sub)
		}

		return slog.Group(a.Key, redacted...)
	default:
		return a
	}
}

// NewProductionLogger creates a logger optimized for production.
//
// It sets no extra keys: redaction.IsSensitiveKey already covers passwords, secrets,
// tokens, API keys, credentials, private keys, sessions, cookies, DSNs and webhook URLs.
// Restating the list here is what let three copies drift apart.
func NewProductionLogger() *slog.Logger {
	return NewLogger(LoggerConfig{
		Level:     "info",
		RedactPII: true,
	})
}

// NewDevelopmentLogger creates a logger for development.
func NewDevelopmentLogger() *slog.Logger {
	return NewLogger(LoggerConfig{
		Level:     "debug",
		RedactPII: false,
	})
}

// NewFromEnv creates a logger based on the ENV environment variable.
func NewFromEnv() *slog.Logger {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("NODE_ENV")
	}

	if env == "production" {
		return NewProductionLogger()
	}

	return NewDevelopmentLogger()
}

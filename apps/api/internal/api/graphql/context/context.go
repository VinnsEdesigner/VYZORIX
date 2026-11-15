// Package gqlcontext provides GraphQL context utilities.
package gqlcontext

import (
	"context"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

type contextKey string

const (
	userKey       contextKey = "user"
	metadataKey       contextKey = "requestMetadata"
	organizationIDKey contextKey = "organizationID"
	mfaVerifiedKey    contextKey = "mfaVerified"
)

// RequestMetadata holds request metadata.
type RequestMetadata struct {
	ClientIP  string
	UserAgent string
}

// WithUser adds the user to the context.
func WithUser(ctx context.Context, op *user.User) context.Context {
	return context.WithValue(ctx, userKey, op)
}

// GetUser retrieves the user from context.
func GetUser(ctx context.Context) (*user.User, bool) {
	val := ctx.Value(userKey)
	if val == nil {
		return nil, false
	}

	op, ok := val.(*user.User)

	return op, ok
}

// WithRequestMetadata adds request metadata to the context.
func WithRequestMetadata(ctx context.Context, clientIP, userAgent string) context.Context {
	return context.WithValue(ctx, metadataKey, RequestMetadata{
		ClientIP:  clientIP,
		UserAgent: userAgent,
	})
}

// GetRequestMetadata retrieves the request metadata from context.
func GetRequestMetadata(ctx context.Context) (RequestMetadata, bool) {
	val := ctx.Value(metadataKey)
	if val == nil {
		return RequestMetadata{}, false
	}

	meta, ok := val.(RequestMetadata)

	return meta, ok
}

// GetClientIP retrieves the client IP from context.
func GetClientIP(ctx context.Context) string {
	if meta, ok := GetRequestMetadata(ctx); ok {
		return meta.ClientIP
	}

	return ""
}

// GetUserAgent retrieves the user agent from context.
func GetUserAgent(ctx context.Context) string {
	if meta, ok := GetRequestMetadata(ctx); ok {
		return meta.UserAgent
	}

	return ""
}

// WithOrganizationID adds the organization ID to the context.
func WithOrganizationID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, organizationIDKey, orgID)
}

// GetOrganizationID retrieves the organization ID from context.
func GetOrganizationID(ctx context.Context) string {
	val := ctx.Value(organizationIDKey)
	if val == nil {
		return ""
	}

	orgID, ok := val.(string)
	if !ok {
		return ""
	}

	return orgID
}

// WithMFAVerified records whether the authenticated session has completed MFA.
// Critical-tier commands require this; the risk gate reads it via GetMFAVerified.
func WithMFAVerified(ctx context.Context, verified bool) context.Context {
	return context.WithValue(ctx, mfaVerifiedKey, verified)
}

// GetMFAVerified reports whether the session is MFA-verified (false if unset).
func GetMFAVerified(ctx context.Context) bool {
	val := ctx.Value(mfaVerifiedKey)
	if val == nil {
		return false
	}
	verified, ok := val.(bool)
	return ok && verified
}

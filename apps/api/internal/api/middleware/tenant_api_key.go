package middleware

import (
	"context"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/serviceaccount"
	"net/http"
	"strings"

	svcapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/serviceaccount"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	keys "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/keys"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/gin-gonic/gin"
)

// PathType defines the authentication boundary for a path.
type PathType int

const (
	PathTypeUnknown        PathType = iota
	PathTypePublic                  // No auth required.
	PathTypeInfrastructure          // Env API Key (ServerAPIToken).
	PathTypeSessionOnly             // Session Cookie required.
	PathTypeDeviceAuth              // HMAC Signature.
	PathTypeTenant                  // Session OR API Key + Scope.
)

// PathBoundary maps path patterns to their authentication requirements.
var PathBoundaries = map[string]PathType{
	// PUBLIC - No auth required.
	"/health":   PathTypePublic,
	"/v1/auth/": PathTypePublic,
	// DEPRECATED: "/v1/device/register":  PathTypePublic // Use /v1/device/inbox,.
	"/v1/device/inbox": PathTypePublic,
	"/v1/device/":      PathTypePublic, // /v1/device/:imei/status - device status check.
	"/metrics":         PathTypePublic, // Prometheus scraping.

	// INFRASTRUCTURE - ServerAPIToken (env var) - handled at route level.
	// /admin/*, /internal/*, /healthz.

	// SESSION ONLY - Session Cookie required (no API key).
	"/bin/":         PathTypeSessionOnly,
	"/v1/api-keys/": PathTypeSessionOnly,
	"/api/v1/apk/":  PathTypeSessionOnly,

	// TENANT - Session OR API Key + Scope.
	"/v1/dashboard/":          PathTypeTenant,
	"/v1/devices/":            PathTypeTenant,
	"/v1/command/":            PathTypeTenant,
	"/v1/telemetry/":          PathTypeTenant,
	"/v1/updates/":            PathTypeTenant,
	"/v1/connections/":        PathTypeTenant,
	"/v1/device/diagnostics/": PathTypeTenant,
}

// ClassifyPath determines the PathType for a given path.
// It checks exact matches first, then prefix matches in order of longest prefix first.
// This ensures /v1/device/diagnostics/ is matched before /v1/device/.
func ClassifyPath(path string) PathType {
	// Check exact matches first.
	if pt, ok := PathBoundaries[path]; ok {
		return pt
	}

	// Check prefix matches - iterate in order of longest prefix first.
	// to ensure more specific paths take precedence over shorter prefixes.
	longestPrefix := ""
	var longestPathType PathType
	for prefix, pt := range PathBoundaries {
		if strings.HasPrefix(path, prefix) {
			if len(prefix) > len(longestPrefix) {
				longestPrefix = prefix
				longestPathType = pt
			}
		}
	}
	if longestPrefix != "" {
		return longestPathType
	}

	return PathTypeTenant // Default to TENANT.
}

// IsPublicPath returns true if the path is PUBLIC (no auth required).
func IsPublicPath(path string) bool {
	return ClassifyPath(path) == PathTypePublic
}

// IsInfrastructurePath returns true if the path is INFRASTRUCTURE (Env API Key).
func IsInfrastructurePath(path string) bool {
	return ClassifyPath(path) == PathTypeInfrastructure
}

// IsSessionOnlyPath returns true if the path requires Session Cookie.
func IsSessionOnlyPath(path string) bool {
	return ClassifyPath(path) == PathTypeSessionOnly
}

// IsDeviceAuthPath returns true if the path requires HMAC authentication.
func IsDeviceAuthPath(path string) bool {
	return ClassifyPath(path) == PathTypeDeviceAuth
}

// IsTenantPath returns true if the path is TENANT (Session OR API Key + Scope).
func IsTenantPath(path string) bool {
	return ClassifyPath(path) == PathTypeTenant
}

// =============================================================================.
// TENANT API KEY AUTHENTICATION MIDDLEWARE.
// =============================================================================.

// TenantAPIKeyAuth provides tenant API key authentication middleware.
// ServiceAccountAuthenticator validates service account tokens. The returned
// identity carries its owning account, so the org comes off the credential.
type ServiceAccountAuthenticator interface {
	Authenticate(ctx context.Context, key string) (*svcapp.AuthenticatedToken, error)
}

// TenantAPIKeyAuth validates user API keys and, as a fallback, service
// account tokens so automation can authenticate without a human identity.
type TenantAPIKeyAuth struct {
	service     *keys.APIKeyService
	saAuth      ServiceAccountAuthenticator
	auditLogger *audit.Logger
}

// NewTenantAPIKeyAuth creates a new TenantAPIKeyAuth middleware.
func NewTenantAPIKeyAuth(service *keys.APIKeyService, auditLogger *audit.Logger) *TenantAPIKeyAuth {
	return &TenantAPIKeyAuth{
		service:     service,
		auditLogger: auditLogger,
	}
}

// SetServiceAccountAuth wires the service account token lookup path.
func (t *TenantAPIKeyAuth) SetServiceAccountAuth(auth ServiceAccountAuthenticator) {
	t.saAuth = auth
}

// Middleware returns the Gin middleware function for tenant API key authentication.
// This handles TENANT paths: Session OR API Key + Scope.
func (t *TenantAPIKeyAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		pathType := ClassifyPath(path)

		// Skip for non-tenant paths (let other middleware handle).
		if pathType != PathTypeTenant {
			c.Next()
			return
		}

		// Skip admin and API key management paths - these require session only.
		if strings.HasPrefix(path, "/v1/admin/") ||
			strings.HasPrefix(path, "/v1/api-keys/") {
			c.Next()
			return
		}

		// Check if already authenticated via session.
		if _, exists := c.Get("user_id"); exists {
			c.Next()
			return
		}

		// Extract API key from header.
		apiKey := extractAPIKeyFromHeader(c)
		if apiKey == "" {
			t.refuseMissingCredential(c)

			return
		}

		// Validate the key using the service. Service account tokens of
		// the form "vxyz_sa_..." fall back to the service account lookup.
		key, err := t.service.ValidateKey(c.Request.Context(), apiKey)
		if err != nil && t.saAuth != nil && strings.HasPrefix(apiKey, "vxyz_sa_") {
			if identity, saErr := t.saAuth.Authenticate(c.Request.Context(), apiKey); saErr == nil {
				token := identity.Token
				c.Set("user_id", "")
				c.Set("api_key_id", token.ID)
				// Scopes are optional now that roles are the permission source, so
				// there may be none; an empty scope must not index the slice.
				if len(token.Scopes) > 0 {
					c.Set("api_key_scope", token.Scopes[0])
				}
				c.Set("api_key_name", token.Name)
				c.Set("auth_type", "service_account")
				c.Set("service_account_id", token.ServiceID)
				c.Set("service_account_scopes", token.Scopes)
				// requireScope evaluates this token, so it travels in context.
				c.Set(ContextKeyServiceAccountToken, token)
				c.Set(ContextKeyOrganizationID, identity.OrgID())
				c.Set("service_account_org_id", identity.OrgID())
				c.Set("org_source", "service_account")
				c.Next()
				return
			}
		}
		if err != nil {
			// Log failed authentication attempt.
			if t.auditLogger != nil {
				// Extract key prefix for logging (last 8 chars before the key ID).
				keyPrefix := ""
				if len(apiKey) > 8 {
					keyPrefix = apiKey[:8]
				}
				reason := "invalid_api_key"
				if err.Error() == "api key has expired" {
					reason = "expired"
				} else if err.Error() == "api key has been revoked" {
					reason = "revoked"
				}
				t.auditLogger.APIKeyFailed(
					c.Request.Context(),
					"", // userID unknown for failed auth.
					keyPrefix,
					c.ClientIP(),
					c.GetHeader("User-Agent"),
					reason,
				)
			}

			statusCode := http.StatusUnauthorized
			if err.Error() == "api key has expired" {
				statusCode = http.StatusUnauthorized
			}
			responses.RespondStructuredAbort(c, statusCode,

				err.Error(),
			)
			return
		}

		// Store key info in context for downstream use.
		// CRITICAL: Also set user_id for handlers that check it (like keys_handler.go).
		c.Set("user_id", key.UserID)
		c.Set("api_key_id", key.ID)
		c.Set("api_key_scope", string(key.Scope))
		c.Set("api_key_name", key.Name)
		c.Set("auth_type", "tenant_api_key")
		// The API key's bound organization is the server-side source of org truth
		// for this credential (org_context reads it; a client header can't change it).
		if key.OrganizationID != "" {
			c.Set(ContextKeyOrganizationID, key.OrganizationID)
			c.Set("org_source", "api_key")
		}
		// Expose the API key's signing secret so SessionSignatureMiddleware can.
		// verify HMAC request signatures for API-key-authenticated requests.
		c.Set("api_key_signing_secret", key.SigningSecret)

		// Set a minimal user in the gin context so that downstream.
		// middleware (OrganizationMembership, RBAC, SuperAdmin) that call.
		// GetUserFromContext work with API key auth, not just sessions.
		// The membership check only needs user.ID; we avoid an extra DB.
		// fetch by constructing a minimal user from the key's UserID.
		c.Set(ContextKeyUser, &user.User{ID: key.UserID})

		// Increment usage asynchronously.
		go func() {
			bgCtx := context.Background()
			_ = t.service.IncrementUsage(bgCtx, key.ID)
		}()

		c.Next()
	}
}

// acceptsDeployCredential reports whether the request carries a deploy credential
// on a deploy path, in which case the deploy middleware owns authentication.
func (t *TenantAPIKeyAuth) acceptsDeployCredential(c *gin.Context) bool {
	return IsDeployPath(c.Request.URL.Path) && c.GetHeader(HeaderDeployCredential) != ""
}

// refuseMissingCredential handles a request that carries no API key and no deploy
// credential: it audits the attempt and aborts with 401.
func (t *TenantAPIKeyAuth) refuseMissingCredential(c *gin.Context) {
	// A deploy credential is a third credential the deploy surface accepts; its
	// own middleware owns the request and has already set the org.
	if t.acceptsDeployCredential(c) {
		c.Next()

		return
	}

	if t.auditLogger != nil {
		t.auditLogger.APIKeyFailed(
			c.Request.Context(),
			"",
			"",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			"missing",
		)
	}
	responses.RespondStructuredAbort(c, http.StatusUnauthorized, "X-API-Key header or session required")
}

// extractAPIKeyFromHeader extracts the API key from the X-API-Key header.
func extractAPIKeyFromHeader(c *gin.Context) string {
	// Try X-API-Key header first.
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// Try Authorization header with Bearer prefix.
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

// ScopeEnforcementFunc is a function that determines required scope based on HTTP method.
type ScopeEnforcementFunc func(method string) domain.Scope

// ScopeEnforcement returns a middleware that enforces scope based on HTTP method.
// It uses a scope determination function so different routes can have different requirements.
func (t *TenantAPIKeyAuth) ScopeEnforcement(scopeFn ScopeEnforcementFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Bearer credentials only; sessions are authorised by role.
		switch c.GetString("auth_type") {
		case "service_account":
			// requireScope does the authoritative action-level check.
			c.Next()
			return
		case "tenant_api_key":
			// Coarse method-based check below.
		default:
			c.Next()
			return
		}

		scopeVal, exists := c.Get("api_key_scope")
		if !exists {
			c.Next()
			return
		}

		scopeStr, ok := scopeVal.(string)
		if !ok {
			c.Next()
			return
		}

		// Determine required scope based on HTTP method.
		requiredScope := scopeFn(c.Request.Method)
		keyScope := domain.Scope(scopeStr)

		if !hasScope(keyScope, requiredScope) {
			responses.RespondStructured(c, http.StatusForbidden,

				"API key scope insufficient for this operation",
			)
			c.Abort()
			return
		}

		c.Next()
	}
}

// hasScope checks if the key's scope includes the required scope.
func hasScope(keyScope, requiredScope domain.Scope) bool {
	switch requiredScope {
	case domain.ScopeRead:
		return keyScope.CanRead()
	case domain.ScopeWrite:
		return keyScope.CanWrite()
	case domain.ScopeAdmin:
		return keyScope.CanDelete()
	default:
		return false
	}
}

// MethodToScope converts an HTTP method to the required scope.
func MethodToScope(method string) domain.Scope {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return domain.ScopeRead
	case "POST", "PUT", "PATCH":
		return domain.ScopeWrite
	case "DELETE":
		return domain.ScopeAdmin
	default:
		return domain.ScopeRead
	}
}

// ContextKeyServiceAccountToken is the gin context key holding the authenticated
// service account token, set by TenantAPIKeyAuth.
const ContextKeyServiceAccountToken = "service_account_token"

// GetServiceAccountToken returns the authenticated service account token, or nil
// when the request was not authenticated by one.
func GetServiceAccountToken(c *gin.Context) *serviceaccount.Token {
	if val, exists := c.Get(ContextKeyServiceAccountToken); exists {
		if token, ok := val.(*serviceaccount.Token); ok {
			return token
		}
	}
	return nil
}

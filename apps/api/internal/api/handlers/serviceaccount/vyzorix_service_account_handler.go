// Package serviceaccount provides HTTP handlers for org-scoped service accounts.
package serviceaccount

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	svcapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/serviceaccount"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/serviceaccount"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
)

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.ServiceAccount
	_ openapi.ServiceAccountToken
	_ openapi.ServiceAccountListResult
	_ openapi.ServiceAccountTokenListResult
	_ openapi.ServiceAccountTokenCreated
	_ openapi.ServiceAccountTokenRotated
	_ openapi.CreateServiceAccountRequest
	_ openapi.UpdateServiceAccountRequest
	_ openapi.CreateServiceAccountTokenRequest
	_ openapi.RotateServiceAccountTokenRequest
	_ openapi.DeletedResult
	_ openapi.RevokedResult
	_ openapi.ErrorResponse
)

// Handler processes service account CRUD and token operations.
type Handler struct {
	service *svcapp.Service
}

// NewHandler creates a new service account Handler.
func NewHandler(service *svcapp.Service) *Handler {
	return &Handler{service: service}
}

// Service returns the underlying application service (GraphQL wiring).
func (h *Handler) Service() *svcapp.Service { return h.service }

type createServiceAccountRequest struct {
	Name string `json:"name"`
}

type updateServiceAccountRequest struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
	// Reason is recorded with a state change so a disabled account can explain
	// itself. Ignored on a rename.
	Reason string `json:"reason"`
}

type createTokenRequest struct {
	ExpiresAt *string     `json:"expires_at"`
	ServiceID string      `json:"service_id"`
	Name      string      `json:"name"`
	BasicRole string      `json:"basic_role"`
	Roles     []roleGrant `json:"roles"`
}

type rotateTokenRequest struct {
	ExpiresAt *string     `json:"expires_at"`
	Name      string      `json:"name"`
	BasicRole string      `json:"basic_role"`
	Roles     []roleGrant `json:"roles"`
}

// roleGrant is one fixed role at one breadth, on the wire.
type roleGrant struct {
	Role  string `json:"role"`
	Scope string `json:"scope"`
}

type updateTokenRolesRequest struct {
	BasicRole string      `json:"basic_role"`
	Roles     []roleGrant `json:"roles"`
}

// toFixedRoleGrants converts wire grants to domain grants.
func toFixedRoleGrants(in []roleGrant) []permission.FixedRoleGrant {
	if in == nil {
		return nil
	}
	out := make([]permission.FixedRoleGrant, len(in))
	for i, g := range in {
		out[i] = permission.FixedRoleGrant{Role: g.Role, Scope: permission.FixedRoleScope(g.Scope)}
	}
	return out
}

// toRoleGrantsJSON converts domain grants to the wire shape.
func toRoleGrantsJSON(in []permission.FixedRoleGrant) []openapi.FixedRoleGrant {
	out := make([]openapi.FixedRoleGrant, len(in))
	for i, g := range in {
		out[i] = openapi.FixedRoleGrant{Role: g.Role, Scope: string(g.Scope)}
	}
	return out
}

func serviceAccountJSON(sa *serviceaccount.ServiceAccount) openapi.ServiceAccount {
	out := openapi.ServiceAccount{
		ID:             sa.ID,
		OrgID:          sa.OrgID,
		Name:           sa.Name,
		Enabled:        sa.Enabled,
		TokenCount:     sa.TokenCount,
		AvatarURL:      serviceAccountAvatarSeed(sa),
		CreatedAt:      sa.CreatedAt.UnixMilli(),
		UpdatedAt:      sa.UpdatedAt.UnixMilli(),
		DisabledBy:     sa.DisabledBy,
		DisabledByName: sa.DisabledByName,
		EnabledBy:      sa.EnabledBy,
		EnabledByName:  sa.EnabledByName,
		DisabledReason: sa.DisabledReason,
	}
	if sa.DisabledAt != nil {
		ms := sa.DisabledAt.UnixMilli()
		out.DisabledAt = &ms
	}

	return out
}

func tokenJSON(token *serviceaccount.Token) openapi.ServiceAccountToken {
	return openapi.ServiceAccountToken{
		ID:         token.ID,
		ServiceID:  token.ServiceID,
		Name:       token.Name,
		KeyPrefix:  token.KeyPrefix,
		Scopes:     token.Scopes,
		BasicRole:  token.BasicRole,
		Roles:      toRoleGrantsJSON(token.Roles),
		Valid:      token.Valid,
		CreatedAt:  token.CreatedAt.UnixMilli(),
		ExpiresAt:  unixMilliPtr(token.ExpiresAt),
		RevokedAt:  unixMilliPtr(token.RevokedAt),
		LastUsedAt: unixMilliPtr(token.LastUsedAt),
	}
}

// serviceAccountAvatarSeed is the value the client hashes to render an avatar.
// The image is a pure function of the id, so nothing is stored.
func serviceAccountAvatarSeed(sa *serviceaccount.ServiceAccount) string {
	if sa == nil {
		return ""
	}
	return sa.ID
}

// unixMilliPtr renders an optional timestamp as Unix milliseconds, nil when unset.
func unixMilliPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

func parseExpiresAt(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, *s); err == nil {
		return &t
	}
	return nil
}

// List handles GET /v1/service-accounts.
// @Summary      List service accounts
// @Description  Returns all org-scoped service accounts
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  openapi.ServiceAccountListResult  "service accounts"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts [get]
func (h *Handler) List(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	accounts, err := h.service.List(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to list service accounts"))
		return
	}
	items := make([]openapi.ServiceAccount, 0, len(accounts))
	for _, sa := range accounts {
		items = append(items, serviceAccountJSON(sa))
	}
	c.JSON(http.StatusOK, openapi.ServiceAccountListResult{
		ServiceAccounts: items,
	})
}

// Create handles POST /v1/service-accounts.
// @Summary      Create service account
// @Description  Creates a new org-scoped service account
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        body  body  openapi.CreateServiceAccountRequest  true  "service account definition"
// @Success      201  {object}  openapi.ServiceAccount  "created service account"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts [post]
func (h *Handler) Create(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	var req createServiceAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid request body: "+err.Error()))
		return
	}
	sa, err := h.service.Create(c.Request.Context(), orgID, req.Name)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, serviceAccountJSON(sa))
}

// Update handles PATCH /v1/service-accounts/:id.
// @Summary      Update service account
// @Description  Renames a service account and/or toggles it enabled. Omitted fields are left unchanged
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "service account ID"
// @Param        body  body  openapi.UpdateServiceAccountRequest  true  "fields to update"
// @Success      200  {object}  openapi.ServiceAccount  "updated service account"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	var req updateServiceAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid request body"))
		return
	}
	actor := middleware.GetUserFromContext(c)
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}

	sa, err := h.service.Update(c.Request.Context(), orgID, c.Param("id"), req.Name, req.Enabled, actorID, req.Reason)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, serviceAccountJSON(sa))
}

// Delete handles DELETE /v1/service-accounts/:id.
// @Summary      Delete service account
// @Description  Removes a service account and its tokens
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "service account ID"
// @Success      200  {object}  openapi.DeletedResult  "deleted confirmation"
// @Failure      400  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if err := h.service.Delete(c.Request.Context(), orgID, c.Param("id")); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, openapi.DeletedResult{Deleted: true})
}

// ListTokens handles GET /v1/service-accounts/:id/tokens.
// @Summary      List service account tokens
// @Description  Returns all tokens for a service account
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "service account ID"
// @Success      200  {object}  openapi.ServiceAccountTokenListResult  "tokens"
// @Failure      400  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id}/tokens [get]
func (h *Handler) ListTokens(c *gin.Context) {
	if _, err := h.service.Get(c.Request.Context(), middleware.GetOrganizationID(c), c.Param("id")); err != nil {
		h.writeServiceError(c, err)
		return
	}
	tokens, err := h.service.ListTokens(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to list tokens"))
		return
	}
	items := make([]openapi.ServiceAccountToken, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, tokenJSON(token))
	}
	c.JSON(http.StatusOK, openapi.ServiceAccountTokenListResult{
		Tokens: items,
	})
}

// CreateToken handles POST /v1/service-accounts/:id/tokens. The full key is
// returned once and never stored.
// @Summary      Create service account token
// @Description  Creates a new token for a service account. Returns the full key once
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "service account ID"
// @Param        body  body  openapi.CreateServiceAccountTokenRequest  true  "token definition"
// @Success      201  {object}  openapi.ServiceAccountTokenCreated  "created token with full key"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input / not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id}/tokens [post]
func (h *Handler) CreateToken(c *gin.Context) {
	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid request body: "+err.Error()))
		return
	}
	if req.ServiceID == "" {
		req.ServiceID = c.Param("id")
	}
	_, fullKey, err := h.service.CreateToken(c.Request.Context(), &svcapp.TokenInput{
		ServiceID: req.ServiceID,
		Name:      req.Name,
		BasicRole: req.BasicRole,
		Roles:     toFixedRoleGrants(req.Roles),
		ExpiresAt: parseExpiresAt(req.ExpiresAt),
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response := openapi.ServiceAccountTokenCreated{
		Secret: fullKey,
	}
	c.JSON(http.StatusCreated, response)
}

// RevokeToken handles DELETE /v1/service-accounts/:id/tokens/:token_id.
// The default is a soft revoke; `?purge=true` deletes the row.
// @Summary      Revoke or delete service account token
// @Description  Revokes a token by ID (soft, auditable). With purge=true the token row is deleted.
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true   "Organization ID"
// @Param        id     path   string  true   "service account ID"
// @Param        token  path   string  true   "token ID"
// @Param        purge  query  bool    false  "Delete the token row instead of marking it revoked"
// @Success      200  {object}  openapi.RevokedResult  "revoked confirmation"
// @Failure      400  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id}/tokens/{token} [delete]
func (h *Handler) RevokeToken(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	serviceID := c.Param("id")
	tokenID := c.Param("token_id")

	if c.Query("purge") == "true" {
		if err := h.service.DeleteToken(c.Request.Context(), orgID, serviceID, tokenID); err != nil {
			h.writeServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, openapi.RevokedResult{Revoked: true})
		return
	}

	if _, err := h.service.Get(c.Request.Context(), orgID, serviceID); err != nil {
		h.writeServiceError(c, err)
		return
	}
	if err := h.service.RevokeToken(c.Request.Context(), tokenID); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, openapi.RevokedResult{Revoked: true})
}

// ListFixedRoles handles GET /v1/service-accounts/roles, the role picker's
// catalog.
// @Summary      List assignable roles
// @Description  Returns the fixed-role catalog and org roles a service account token may be assigned
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.FixedRoleCatalogResult  "assignable roles"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/roles [get]
func (h *Handler) ListFixedRoles(c *gin.Context) {
	catalog := permission.FixedRoles()
	roles := make([]openapi.FixedRoleInfo, len(catalog))
	for i, role := range catalog {
		roles[i] = openapi.FixedRoleInfo{
			ID:     role.ID,
			Group:  role.Group,
			Access: role.Access.String(),
		}
	}
	c.JSON(http.StatusOK, openapi.FixedRoleCatalogResult{
		Roles:    roles,
		OrgRoles: permission.OrgRoles(),
		Scopes:   []string{string(permission.ScopeOrgFixed), string(permission.ScopeAllFixed)},
	})
}

// UpdateTokenRoles handles PATCH /v1/service-accounts/:id/tokens/:token_id.
// Roles are editable without rotating the key.
// @Summary      Update service account token roles
// @Description  Replaces a token's basic role and fixed roles. The token's key is unchanged
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id     path  string  true  "service account ID"
// @Param        token  path  string  true  "token ID"
// @Param        body   body  openapi.UpdateServiceAccountTokenRolesRequest  true  "roles to assign"
// @Success      200  {object}  openapi.ServiceAccountToken  "updated token"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid role"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id}/tokens/{token} [patch]
func (h *Handler) UpdateTokenRoles(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	var req updateTokenRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid request body"))
		return
	}
	token, err := h.service.UpdateTokenRoles(
		c.Request.Context(), orgID, c.Param("id"), c.Param("token_id"),
		req.BasicRole, toFixedRoleGrants(req.Roles),
	)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tokenJSON(token))
}

// RotateToken handles POST /v1/service-accounts/:id/tokens/:token_id/rotate.
// @Summary      Rotate service account token
// @Description  Revokes the existing token and issues a new one. Returns the full key once
// @Tags         service-accounts
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "service account ID"
// @Param        token  path  string  true  "token ID"
// @Param        body  body  openapi.RotateServiceAccountTokenRequest  true  "new token definition"
// @Success      201  {object}  openapi.ServiceAccountTokenRotated  "rotated token with full key"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input / not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /service-accounts/{id}/tokens/{token}/rotate [post]
func (h *Handler) RotateToken(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if _, err := h.service.Get(c.Request.Context(), orgID, c.Param("id")); err != nil {
		h.writeServiceError(c, err)
		return
	}
	var req rotateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid request body: "+err.Error()))
		return
	}
	_, fullKey, err := h.service.RotateToken(c.Request.Context(), c.Param("token_id"), &svcapp.TokenInput{
		ServiceID: c.Param("id"),
		Name:      req.Name,
		BasicRole: req.BasicRole,
		Roles:     toFixedRoleGrants(req.Roles),
		ExpiresAt: parseExpiresAt(req.ExpiresAt),
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response := openapi.ServiceAccountTokenCreated{
		Secret: fullKey,
	}
	c.JSON(http.StatusCreated, response)
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, svcapp.ErrServiceAccountNotFound):
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "service account not found"))
	case errors.Is(err, svcapp.ErrDuplicateTokenName):
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceAlreadyExists, err.Error()))
	case errors.Is(err, svcapp.ErrInvalidToken), errors.Is(err, svcapp.ErrInvalidServiceAccount), errors.Is(err, svcapp.ErrInvalidScope):
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, err.Error()))
	case errors.Is(err, serviceaccount.ErrDisabledReasonTooLong):
		// A caller error, not a server fault: the reason exceeded the cap.
		_ = c.Error(apperrors.NewServerError(
			apperrors.CodeValidationFailed,
			fmt.Sprintf("reason must be %d characters or fewer", serviceaccount.MaxDisabledReasonLength),
		))
	default:
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "service account operation failed"))
	}
}

package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

// orgUsersToOpenAPI maps the org-scoped listing to its wire shape.
func orgUsersToOpenAPI(in []dto.OrgUserResponse) []openapi.OrgUser {
	out := make([]openapi.OrgUser, len(in))
	for i, o := range in {
		out[i] = orgUserJSON(o)
	}
	return out
}

// orgUserJSON maps one member row to its wire shape.
func orgUserJSON(o dto.OrgUserResponse) openapi.OrgUser {
	return openapi.OrgUser{
		ID:            o.ID,
		MemberID:      o.MemberID,
		Email:         o.Email,
		Name:          o.Name,
		Role:          o.Role,
		Status:        o.Status,
		JoinedAt:      o.JoinedAt,
		AvatarURL:     o.AvatarURL,
		CreatedAt:     o.CreatedAt,
		LastSeenAt:    o.LastSeenAt,
		MFAEnabled:    o.MFAEnabled,
		EmailVerified: o.EmailVerified,
		Disabled:      o.Disabled,
	}
}

// orgUserFromEntity builds the wire shape from a membership-scoped lookup.
func orgUserFromEntity(op *user.OrganizationUser) openapi.OrgUser {
	var lastSeen *int64
	if op.LastSeenAt != nil {
		ms := op.LastSeenAt.UnixMilli()
		lastSeen = &ms
	}

	return openapi.OrgUser{
		ID:            op.ID,
		MemberID:      op.MemberID,
		Email:         op.Email,
		Name:          op.Name,
		Role:          op.Role,
		Status:        op.Status,
		JoinedAt:      op.JoinedAt.UnixMilli(),
		AvatarURL:     op.AvatarURL,
		CreatedAt:     op.CreatedAt.UnixMilli(),
		LastSeenAt:    lastSeen,
		MFAEnabled:    op.MFAEnabled,
		EmailVerified: op.EmailVerified,
		Disabled:      op.Disabled,
	}
}

// parseIntQuery reads a bounded integer query parameter, returning the fallback
// when absent or unparseable.
func parseIntQuery(c *gin.Context, key string, fallback, lo, hi int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.OrgUserListResult
	_ openapi.OrgUser
	_ openapi.OrgUserStatsResult
	_ openapi.AdminUserSessionListResult
	_ openapi.SuccessResult
	_ openapi.RevokeResult
	_ openapi.ErrorResponse
)

// AdminHandler handles admin endpoints with organization-scoped access control.
// All endpoints require the user to be a super_admin in the current organization.
type AdminHandler struct {
	authService *auth.AuthService
	presenter   *response.Presenter
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(authService *auth.AuthService, presenter *response.Presenter) *AdminHandler {
	return &AdminHandler{authService: authService, presenter: presenter}
}

// UserStats handles GET /v1/auth/admin/users/stats.
// @Summary      Organisation member stats
// @Description  Member count per role plus the pending invitation count for the current organisation (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  openapi.OrgUserStatsResult  "stats"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/stats [get]
func (h *AdminHandler) UserStats(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "not authenticated"))
		return
	}

	if !op.IsSuperAdminIn(orgID) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzInsufficientPermissions, "insufficient privileges"))
		return
	}

	stats, err := h.authService.UserStats(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to count members"))
		return
	}

	c.JSON(http.StatusOK, openapi.OrgUserStatsResult{
		ByRole:         stats.ByRole,
		Total:          stats.Total,
		PendingInvites: stats.PendingInvites,
	})
}

// ListUsers handles GET /v1/auth/admin/users.
// @Summary      List organisation members
// @Description  Lists the members of the current organisation, paginated and optionally searched by email or name (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true   "Organization ID"
// @Param        page               query   int     false  "Page number (1-based)"
// @Param        limit              query   int     false  "Page size (max 100)"
// @Param        q                  query   string  false  "Search by email or name"
// @Param        sort               query   string  false  "Sort column: email, name, created_at, last_login_at"
// @Param        order              query   string  false  "Sort direction: asc or desc"
// @Param        status             query   string  false  "Membership status filter: active, suspended; empty means every state except removed"
// @Success      200  {object}  openapi.OrgUserListResult  "members"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "not authenticated"))
		return
	}

	// Org-scoped check - user must be super_admin in this organization.
	if !op.IsSuperAdminIn(orgID) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzInsufficientPermissions, "insufficient privileges"))
		return
	}

	page := parseIntQuery(c, "page", 1, 1, 1_000_000)
	limit := parseIntQuery(c, "limit", 20, 1, 100)
	// Default to newest first; "asc" flips it.
	desc := c.Query("order") != "asc"

	users, total, err := h.authService.ListOrgUsers(c.Request.Context(), auth.ListUsersQuery{
		Search: strings.TrimSpace(c.Query("q")),
		Sort:   c.Query("sort"),
		OrgID:  orgID,
		Status: c.Query("status"),
		Desc:   desc,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to list users"))
		return
	}

	totalPages := (total + limit - 1) / limit

	c.JSON(http.StatusOK, openapi.OrgUserListResult{
		Users:      orgUsersToOpenAPI(users),
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

// GetUser handles GET /v1/auth/admin/users/:id.
// @Summary      Get organisation member
// @Description  Returns a member of the current organisation (super_admin only). A user outside the organisation is reported as not found.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.OrgUser  "member"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not a member of this organisation"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id} [get]
func (h *AdminHandler) GetUser(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "not authenticated"))
		return
	}

	if !op.IsSuperAdminIn(orgID) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzInsufficientPermissions, "insufficient privileges"))
		return
	}

	userID := c.Param("id")
	if userID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "user id is required"))
		return
	}

	target, err := h.authService.GetOrgUser(c.Request.Context(), orgID, userID)
	if err != nil {
		if errors.Is(err, application.ErrUserNotFound) {
			_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "not_found"))
			return
		}
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to read user"))
		return
	}

	c.JSON(http.StatusOK, orgUserFromEntity(target))
}

// DeleteUser handles DELETE /v1/instance/users/:id.
// @Summary      Delete an account
// @Description  Deletes an account and its memberships (instance admin only)
// @Tags         instance
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.SuccessResult  "deleted"
// @Failure      400  {object}  openapi.ErrorResponse  "cannot delete your own account"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "instance admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "cannot delete the last super admin"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /instance/users/{id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	userID := c.Param("id")
	if userID == "" {
		h.presenter.BadRequest(c, "user id is required")
		return
	}

	if userID == op.ID {
		h.presenter.BadRequest(c, "cannot delete your own account")
		return
	}

	if err := h.authService.DeleteUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, application.ErrUserNotFound) {
			h.presenter.NotFound(c, "user not found")
			return
		}
		if errors.Is(err, application.ErrCannotDeleteLastSuperAdmin) {
			h.presenter.Conflict(c, "cannot delete the last super admin of an organization")
			return
		}
		h.presenter.InternalError(c, "failed to delete user")
		return
	}

	h.presenter.AdminAction(c, op.ID, "delete_user", "user", userID, nil)
	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: "user deleted"})
}

// requireSuperAdmin loads the caller and the org, and enforces the super-admin
// check shared by the organisation-scoped admin handlers. It writes the error
// response itself and reports false when the caller may not proceed.
func (h *AdminHandler) requireSuperAdmin(c *gin.Context) (*user.User, bool) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return nil, false
	}
	if !op.IsSuperAdminIn(orgID) {
		h.presenter.Forbidden(c, "insufficient privileges")
		return nil, false
	}
	return op, true
}

// DisableUser handles POST /v1/instance/users/:id/disable.
// @Summary      Disable an account
// @Description  Disables an account and revokes every session (instance admin only)
// @Tags         instance
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.SuccessResult  "disabled"
// @Failure      400  {object}  openapi.ErrorResponse  "cannot disable your own account"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "instance admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /instance/users/{id}/disable [post]
func (h *AdminHandler) DisableUser(c *gin.Context) {
	h.setUserDisabled(c, true)
}

// EnableUser handles POST /v1/instance/users/:id/enable.
// @Summary      Re-enable an account
// @Description  Clears the account-disable timestamp (instance admin only)
// @Tags         instance
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.SuccessResult  "enabled"
// @Failure      400  {object}  openapi.ErrorResponse  "user id required"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "instance admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /instance/users/{id}/enable [post]
func (h *AdminHandler) EnableUser(c *gin.Context) {
	h.setUserDisabled(c, false)
}

// setUserDisabled is the shared body of DisableUser and EnableUser. The route is
// instance-scoped, so the authority check lives in requireInstanceScope; the
// handler only refuses the self-disable case.
func (h *AdminHandler) setUserDisabled(c *gin.Context, disabled bool) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	userID := c.Param("id")
	if userID == "" {
		h.presenter.BadRequest(c, "user id is required")
		return
	}
	if disabled && userID == op.ID {
		h.presenter.BadRequest(c, "cannot disable your own account")
		return
	}

	if _, err := h.authService.SetUserDisabled(c.Request.Context(), userID, disabled); err != nil {
		h.writeUserError(c, err)
		return
	}

	action := "enable_user"
	message := "user enabled"
	if disabled {
		action = "disable_user"
		message = "user disabled"
	}
	h.presenter.AdminAction(c, op.ID, action, "user", userID, nil)
	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: message})
}

// ListUserSessions handles GET /v1/auth/admin/users/:id/sessions.
// @Summary      List user sessions
// @Description  Lists the active sessions of a user (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.AdminUserSessionListResult  "sessions"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id}/sessions [get]
func (h *AdminHandler) ListUserSessions(c *gin.Context) {
	if _, ok := h.requireSuperAdmin(c); !ok {
		return
	}

	userID := c.Param("id")
	if userID == "" {
		h.presenter.BadRequest(c, "user id is required")
		return
	}

	sessions, err := h.authService.ListUserSessions(c.Request.Context(), userID)
	if err != nil {
		h.writeUserError(c, err)
		return
	}

	items := make([]openapi.SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, openapi.SessionInfo{
			ID:                     sess.ID,
			UserID:                 sess.UserID,
			IPAddress:              sess.IPAddress,
			UserAgent:              sess.UserAgent,
			CreatedAt:              sess.CreatedAt.UnixMilli(),
			ExpiresAt:              sess.ExpiresAt.UnixMilli(),
			SelectedOrganizationID: sess.SelectedOrganizationID,
		})
	}

	c.JSON(http.StatusOK, openapi.AdminUserSessionListResult{
		Sessions: items,
		Total:    len(items),
	})
}

// RevokeUserSession handles DELETE /v1/auth/admin/users/:id/sessions/:sessionId.
// @Summary      Revoke user session
// @Description  Revokes one session belonging to a user (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id         path  string  true  "user ID"
// @Param        sessionId  path  string  true  "session ID"
// @Success      200  {object}  openapi.SuccessResult  "session revoked"
// @Failure      400  {object}  openapi.ErrorResponse  "session id required"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "session not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id}/sessions/{sessionId} [delete]
func (h *AdminHandler) RevokeUserSession(c *gin.Context) {
	op, ok := h.requireSuperAdmin(c)
	if !ok {
		return
	}

	userID := c.Param("id")
	sessionID := c.Param("sessionId")
	if userID == "" || sessionID == "" {
		h.presenter.BadRequest(c, "user id and session id are required")
		return
	}

	if err := h.authService.RevokeUserSession(c.Request.Context(), userID, sessionID); err != nil {
		if errors.Is(err, application.ErrUserNotFound) {
			h.presenter.NotFound(c, "session not found")
			return
		}
		h.presenter.InternalError(c, "failed to revoke session")
		return
	}

	h.presenter.AdminAction(c, op.ID, "revoke_user_session", "user", userID, map[string]string{"session_id": sessionID})
	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: "session revoked"})
}

// RevokeAllUserSessions handles DELETE /v1/auth/admin/users/:id/sessions.
// @Summary      Revoke all user sessions
// @Description  Revokes every active session of a user (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.RevokeResult  "sessions revoked"
// @Failure      400  {object}  openapi.ErrorResponse  "user id required"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id}/sessions [delete]
func (h *AdminHandler) RevokeAllUserSessions(c *gin.Context) {
	op, ok := h.requireSuperAdmin(c)
	if !ok {
		return
	}

	userID := c.Param("id")
	if userID == "" {
		h.presenter.BadRequest(c, "user id is required")
		return
	}

	count, err := h.authService.RevokeAllUserSessions(c.Request.Context(), userID)
	if err != nil {
		h.presenter.InternalError(c, "failed to revoke sessions")
		return
	}

	h.presenter.AdminAction(c, op.ID, "revoke_all_user_sessions", "user", userID, nil)
	h.presenter.OK(c, openapi.RevokeResult{
		Success:      true,
		RevokedCount: count,
		Message:      "all sessions revoked",
	})
}

// writeUserError maps the shared user-mutation errors to HTTP responses.
func (h *AdminHandler) writeUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrUserNotFound):
		h.presenter.NotFound(c, "user not found")
	case errors.Is(err, application.ErrEmailExists):
		h.presenter.Conflict(c, "email already in use")
	case errors.Is(err, application.ErrPasswordBreached):
		h.presenter.BadRequest(c, "password found in a data breach, please choose another")
	case errors.Is(err, application.ErrPasswordPolicy), errors.Is(err, application.ErrInvalidInput):
		h.presenter.BadRequest(c, "password does not meet the policy")
	case errors.Is(err, application.ErrBreachCheckUnavailable):
		h.presenter.ServiceUnavailable(c, "password verification temporarily unavailable")
	default:
		h.presenter.InternalError(c, "user operation failed")
	}
}

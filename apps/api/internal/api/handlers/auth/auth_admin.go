package auth

import (
	"errors"
	"net/http"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

func dtoUsersToOpenAPI(in []dto.UserListResponse) []openapi.AdminUser {
	out := make([]openapi.AdminUser, len(in))
	for i, o := range in {
		out[i] = openapi.AdminUser{
			ID: o.ID, Email: o.Email, Name: o.Name, Role: o.Role,
			MFAEnabled: o.MFAEnabled, EmailVerified: o.EmailVerified, CreatedAt: o.CreatedAt,
		}
	}
	return out
}

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.AdminUserListResult
	_ openapi.AdminUser
	_ openapi.CreateUserRequest
	_ openapi.UpdateUserRequest
	_ openapi.SuccessResult
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

// ListUsers handles GET /v1/auth/admin/users.
// @Summary      List users
// @Description  Lists all users in the system (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  openapi.AdminUserListResult  "users"
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

	users, total, err := h.authService.ListAllUsers(c.Request.Context(), 20, 0)
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to list users"))
		return
	}

	c.JSON(http.StatusOK, openapi.AdminUserListResult{
		Users: dtoUsersToOpenAPI(users),
		Total:     total,
	})
}

// CreateUser handles POST /v1/auth/admin/users.
// @Summary      Create user
// @Description  Creates a new user (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        body  body  openapi.CreateUserRequest  true  "user credentials"
// @Success      201  {object}  openapi.AdminUser  "created user"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      409  {object}  openapi.ErrorResponse  "email already in use"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users [post]
func (h *AdminHandler) CreateUser(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	// Org-scoped check.
	if !op.IsSuperAdminIn(orgID) {
		h.presenter.Forbidden(c, "insufficient privileges")
		return
	}

	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid request body")
		return
	}

	if req.Email == "" {
		h.presenter.BadRequest(c, "email is required")
		return
	}

	if req.Password == "" {
		h.presenter.BadRequest(c, "password is required")
		return
	}

	newOp, err := h.authService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, application.ErrEmailExists) {
			h.presenter.Conflict(c, "email already in use")
			return
		}
		if errors.Is(err, application.ErrInvalidInput) {
			h.presenter.BadRequest(c, "invalid input")
			return
		}
		h.presenter.InternalError(c, "Invalid request")
		return
	}

	h.presenter.AdminAction(c, op.ID, "create_user", "user", newOp.ID, nil)
	h.presenter.Created(c, openapi.AdminUser{
		ID:        newOp.ID,
		Email:     newOp.Email,
		Name:      newOp.Name,
		CreatedAt: newOp.CreatedAt.UnixMilli(),
	})
}

// GetUser handles GET /v1/auth/admin/users/:id.
// @Summary      Get user
// @Description  Returns a single user by ID (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.AdminUser  "created user"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id} [get]
func (h *AdminHandler) GetUser(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "not authenticated"))
		return
	}

	// Org-scoped check.
	if !op.IsSuperAdminIn(orgID) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzInsufficientPermissions, "insufficient privileges"))
		return
	}

	userID := c.Param("id")
	if userID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "operator id is required"))
		return
	}

	targetOp, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "not_found"))
		return
	}

	c.JSON(http.StatusOK, openapi.AdminUser{
		ID:            targetOp.ID,
		Email:         targetOp.Email,
		Name:          targetOp.Name,
		MFAEnabled:    targetOp.MFASecret != "",
		EmailVerified: targetOp.EmailVerified,
		CreatedAt:     targetOp.CreatedAt.UnixMilli(),
		UpdatedAt:     targetOp.UpdatedAt.UnixMilli(),
	})
}

// UpdateUser handles PATCH /v1/auth/admin/users/:id.
// @Summary      Update user
// @Description  Updates mutable user fields (super_admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id    path  string  true  "user ID"
// @Param        body  body  openapi.UpdateUserRequest  true  "user update"
// @Success      200  {object}  openapi.AdminUser  "updated user"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "email already in use"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id} [patch]
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "not authenticated"))
		return
	}

	// Org-scoped check.
	if !op.IsSuperAdminIn(orgID) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzInsufficientPermissions, "insufficient privileges"))
		return
	}

	userID := c.Param("id")
	if userID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "operator id is required"))
		return
	}

	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid request body"))
		return
	}

	updatedOp, err := h.authService.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, application.ErrUserNotFound) {
			h.presenter.NotFound(c, "operator not found")
			return
		}
		if errors.Is(err, application.ErrEmailExists) {
			h.presenter.Conflict(c, "email already in use")
			return
		}
		if errors.Is(err, application.ErrInvalidInput) {
			h.presenter.BadRequest(c, "invalid request body")
			return
		}
		h.presenter.InternalError(c, "Invalid request")
		return
	}

	c.JSON(http.StatusOK, openapi.AdminUser{
		ID:        updatedOp.ID,
		Email:     updatedOp.Email,
		Name:      updatedOp.Name,
		UpdatedAt: updatedOp.UpdatedAt.UnixMilli(),
	})
}

// DeleteUser handles DELETE /v1/auth/admin/users/:id.
// @Summary      Delete user
// @Description  Deletes a user. Cannot delete yourself or the last super_admin
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "user ID"
// @Success      200  {object}  openapi.SuccessResult  "user deleted"
// @Failure      400  {object}  openapi.ErrorResponse  "user id required / cannot delete self"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "cannot delete last super_admin"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /auth/admin/users/{id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	orgID := middleware.GetOrganizationID(c)
	if op == nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	// Org-scoped check.
	if !op.IsSuperAdminIn(orgID) {
		h.presenter.Forbidden(c, "insufficient privileges")
		return
	}

	userID := c.Param("id")
	if userID == "" {
		h.presenter.BadRequest(c, "operator id is required")
		return
	}

	if userID == op.ID {
		h.presenter.BadRequest(c, "cannot delete your own account")
		return
	}

	if err := h.authService.DeleteUser(c.Request.Context(), userID); err != nil {
		if errors.Is(err, application.ErrUserNotFound) {
			h.presenter.NotFound(c, "operator not found")
			return
		}
		if errors.Is(err, application.ErrCannotDeleteLastSuperAdmin) {
			h.presenter.Conflict(c, "cannot delete the last super admin of an organization")
			return
		}
		h.presenter.InternalError(c, "Invalid request")
		return
	}

	h.presenter.AdminAction(c, op.ID, "delete_user", "user", userID, nil)
	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: "operator deleted"})
}

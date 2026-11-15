package auth

import (
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	appauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.LockoutStatusResult
	_ openapi.SuccessResult
	_ openapi.ErrorResponse
)

// LockoutHandler handles account lockout endpoints.
type LockoutHandler struct {
	authService *appauth.AuthService
	lockout     *middleware.Lockout
	presenter   *response.Presenter
}

// NewLockoutHandler creates a new LockoutHandler.
func NewLockoutHandler(authService *appauth.AuthService, lockout *middleware.Lockout, presenter *response.Presenter) *LockoutHandler {
	return &LockoutHandler{
		authService: authService,
		lockout:     lockout,
		presenter:   presenter,
	}
}

// Middleware returns a Gin middleware that checks lockout status.
// 3: Exposed so other handlers can use it.
func (h *LockoutHandler) Middleware() gin.HandlerFunc {
	return middleware.LockoutMiddleware(h.lockout)
}

// GetLockoutStatus handles GET /v1/auth/lockout/status.
// @Summary      Get lockout status
// @Description  Returns the current account lockout status for the authenticated user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.LockoutStatusResult  "lockout status"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/lockout/status [get]
func (h *LockoutHandler) GetLockoutStatus(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "")
		return
	}

	locked, attemptsRemaining, retryAfter := h.lockout.GetLockoutInfo(op.Email)
	if locked {
		h.presenter.OK(c, openapi.LockoutStatusResult{
			Locked:     true,
			Reason:     "Too many failed attempts",
			RetryAfter: int64Ptr(int64(retryAfter.Seconds())),
			Attempts:   attemptsRemaining,
		})

		return
	}
}

// UnlockAccount handles POST /v1/instance/lockout/unlock/:user_id.
// @Summary      Unlock account
// @Description  Clears a user's account lockout. Instance-scoped: requires admin.lockout on the deployment, not an organization role. Locking a user out of the deployment is not contained by any single organization.
// @Tags         instance
// @Accept       json
// @Produce      json
// @Param        user_id  path  string  true  "user ID to unlock"
// @Success      200  {object}  openapi.SuccessResult  "account unlocked"
// @Failure      400  {object}  openapi.ErrorResponse  "user_id required"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "instance admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "user not found"
// @Router       /instance/lockout/unlock/{user_id} [post]
func (h *LockoutHandler) UnlockAccount(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "")
		return
	}

	// Authority is the route's requireInstanceScope(admin.lockout) guard; the
	// handler performs no org check, because this action is deployment-scoped.
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		h.presenter.BadRequest(c, "")
		return
	}

	// Get target user email to clear their lockout.
	targetOp, err := h.authService.GetUserByID(c.Request.Context(), targetUserID)
	if err != nil || targetOp == nil {
		h.presenter.NotFound(c, "")
		return
	}

	// Clear lockout using in-memory lockout (email-based).
	h.lockout.RecordSuccessfulAttempt(targetOp.Email)

	h.presenter.AdminAction(c, op.ID, "unlock_account", "user", targetUserID, nil)
	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: "Account unlocked successfully"})
}

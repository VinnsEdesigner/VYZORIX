package admin

import (
	"net/http"
	"strconv"

	apikeyapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/keys"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	apikeydomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
	"github.com/gin-gonic/gin"
)

func apiKeysToAdminOpenAPI(in []apikeydomain.APIKeyResponse) []openapi.AdminAPIKey {
	out := make([]openapi.AdminAPIKey, len(in))
	for i, k := range in {
		item := openapi.AdminAPIKey{
			ID:           k.ID,
			Name:         k.Name,
			KeyPrefix:    k.KeyPrefix,
			Scope:        string(k.Scope),
			RequestCount: k.RequestCount,
			IsActive:     k.IsActive,
			CreatedAt:    k.CreatedAt.UnixMilli(),
			UpdatedAt:    k.UpdatedAt.UnixMilli(),
		}
		if k.ExpiresAt != nil {
			item.ExpiresAt = int64Ptr(k.ExpiresAt.UnixMilli())
		}
		if k.LastRequest != nil {
			item.LastRequestAt = int64Ptr(k.LastRequest.UnixMilli())
		}
		if k.RevokedAt != nil {
			item.RevokedAt = int64Ptr(k.RevokedAt.UnixMilli())
		}
		out[i] = item
	}
	return out
}

func adminKeysByUserToOpenAPI(in []apikeydomain.AdminAPIKeyResponse) []openapi.AdminAPIKey {
	out := make([]openapi.AdminAPIKey, len(in))
	for i, k := range in {
		item := openapi.AdminAPIKey{
			UserID:       k.UserID,
			UserName:     k.UserName,
			ID:           k.ID,
			Name:         k.Name,
			KeyPrefix:    k.KeyPrefix,
			Scope:        string(k.Scope),
			RequestCount: k.RequestCount,
			IsActive:     k.IsActive,
			CreatedAt:    k.CreatedAt.UnixMilli(),
			UpdatedAt:    k.UpdatedAt.UnixMilli(),
		}
		if k.ExpiresAt != nil {
			item.ExpiresAt = int64Ptr(k.ExpiresAt.UnixMilli())
		}
		if k.LastRequest != nil {
			item.LastRequestAt = int64Ptr(k.LastRequest.UnixMilli())
		}
		if k.RevokedAt != nil {
			item.RevokedAt = int64Ptr(k.RevokedAt.UnixMilli())
		}
		out[i] = item
	}
	return out
}

func apikeydomainPagination(p apikeydomain.Pagination) openapi.Pagination {
	return openapi.Pagination{Page: p.Page, Limit: p.Limit, Total: int64(p.Total), TotalPages: p.TotalPages}
}

func int64Ptr(v int64) *int64 { return &v }

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.APIKeyListResult
	_ openapi.GlobalAPIKeyStatsResult
	_ openapi.UserAPIKeyStatsResult
	_ openapi.SuccessResult
	_ openapi.ErrorResponse
)

// SuperAdminHandler handles super admin API key endpoints.
type SuperAdminHandler struct {
	service     *apikeyapp.APIKeyService
	auditLogger *audit.Logger
}

// NewSuperAdminHandler creates a new super admin API key handler.
func NewSuperAdminHandler(service *apikeyapp.APIKeyService, auditLogger *audit.Logger) *SuperAdminHandler {
	return &SuperAdminHandler{
		service:     service,
		auditLogger: auditLogger,
	}
}

// RegisterRoutes registers the super admin API key routes.
// Note: The /admin/api-keys prefix is already added by the caller in setupAdminRoutes.
func (h *SuperAdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	// r is already the /admin/api-keys group from setupAdminRoutes.
	r.GET("", h.ListAllKeys)
	r.GET("/user/:userId", h.GetUserKeys)
	r.DELETE("/:keyId", h.ForceRevokeKey)
	r.GET("/stats", h.GetGlobalStats)
	r.GET("/stats/user/:userId", h.GetUserStats)
}

// ListAllKeys handles GET /v1/admin/api-keys.
// @Summary      List all API keys
// @Description  Lists all API keys across all users (super admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        page   query int  false  "page number (default 1)"
// @Param        limit  query int  false  "page size (default 20)"
// @Param        user_id  query string  false  "filter by user ID"
// @Param        search  query string  false  "search by key name"
// @Success      200  {object}  openapi.AdminAPIKeyListResult  "API keys with user identity"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Router       /admin/api-keys [get]
func (h *SuperAdminHandler) ListAllKeys(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	userID := c.Query("user_id")
	search := c.Query("search")

	result, err := h.service.ListAllKeys(c.Request.Context(), page, limit, userID, search)
	if err != nil {
		status := apikeydomain.HTTPStatusCode(err)
		c.JSON(status, openapi.ErrorResponse{
			Error:   apikeydomain.ErrorCode(err),
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, openapi.AdminAPIKeyListResult{
		Keys:       adminKeysByUserToOpenAPI(result.Keys),
		Pagination: apikeydomainPagination(result.Pagination),
	})
}

// GetUserKeys handles GET /v1/admin/api-keys/user/:userId.
// @Summary      List user API keys
// @Description  Lists all API keys for a specific user (super admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        userId  path  string  true  "user ID"
// @Param        page   query int  false  "page number (default 1)"
// @Param        limit  query int  false  "page size (default 20)"
// @Success      200  {object}  openapi.AdminAPIKeyListResult  "user API keys"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Router       /admin/api-keys/user/{userId} [get]
func (h *SuperAdminHandler) GetUserKeys(c *gin.Context) {
	userID := c.Param("userId")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	result, err := h.service.ListKeys(c.Request.Context(), userID, page, limit)
	if err != nil {
		status := apikeydomain.HTTPStatusCode(err)
		c.JSON(status, openapi.ErrorResponse{
			Error:   apikeydomain.ErrorCode(err),
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, openapi.AdminAPIKeyListResult{
		Keys:                 apiKeysToAdminOpenAPI(result.Keys),
		Pagination:           apikeydomainPagination(result.Pagination),
		MonthlyLimit:         result.MonthlyLimit,
		KeysCreatedThisMonth: result.KeysCreatedThisMonth,
	})
}

// ForceRevokeKey handles DELETE /v1/admin/api-keys/:keyId.
// @Summary      Force revoke API key
// @Description  Force revokes an API key for any user (super admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        keyId  path  string  true  "API key ID"
// @Success      200  {object}  openapi.SuccessResult  "key revoked"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Failure      404  {object}  openapi.ErrorResponse  "key not found"
// @Router       /admin/api-keys/{keyId} [delete]
func (h *SuperAdminHandler) ForceRevokeKey(c *gin.Context) {
	keyID := c.Param("keyId")

	// Get key info for audit before revoking.
	key, err := h.service.GetKey(c.Request.Context(), "", keyID)
	if err != nil {
		status := apikeydomain.HTTPStatusCode(err)
		c.JSON(status, openapi.ErrorResponse{
			Error:   apikeydomain.ErrorCode(err),
			Message: err.Error(),
		})
		return
	}

	err = h.service.ForceRevokeKey(c.Request.Context(), keyID)
	if err != nil {
		status := apikeydomain.HTTPStatusCode(err)
		c.JSON(status, openapi.ErrorResponse{
			Error:   apikeydomain.ErrorCode(err),
			Message: err.Error(),
		})
		return
	}

	// Audit log force revocation by super admin.
	if h.auditLogger != nil {
		h.auditLogger.APIKeyRevoked(
			c.Request.Context(),
			key.UserID,
			keyID,
			key.Name,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
		)
	}

	c.Status(http.StatusNoContent)
}

// GetGlobalStats handles GET /v1/admin/api-keys/stats.
// @Summary      Get global API key stats
// @Description  Returns global API key statistics (super admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  openapi.GlobalAPIKeyStatsResult  "global stats"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Router       /admin/api-keys/stats [get]
func (h *SuperAdminHandler) GetGlobalStats(c *gin.Context) {
	stats, err := h.service.GetGlobalStats(c.Request.Context())
	if err != nil {
		status := apikeydomain.HTTPStatusCode(err)
		c.JSON(status, openapi.ErrorResponse{
			Error:   apikeydomain.ErrorCode(err),
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetUserStats handles GET /v1/admin/api-keys/stats/user/:userId.
// @Summary      Get user API key stats
// @Description  Returns API key statistics for a specific user (super admin only)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        userId  path  string  true  "user ID"
// @Success      200  {object}  openapi.UserAPIKeyStatsResult  "user stats"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      403  {object}  openapi.ErrorResponse  "super_admin required"
// @Router       /admin/api-keys/stats/user/{userId} [get]
func (h *SuperAdminHandler) GetUserStats(c *gin.Context) {
	userID := c.Param("userId")

	page := 1
	limit := 20

	result, err := h.service.ListKeys(c.Request.Context(), userID, page, limit)
	if err != nil {
		status := apikeydomain.HTTPStatusCode(err)
		c.JSON(status, openapi.ErrorResponse{
			Error:   apikeydomain.ErrorCode(err),
			Message: err.Error(),
		})
		return
	}

	totalKeys := int64(result.Pagination.Total)
	activeKeys := int64(0)
	for _, key := range result.Keys {
		if key.IsActive {
			activeKeys++
		}
	}

	c.JSON(http.StatusOK, openapi.UserAPIKeyStatsResult{
		UserID:               userID,
		TotalKeys:            totalKeys,
		ActiveKeys:           activeKeys,
		RevokedKeys:          totalKeys - activeKeys,
		KeysCreatedThisMonth: result.KeysCreatedThisMonth,
		MonthlyLimit:         result.MonthlyLimit,
	})
}

package auth

import (
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

func orgInfosToOpenAPI(in []auth.OrganizationInfo) []openapi.OrganizationInfo {
	out := make([]openapi.OrganizationInfo, len(in))
	for i, o := range in {
		out[i] = openapi.OrganizationInfo{ID: o.ID, Name: o.Name, Role: o.Role}
	}
	return out
}

func orgInfoToOpenAPIPtr(in *dto.OrganizationInfo) *openapi.OrganizationInfo {
	if in == nil {
		return nil
	}
	return &openapi.OrganizationInfo{ID: in.ID, Name: in.Name, Role: in.Role}
}

// Compile-time references for swaggo-annotated openapi DTO types.
var _ openapi.MeResult

// MeHandler handles GET /v1/auth/me.
type MeHandler struct {
	authService *auth.AuthService
	presenter   *response.Presenter
	userRepo    user.Repository
}

// NewMeHandler creates a new MeHandler.
func NewMeHandler(authService *auth.AuthService, userRepo user.Repository, presenter *response.Presenter) *MeHandler {
	return &MeHandler{authService: authService, presenter: presenter, userRepo: userRepo}
}

// Handle handles GET /v1/auth/me.
// @Summary      Get current user
// @Description  Returns the authenticated user's profile and organization context
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.MeResult  "current user"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      500  {object}  openapi.ErrorResponse  "failed to get organizations"
// @Router       /auth/me [get]
func (h *MeHandler) Handle(c *gin.Context) {
	// Get user from context (set by cookieAuth middleware).
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	// Get user's organizations.
	orgs, err := h.authService.GetUserOrganizations(c.Request.Context(), op)
	if err != nil {
		h.presenter.InternalError(c, "failed to get organizations")
		return
	}

	// Determine if organization selection is required:.
	// - 0 memberships: needs organization (create/join).
	// - Multiple memberships without valid selected org: needs selection.
	needsOrg := len(orgs) == 0

	// Find selected organization from session or last used.
	var selectedOrg *dto.OrganizationInfo
	if op.LastOrganizationID != "" && !needsOrg {
		for _, org := range orgs {
			if org.ID == op.LastOrganizationID {
				selectedOrg = &dto.OrganizationInfo{
					ID:   org.ID,
					Name: org.Name,
					Role: string(org.Role),
				}
				break
			}
		}
		// If LastOrganizationID is set but not found in active orgs,.
		// user has multiple orgs but the last one is invalid - needs selection.
		if selectedOrg == nil && len(orgs) > 1 {
			needsOrg = true
		}
	}

	// If multiple orgs exist but none is selected, needs organization selection.
	if !needsOrg && len(orgs) > 1 && selectedOrg == nil {
		needsOrg = true
	}

	// Last login is stored in its own column and is not part of the user row the
	// context carries, so read it here. A never-recorded value stays 0, which the
	// client renders as a dash rather than a fabricated time.
	var lastLoginAt int64
	if t, ok, err := h.userRepo.LastLoginAt(c.Request.Context(), op.ID); err == nil && ok {
		lastLoginAt = t.UnixMilli()
	}

	h.presenter.OK(c, openapi.MeResult{
		ID:                   op.ID,
		Email:                op.Email,
		Name:                 op.Name,
		AvatarURL:            op.AvatarURL,
		MFAEnabled:           op.MFAEnabled,
		EmailVerified:        op.EmailVerified,
		NeedsOrganization:    needsOrg,
		Organizations:        orgInfosToOpenAPI(orgs),
		LastOrganizationID:   op.LastOrganizationID,
		SelectedOrganization: orgInfoToOpenAPIPtr(selectedOrg),
		CreatedAt:            op.CreatedAt.UnixMilli(),
		LastLoginAt:          lastLoginAt,
	})
}

package deployhandlers

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/operator"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// OperatorLookup resolves the operator a credential authenticates as. API-key
// auth builds a minimal operator (id only), so the name comes from the store
// rather than the request.
type OperatorLookup interface {
	FindByID(ctx context.Context, id string) (*operator.Operator, error)
}

// OrganizationLookup resolves the org a request is scoped to, for its display
// name. A missing org is not an error the caller can act on here: the endpoint
// reports the id it was given and an empty name.
type OrganizationLookup interface {
	FindByID(ctx context.Context, id string) (*organization.Organization, error)
}

// identityResponse is what the CLI prints after `login` and in the deploy banner.
// It carries no secret: a name, a role, an org and, when a device was named, that
// device's tier.
type identityResponse struct {
	Device       *deviceSummary `json:"device,omitempty"`
	OperatorName string         `json:"operatorName"`
	Email        string         `json:"email"`
	Role         string         `json:"role"`
	OrgID        string         `json:"orgId"`
	OrgName      string         `json:"orgName"`
}

// deviceSummary is the device half of the identity, present only when the request
// named a device. SupportedABIs is what the device reported at enroll.
type deviceSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Tier          string   `json:"tier"`
	SupportedABIs []string `json:"supportedAbis,omitempty"`
}

// Identity handles GET /v1/deploy/whoami: who the caller is, which org, and — if
// `?device=` names one — that device's tier. It is the endpoint `vyzorix login`
// calls to prove a credential works and to show what it reaches.
//
// @Summary      Current operator identity for the deploy surface
// @Tags         deploy
// @Produce      json
// @Param        device  query  string  false  "device id, to include its tier"
// @Success      200  {object}  identityResponse  "identity"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Router       /deploy/whoami [get]
func (h *Handler) Identity(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	credential, isCredential := middleware.DeployCredentialFromContext(c)

	// A deploy credential names its own device; an operator may name one with the
	// query. A credential cannot be steered to a different device — that is the
	// whole point of the scope — so a mismatch is refused rather than silently
	// answered for the credential's device.
	deviceID := c.Query("device")
	if isCredential {
		if deviceID != "" && deviceID != credential.DeviceID {
			responses.RespondNotFoundError(c, "device")

			return
		}

		deviceID = credential.DeviceID
	}

	// A deploy credential has no operator: it is a CI token, not a person.
	if isCredential {
		c.JSON(200, identityResponse{
			OperatorName: "deploy credential",
			OrgID:        orgID,
			OrgName:      h.orgName(c.Request.Context(), orgID),
			Device:       h.deviceSummary(c.Request.Context(), orgID, deviceID),
		})

		return
	}

	op := middleware.GetOperatorFromContext(c)
	if op == nil {
		responses.RespondWithErrorCode(c, apperrors.CodeAuthTokenInvalid, "authentication required")

		return
	}

	// The API-key path builds an operator with only an id, so the name and email
	// come from the store. A session already carries them. The loaded record fills
	// only those two fields: replacing the operator outright would drop the
	// memberships the middleware established, which the role needs.
	name, email := op.Name, op.Email
	if h.operators != nil && (name == "" || email == "") {
		if loaded, err := h.operators.FindByID(c.Request.Context(), op.ID); err == nil && loaded != nil {
			if name == "" {
				name = loaded.Name
			}

			if email == "" {
				email = loaded.Email
			}
		}
	}

	c.JSON(200, identityResponse{
		OperatorName: name,
		Email:        email,
		Role:         h.role(c, op, orgID),
		OrgID:        orgID,
		OrgName:      h.orgName(c.Request.Context(), orgID),
		Device:       h.deviceSummary(c.Request.Context(), orgID, deviceID),
	})
}

// role returns the operator's role in the org, from the in-memory membership the
// session carries or the database the API-key path falls back to.
func (h *Handler) role(c *gin.Context, op *operator.Operator, orgID string) string {
	if member := h.membership(c, op, orgID); member != nil {
		return string(member.Role)
	}

	return ""
}

// orgName resolves an org's display name, or "" if it cannot be read. A missing
// name is not worth failing an identity call the caller has already authenticated
// for.
func (h *Handler) orgName(ctx context.Context, orgID string) string {
	if h.organizations == nil || orgID == "" {
		return ""
	}

	org, err := h.organizations.FindByID(ctx, orgID)
	if err != nil || org == nil {
		return ""
	}

	return org.Name
}

// deviceSummary resolves the named device's tier and ABIs. A device the caller
// cannot see yields no summary rather than a refusal: the identity call is not the
// authorization point, and the deploy itself still enforces the scope.
func (h *Handler) deviceSummary(ctx context.Context, orgID, deviceID string) *deviceSummary {
	if deviceID == "" || h.devices == nil {
		return nil
	}

	dev, err := h.devices.FindForOrg(ctx, deviceID, orgID)
	if err != nil || dev == nil {
		return nil
	}

	return &deviceSummary{
		ID:            dev.ID,
		Name:          dev.DeviceName,
		Tier:          tierLabel(int(dev.Tier)),
		SupportedABIs: splitABIs(dev.SupportedABIs),
	}
}

// splitABIs turns the stored comma-joined ABI list into a slice.
func splitABIs(joined string) []string {
	if joined == "" {
		return nil
	}

	parts := strings.Split(joined, ",")
	abis := make([]string, 0, len(parts))

	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			abis = append(abis, p)
		}
	}

	return abis
}

// tierLabel names a device tier for a human. The numbers are the contract between
// device.Tier and permission.Tier; the words are this endpoint's own.
func tierLabel(tier int) string {
	switch tier {
	case 3:
		return "advanced"
	case 2:
		return "rooted"
	default:
		return "unrooted"
	}
}

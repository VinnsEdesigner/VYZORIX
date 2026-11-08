package deployhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
)

// The deploy config surface: POST /v1/deploy, GET /v1/deploy/{imei},
// GET /v1/deploy/{imei}/document, GET /v1/deploy/schema/{version}.
//
// The device segment is spelled `:imei` because gin refuses two different
// parameter names at the same path position, and the device-signed routes under
// /v1/device already use `:imei`. The value is a device id, not an IMEI.

// deployResponse is what the operator is told (§3). It names the version and the
// authoriser, so the receipt can be checked against the device's config.
type deployResponse struct {
	AuthorizedBy string   `json:"authorizedBy"`
	Document     string   `json:"document,omitempty"`
	AppNames     []string `json:"apps"`
	Artifacts    int      `json:"artifacts"`
	Version      int      `json:"version"`
	DryRun       bool     `json:"dryRun"`
}

// Deploy handles POST /v1/deploy.
// @Summary      Deploy a manifest to a device
// @Description  Compiles, authorises, signs and persists a device config. With
// @Description  ?dry_run=true it compiles and signs but stores nothing.
// @Tags         deploy
// @Accept       application/toml
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        dry_run  query  bool  false  "compile and sign without persisting"
// @Success      202  {object}  deployResponse  "deployed"
// @Failure      400  {object}  openapi.ErrorResponse  "manifest invalid"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Failure      409  {object}  openapi.ErrorResponse  "device not registered"
// @Router       /deploy [post]
func (h *Handler) Deploy(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		respondMissingOrg(c)

		return
	}

	caller, memberID, ok := h.resolveCaller(c, orgID)
	if !ok {
		respondNotAMember(c)

		return
	}

	body, err := c.GetRawData()
	if err != nil {
		respondBadBody(c)

		return
	}

	result, err := h.service.Deploy(c.Request.Context(), deploy.DeployRequest{
		Caller: caller,
		OrgID:  orgID,
		// POST /v1/deploy has no device segment; the manifest's device.id is the
		// target and the service reads it. Passing the path param here (always empty
		// on this route) is what made every deploy fail authorization.
		//
		// The member is the resolved caller's, not the context operator's: a CI deploy
		// authenticated by a credential has no operator in the context, and §7.3
		// requires the config to name the human who minted the credential.
		MemberID: memberID,
		Manifest: body,
		DryRun:   c.Query("dry_run") == "true",
	})
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	// 202, not 200: the deploy is accepted and stored, and the device applies it
	// on its next connection. Nothing has run yet.
	c.JSON(http.StatusAccepted, deployResponse{
		Version:      result.Version,
		AuthorizedBy: result.AuthorizedBy,
		AppNames:     result.AppNames,
		Artifacts:    len(result.Artifacts),
		DryRun:       result.DryRun,
		Document:     result.Document,
	})
}

// currentResponse is the GET /v1/deploy/{imei} body. It carries the document,
// because the console shows it and the operator can compare it to what the device
// reports.
type currentResponse struct {
	DeviceID  string `json:"deviceId"`
	Document  string `json:"document"`
	Kind      string `json:"kind"`
	Version   int    `json:"version"`
	CreatedAt int64  `json:"createdAt"`
}

// Current handles GET /v1/deploy/{imei}.
// @Summary      Get a device's current config
// @Description  Returns the signed config the device should be running.
// @Tags         deploy
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Success      200  {object}  currentResponse  "current config"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device or config not found"
// @Router       /deploy/{imei} [get]
func (h *Handler) Current(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		respondMissingOrg(c)

		return
	}

	caller, _, ok := h.resolveCaller(c, orgID)
	if !ok {
		respondNotAMember(c)

		return
	}

	record, err := h.service.Current(c.Request.Context(), caller, orgID, c.Param("imei"))
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.JSON(http.StatusOK, currentResponse{
		DeviceID:  record.DeviceID,
		Version:   record.Version,
		Kind:      string(record.Kind),
		Document:  record.Document,
		CreatedAt: record.CreatedAt.UnixMilli(),
	})
}

// Document handles GET /v1/deploy/{imei}/document: the signed config verbatim,
// as the device fetches it. It is separate from Current so a caller that only
// needs the bytes the device verifies does not parse a JSON envelope.
// @Summary      Get a device's signed config document
// @Description  Returns the signed vyzorix.v text verbatim.
// @Tags         deploy
// @Produce      text/plain
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Success      200  {string}  string  "signed config"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device or config not found"
// @Router       /deploy/{imei}/document [get]
func (h *Handler) Document(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		respondMissingOrg(c)

		return
	}

	caller, _, ok := h.resolveCaller(c, orgID)
	if !ok {
		respondNotAMember(c)

		return
	}

	record, err := h.service.Current(c.Request.Context(), caller, orgID, c.Param("imei"))
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(record.Document))
}

package deployhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deploycredential"
)

// The deploy-credential surface: mint, list and revoke a token scoped to
// one device. Minting is the only call that returns the plaintext, once; every
// read returns the prefix and the metadata, never the token.

// mintCredentialRequest is the POST body. `name` is required so a credential can
// be revoked by intent rather than by its prefix.
type mintCredentialRequest struct {
	Name string `json:"name"`
}

// credentialResponse is one credential's metadata. It carries `prefix` — the
// non-secret display form — and never `token`.
type credentialResponse struct {
	LastUsedAt *int64 `json:"lastUsedAt,omitempty"`
	RevokedAt  *int64 `json:"revokedAt,omitempty"`
	ID         string `json:"id"`
	DeviceID   string `json:"deviceId"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	CreatedBy  string `json:"createdBy"`
	CreatedAt  int64  `json:"createdAt"`
}

// mintCredentialResponse returns the plaintext exactly once, at mint time.
type mintCredentialResponse struct {
	Token string `json:"token"`
	credentialResponse
}

// credentialListResponse is the list envelope.
type credentialListResponse struct {
	Credentials []credentialResponse `json:"credentials"`
}

// MintCredential handles POST /v1/deploy/{imei}/credentials.
// @Summary      Mint a device-scoped deploy credential
// @Description  Returns the token once. It is never returned by any GET.
// @Tags         deploy
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Param        body  body  mintCredentialRequest  true  "credential name"
// @Success      201  {object}  mintCredentialResponse  "credential minted"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid request"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Router       /deploy/{imei}/credentials [post]
func (h *Handler) MintCredential(c *gin.Context) {
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

	deviceID := c.Param("imei")

	// Minting a credential is writing a long-lived key to a device, so it takes
	// the same grant as deploying to it.
	if _, err := deploy.AuthorizeDeploy(c.Request.Context(), h.devices, caller, deviceID); err != nil {
		h.respondDeployError(c, err)

		return
	}

	var req mintCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c)

		return
	}

	token, credential, err := h.credentials.Mint(c.Request.Context(), orgID, deviceID, req.Name, memberID)
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.JSON(http.StatusCreated, mintCredentialResponse{
		credentialResponse: toCredentialResponse(credential),
		Token:              token,
	})
}

// ListCredentials handles GET /v1/deploy/{imei}/credentials.
// @Summary      List a device's deploy credentials
// @Description  Names, prefixes and revocation state. Never a token.
// @Tags         deploy
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Success      200  {object}  credentialListResponse  "credentials"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Router       /deploy/{imei}/credentials [get]
func (h *Handler) ListCredentials(c *gin.Context) {
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

	deviceID := c.Param("imei")

	if _, err := deploy.AuthorizeDeploy(c.Request.Context(), h.devices, caller, deviceID); err != nil {
		h.respondDeployError(c, err)

		return
	}

	credentials, err := h.credentials.List(c.Request.Context(), deviceID)
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	items := make([]credentialResponse, 0, len(credentials))
	for _, credential := range credentials {
		items = append(items, toCredentialResponse(credential))
	}

	c.JSON(http.StatusOK, credentialListResponse{Credentials: items})
}

// RevokeCredential handles DELETE /v1/deploy/{imei}/credentials/{id}.
// @Summary      Revoke a deploy credential
// @Description  A revoked credential is refused on its next use.
// @Tags         deploy
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Param        id    path  string  true  "credential id"
// @Success      204  "revoked"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Router       /deploy/{imei}/credentials/{id} [delete]
func (h *Handler) RevokeCredential(c *gin.Context) {
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

	deviceID := c.Param("imei")

	if _, err := deploy.AuthorizeDeploy(c.Request.Context(), h.devices, caller, deviceID); err != nil {
		h.respondDeployError(c, err)

		return
	}

	if err := h.credentials.Revoke(c.Request.Context(), deviceID, c.Param("id")); err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.Status(http.StatusNoContent)
}

// toCredentialResponse projects a credential onto the wire shape. It takes the
// domain type so a field added there cannot be forgotten here, and it never
// reads TokenHash.
func toCredentialResponse(credential *deploycredential.Credential) credentialResponse {
	response := credentialResponse{
		ID:        credential.ID,
		DeviceID:  credential.DeviceID,
		Name:      credential.Name,
		Prefix:    credential.Prefix,
		CreatedBy: credential.CreatedBy,
		CreatedAt: credential.CreatedAt.UnixMilli(),
	}

	if credential.LastUsedAt != nil {
		millis := credential.LastUsedAt.UnixMilli()
		response.LastUsedAt = &millis
	}

	if credential.RevokedAt != nil {
		millis := credential.RevokedAt.UnixMilli()
		response.RevokedAt = &millis
	}

	return response
}

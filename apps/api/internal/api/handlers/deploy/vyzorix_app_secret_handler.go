package deployhandlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// The app-secret surface: set, list names, unset, and the device's
// ciphertext fetch. A value enters on exactly one call, `set`, and leaves on
// exactly one, the device's fetch, which is wrapped to that device's key. No
// response here carries a plaintext.

// setSecretRequest is the POST body. The value is the one plaintext the server
// ever sees; it is wrapped before it leaves this handler.
type setSecretRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// setSecretResponse reports the new generation, so a device knows to re-fetch.
type setSecretResponse struct {
	App        string `json:"app"`
	Key        string `json:"key"`
	Generation int64  `json:"generation"`
}

// secretNameResponse is one secret's name and generation, never its value.
type secretNameResponse struct {
	Key        string `json:"key"`
	Generation int64  `json:"generation"`
	UpdatedAt  int64  `json:"updatedAt"`
}

// secretListResponse is the list envelope.
type secretListResponse struct {
	App     string               `json:"app"`
	Secrets []secretNameResponse `json:"secrets"`
}

// ciphertextResponse is the device's fetch result: the value wrapped to its key.
type ciphertextResponse struct {
	App        string `json:"app"`
	Key        string `json:"key"`
	Ciphertext string `json:"ciphertext"`
	Envelope   string `json:"envelope"`
	Generation int64  `json:"generation"`
}

// SetSecret handles POST /v1/deploy/{imei}/secrets/{app}.
// @Summary      Set an app secret
// @Description  Wraps the value to the device's key and stores the ciphertext.
// @Tags         deploy
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Param        app   path  string  true  "app name"
// @Param        body  body  setSecretRequest  true  "key and value"
// @Success      200  {object}  setSecretResponse  "stored"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid name or value"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Router       /deploy/{imei}/secrets/{app} [post]
func (h *Handler) SetSecret(c *gin.Context) {
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

	var req setSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c)

		return
	}

	generation, err := h.secrets.Set(c.Request.Context(), deviceID, c.Param("app"), req.Key, req.Value)
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	// The VALUE is never recorded — only that a secret was set, for which app and key,
	// and at which generation. An audit trail is read far more widely than the secret
	// store, so a value written here would be a copy of the secret with no envelope.
	h.recordAudit(c, h.auditEvent(c, orgID, deviceID, audit.ActionDeploySecretSet,
		audit.ResultSuccess, fmt.Sprintf("secret %s/%s set at generation %d", c.Param("app"), req.Key, generation)))

	c.JSON(http.StatusOK, setSecretResponse{
		App:        c.Param("app"),
		Key:        req.Key,
		Generation: generation,
	})
}

// ListSecrets handles GET /v1/deploy/{imei}/secrets/{app}: names and generations,
// never values.
// @Summary      List an app's secret names
// @Description  Returns key names and generations. Never a value.
// @Tags         deploy
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Param        app   path  string  true  "app name"
// @Success      200  {object}  secretListResponse  "secret names"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Router       /deploy/{imei}/secrets/{app} [get]
func (h *Handler) ListSecrets(c *gin.Context) {
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

	records, err := h.secrets.List(c.Request.Context(), deviceID, c.Param("app"))
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	items := make([]secretNameResponse, 0, len(records))
	for _, record := range records {
		items = append(items, secretNameResponse{
			Key:        record.Key,
			Generation: record.Generation,
			UpdatedAt:  record.UpdatedAt.UnixMilli(),
		})
	}

	c.JSON(http.StatusOK, secretListResponse{App: c.Param("app"), Secrets: items})
}

// UnsetSecret handles DELETE /v1/deploy/{imei}/secrets/{app}/{key}.
// @Summary      Remove an app secret
// @Tags         deploy
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Param        app   path  string  true  "app name"
// @Param        key   path  string  true  "secret key"
// @Success      204  "removed"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Router       /deploy/{imei}/secrets/{app}/{key} [delete]
func (h *Handler) UnsetSecret(c *gin.Context) {
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

	if err := h.secrets.Unset(c.Request.Context(), deviceID, c.Param("app"), c.Param("key")); err != nil {
		h.respondDeployError(c, err)

		return
	}

	h.recordAudit(c, h.auditEvent(c, orgID, deviceID, audit.ActionDeploySecretUnset,
		audit.ResultSuccess, "secret "+c.Param("app")+"/"+c.Param("key")+" removed"))

	c.Status(http.StatusNoContent)
}

// FetchSecret handles GET /v1/deploy/{imei}/secrets/{app}/{key} for the device
// itself. The route is on the device-signed group, so requireHMAC has already
// bound the request to :imei before this runs; there is no user here and no
// org, which is why it takes no caller.
//
// The current signed config is the gate: a device may fetch only a key its own
// config names, so the fetch cannot be turned into a read of another
// app's secret by guessing a name.
// @Summary      Fetch a secret's ciphertext (device)
// @Description  Returns the value wrapped to the device's key. Only for a key the
// @Description  current signed config names.
// @Tags         deploy
// @Produce      json
// @Param        imei  path  string  true  "device id"
// @Param        app   path  string  true  "app name"
// @Param        key   path  string  true  "secret key"
// @Success      200  {object}  ciphertextResponse  "wrapped secret"
// @Failure      403  {object}  openapi.ErrorResponse  "config does not name this secret"
// @Failure      404  {object}  openapi.ErrorResponse  "no config, or secret not set"
// @Router       /deploy/{imei}/secrets/{app}/{key} [get]
func (h *Handler) FetchSecret(c *gin.Context) {
	deviceID := c.Param("imei")

	record, err := h.service.CurrentForDevice(c.Request.Context(), deviceID)
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	secret, err := h.secrets.FetchForDevice(c.Request.Context(), deviceID, c.Param("app"), c.Param("key"), record.Document)
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.JSON(http.StatusOK, ciphertextResponse{
		App:        secret.App,
		Key:        secret.Key,
		Generation: secret.Generation,
		Ciphertext: secret.Ciphertext,
		Envelope:   crypto.SecretEnvelopeAlgorithm,
	})
}

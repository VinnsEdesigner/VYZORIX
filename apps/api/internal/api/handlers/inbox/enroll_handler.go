package inbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	devicesvc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
	"github.com/gin-gonic/gin"
)

// Compile-time references for swaggo-annotated openapi DTO types. swag resolves a
// type by name from the package's imports, so an unused import would be dropped
// by gofmt and the annotations would stop resolving.
var (
	_ openapi.ErrorResponse
)

// EnrollHandler serves the keypair registration endpoints.
//
// Enroll is public: a freshly installed device has no credential yet, so there is
// nothing to authenticate with. Claim is operator-authenticated: it is the step
// that decides who owns the device.
//
// See doc/REGISTRATION_CONTRACT.md for the flow this implements.
type EnrollHandler struct {
	devices *devicesvc.Service
	// hub delivers the config to the device over the WebSocket it is holding
	// open through onboarding. Nil leaves the config to be fetched later.
	hub *hub.Hub
	// signer signs the config and the frame carrying it. Nil means no key is
	// configured, in which case no config is issued rather than an unsigned one.
	signer *cryptohmac.ServerSigner
}

// NewEnrollHandler creates the handler. It returns nil when there is no device
// service, so the caller's nil check leaves the routes unregistered rather than
// panicking at request time.
func NewEnrollHandler(
	devices *devicesvc.Service,
	hubRef *hub.Hub,
	signer *cryptohmac.ServerSigner,
) *EnrollHandler {
	if devices == nil {
		return nil
	}

	return &EnrollHandler{devices: devices, hub: hubRef, signer: signer}
}

// enrollRequest is the wire shape. It carries no secret: the public key is the
// device's identity, and the private half never leaves the device.
type enrollRequest struct {
	AppVersion    string `json:"appVersion"`
	SecurityPatch string `json:"securityPatch"`
	Model         string `json:"model"`
	Manufacturer  string `json:"manufacturer"`
	OSVersion     string `json:"osVersion"`
	DeviceClass   string `json:"deviceClass"`
	BuildID       string `json:"buildId"`
	DeviceUUID    string `json:"deviceUuid" binding:"required"`
	PublicKey     string `json:"publicKey" binding:"required"`
	// SecretKey is the device's second public key (PURPOSE_DECRYPT), which a
	// secret is wrapped to. Optional: a device that does not support secrets
	// omits it, and setting a secret for such a device fails rather than
	// wrapping to nothing.
	SecretKey     string   `json:"secretKey"`
	SupportedABIs []string `json:"supportedAbis"`
	StorageBytes  int64    `json:"storageBytes"`
	RAMBytes      int64    `json:"ramBytes"`
	APIVersion    int      `json:"apiLevel"`
	TierHint      int      `json:"tierHint"`
}

// enrollResponse is what the device shows in its notification. Nothing secret.
type enrollResponse struct {
	DeviceID       string `json:"deviceId"`
	PairingCode    string `json:"pairingCode"`
	PairingDisplay string `json:"pairingDisplay"`
	ExpiresAt      int64  `json:"expiresAt"`
	// ExpiresInSeconds is what the device counts down. Sending the remaining time
	// as well as the absolute deadline means a device with a wrong clock still
	// expires its code correctly.
	ExpiresInSeconds int64 `json:"expiresInSeconds"`
	// PollIntervalSeconds is how often the device should ask for its config while
	// it waits to be claimed. The config normally arrives over the held WebSocket
	// the moment an operator claims the code, so this is the fallback for a device
	// whose socket is not up — it bounds how long a claimed device can sit
	// unconfigured. The server owns the value so the cadence can be tuned without
	// shipping a new APK.
	PollIntervalSeconds int64 `json:"pollIntervalSeconds"`
}

// PairingPollIntervalSeconds is the cadence a pending device should use to ask for
// its config. Ten seconds keeps the worst-case wait after a claim short while
// staying far below anything that would load the server: a device polls only until
// it is configured, and stops as soon as it has a config.
const PairingPollIntervalSeconds = 10

// Enroll handles POST /v1/device/enroll.
//
// @Summary      Enroll a new device
// @Description  Registers an unknown device as pending and returns a pairing code.
// @Tags         device
// @Accept       json
// @Produce      json
// @Param        body  body      enrollRequest   true  "device identity and facts"
// @Success      201   {object}  enrollResponse  "device enrolled, code issued"
// @Failure      400   {object}  openapi.ErrorResponse  "invalid request or public key"
// @Failure      409   {object}  openapi.ErrorResponse  "device already claimed"
// @Failure      429   {object}  openapi.ErrorResponse  "rate limited"
// @Router       /v1/device/enroll [post]
func (h *EnrollHandler) Enroll(c *gin.Context) {
	var req enrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.RespondValidationError(c, nil)

		return
	}

	result, err := h.devices.Enroll(c.Request.Context(), devicesvc.EnrollRequest{
		DeviceUUID:    req.DeviceUUID,
		PublicKeyPEM:  req.PublicKey,
		SecretKeyPEM:  req.SecretKey,
		Model:         req.Model,
		Manufacturer:  req.Manufacturer,
		OSVersion:     req.OSVersion,
		APIVersion:    req.APIVersion,
		AppVersion:    req.AppVersion,
		DeviceClass:   req.DeviceClass,
		BuildID:       req.BuildID,
		SecurityPatch: req.SecurityPatch,
		SupportedABIs: req.SupportedABIs,
		RAMBytes:      req.RAMBytes,
		StorageBytes:  req.StorageBytes,
		TierHint:      req.TierHint,
	})
	if err != nil {
		switch {
		case errors.Is(err, devicesvc.ErrPublicKeyRequired),
			errors.Is(err, devicesvc.ErrPublicKeyInvalid),
			errors.Is(err, devicesvc.ErrSecretKeyInvalid):
			_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, err.Error()))
		case errors.Is(err, devicesvc.ErrDeviceAlreadyOwned):
			responses.RespondConflictError(c, err.Error())
		default:
			// Attach the cause so RespondInternalError logs it. The response body
			// stays generic, but without this the only way to diagnose a 500 is a
			// debugger.
			_ = c.Error(err)
			responses.RespondInternalError(c)
		}

		return
	}

	remaining := result.ExpiresAt - time.Now().UnixMilli()

	c.JSON(http.StatusCreated, enrollResponse{
		DeviceID:            result.DeviceID,
		PairingCode:         result.PairingCode,
		PairingDisplay:      result.PairingDisplay,
		ExpiresAt:           result.ExpiresAt,
		ExpiresInSeconds:    remaining / 1000,
		PollIntervalSeconds: PairingPollIntervalSeconds,
	})
}

// claimRequest is the operator's action: the code they typed, plus the details
// they chose.
type claimRequest struct {
	PairingCode string `json:"pairingCode" binding:"required"`
	Name        string `json:"name"`
	TierCap     int    `json:"tierCap"`
}

// Claim handles POST /v1/device/claim.
//
// @Summary      Claim a device with a pairing code
// @Description  Binds a pending device to the caller's organization and consumes the code.
// @Tags         device
// @Accept       json
// @Produce      json
// @Param        body  body      claimRequest  true  "pairing code and assignment"
// @Success      200   {object}  map[string]any  "device claimed"
// @Failure      400   {object}  openapi.ErrorResponse  "invalid request"
// @Failure      404   {object}  openapi.ErrorResponse  "code not recognised or expired"
// @Failure      409   {object}  openapi.ErrorResponse  "device already claimed"
// @Router       /v1/device/claim [post]
func (h *EnrollHandler) Claim(c *gin.Context) {
	op := middleware.GetOperatorFromContext(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "authentication required"))

		return
	}

	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "organization context required"))

		return
	}

	var req claimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.RespondValidationError(c, nil)

		return
	}

	d, err := h.devices.Claim(c.Request.Context(), devicesvc.ClaimRequest{
		PairingCode:    req.PairingCode,
		OperatorID:     op.ID,
		OrganizationID: orgID,
		Name:           req.Name,
		TierCap:        req.TierCap,
	})
	if err != nil {
		switch {
		case errors.Is(err, devicesvc.ErrPairingCodeUnknown),
			errors.Is(err, devicesvc.ErrPairingCodeExpired):
			// One 404 for unknown and expired: distinguishing them would let a
			// caller probe which codes exist.
			responses.RespondNotFoundError(c, "pairing code")
		case errors.Is(err, devicesvc.ErrDeviceAlreadyOwned):
			responses.RespondConflictError(c, err.Error())
		default:
			responses.RespondInternalError(c)
		}

		return
	}

	// The operator sees these facts and confirms the device is theirs (REGISTRATION_CONTRACT.md
	// §3), so the whole set the device reported is returned, not just the name and model.
	c.JSON(http.StatusOK, gin.H{
		"deviceId":        d.ID,
		"name":            d.DeviceName,
		"model":           d.Model,
		"manufacturer":    d.Manufacturer,
		"osVersion":       d.OSVersion,
		"apiLevel":        d.APIVersion,
		"buildId":         d.BuildID,
		"securityPatch":   d.SecurityPatch,
		"supportedAbis":   d.SupportedABIs,
		"ramBytes":        d.RAMBytes,
		"storageBytes":    d.StorageBytes,
		"organizationId":  d.OrganizationID,
		"tier":            int(d.Tier),
		"configDelivered": h.deliverConfig(c, d.ID),
	})
}

// GetConfig handles GET /v1/device/:imei/config.
//
// The device asks for its own config by version. This exists because the config
// is pushed exactly once, at claim time, over the WebSocket the device is holding
// open through onboarding — a device that was offline at that moment would
// otherwise never receive one. It re-fetches on reconnect, and whenever the
// version it holds differs from the one reported here.
//
// It is device-authenticated (requireHMAC, the keypair path): the device proves
// possession of the key it enrolled, and can only read its own config because the
// imei is the one it signed for.
//
// @Summary      Fetch the device's own config
// @Description  Returns the signed vyzorix.v document for the device, for a device that missed the push
// @Tags         device
// @Produce      json
// @Param        imei  path  string  true  "device IMEI"
// @Success      200  {object}  map[string]any  "signed config document"
// @Failure      401  {object}  openapi.ErrorResponse  "device signature invalid"
// @Failure      404  {object}  openapi.ErrorResponse  "device not found or not claimed"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /device/{imei}/config [get]
func (h *EnrollHandler) GetConfig(c *gin.Context) {
	deviceID := c.Param("imei")
	if deviceID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "device imei is required"))

		return
	}

	if h.signer == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "no signing key configured; cannot issue a config"))

		return
	}

	envelope, err := h.devices.ConfigForDevice(c.Request.Context(), h.signer.SignDocument, deviceID)
	if err != nil {
		// A device that has not been claimed has no org and no config. The same 404
		// covers "unknown device", so the endpoint cannot be used to probe which
		// device ids exist.
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "no config for this device"))

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deviceId": envelope.DeviceID,
		"version":  envelope.Version,
		"issuedAt": envelope.IssuedAt,
		"document": envelope.Document,
	})
}

// deliverConfig signs and pushes the device's config over the WebSocket it is
// holding open through onboarding, and reports whether it was sent.
//
// A failure is not an error for the claim: the device is bound and registered
// either way, and it re-fetches on reconnect by version. Reporting the outcome
// lets the operator see that the device has not been configured yet.
func (h *EnrollHandler) deliverConfig(c *gin.Context, deviceID string) bool {
	sent, err := h.pushConfig(c.Request.Context(), deviceID)
	if err != nil {
		// Logged, not surfaced: the claim succeeded and the config is retryable.
		_ = c.Error(err)
	}

	return sent
}

// PushConfig signs and pushes the device's current config to it over the
// WebSocket, and reports whether it was sent. It is exported so a deploy can
// notify a device that is connected right now rather than waiting for its next
// reconnect (VYZORIX_DEPLOY_CONTRACT.md §7); a device that is not connected is
// not an error, because it re-fetches on reconnect.
func (h *EnrollHandler) PushConfig(ctx context.Context, deviceID string) bool {
	sent, _ := h.pushConfig(ctx, deviceID)

	return sent
}

// pushConfig builds, signs and sends the device_config frame. It returns the
// send outcome and any build error separately, so the claim path can surface an
// error while the deploy path treats both as best-effort.
func (h *EnrollHandler) pushConfig(ctx context.Context, deviceID string) (bool, error) {
	if h.hub == nil || h.signer == nil {
		return false, nil
	}

	sign := func(document string) (string, error) {
		return h.signer.SignDocument(document)
	}

	envelope, err := h.devices.ConfigForDevice(ctx, sign, deviceID)
	if err != nil {
		return false, err
	}

	frame := command.CommandFrame{
		Type:      "device_config",
		Command:   command.TypeDeviceConfig,
		Args:      []byte(fmt.Sprintf(`{"version":%d,"document":%q}`, envelope.Version, envelope.Document)),
		Timestamp: envelope.IssuedAt,
	}

	// Sign the frame so the device can verify it came from this server. The
	// document carries its own signature too; this covers the envelope, so a
	// swapped document fails the frame check.
	if err := cryptohmac.SignCommandFrameWithKey(h.signer, &frame, deviceID); err != nil {
		return false, err
	}

	return h.hub.Send(deviceID, frame), nil
}

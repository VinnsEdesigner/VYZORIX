package inbox

import (
	"errors"
	"net/http"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	devicesvc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/device"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/gin-gonic/gin"
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
}

// NewEnrollHandler creates the handler. It returns nil when there is no device
// service, so the caller's nil check leaves the routes unregistered rather than
// panicking at request time.
func NewEnrollHandler(devices *devicesvc.Service) *EnrollHandler {
	if devices == nil {
		return nil
	}

	return &EnrollHandler{devices: devices}
}

// enrollRequest is the wire shape. It carries no secret: the public key is the
// device's identity, and the private half never leaves the device.
type enrollRequest struct {
	DeviceUUID   string `json:"deviceUuid" binding:"required"`
	PublicKey    string `json:"publicKey" binding:"required"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`
	OSVersion    string `json:"osVersion"`
	AppVersion   string `json:"appVersion"`
	DeviceClass  string `json:"deviceClass"`
	TierHint     int    `json:"tierHint"`
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
}

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
		DeviceUUID:   req.DeviceUUID,
		PublicKeyPEM: req.PublicKey,
		Model:        req.Model,
		Manufacturer: req.Manufacturer,
		OSVersion:    req.OSVersion,
		AppVersion:   req.AppVersion,
		DeviceClass:  req.DeviceClass,
		TierHint:     req.TierHint,
	})
	if err != nil {
		switch {
		case errors.Is(err, devicesvc.ErrPublicKeyRequired),
			errors.Is(err, devicesvc.ErrPublicKeyInvalid):
			_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, err.Error()))
		case errors.Is(err, devicesvc.ErrDeviceAlreadyOwned):
			responses.RespondConflictError(c, err.Error())
		default:
			responses.RespondInternalError(c)
		}

		return
	}

	remaining := result.ExpiresAt - time.Now().UnixMilli()

	c.JSON(http.StatusCreated, enrollResponse{
		DeviceID:         result.DeviceID,
		PairingCode:      result.PairingCode,
		PairingDisplay:   result.PairingDisplay,
		ExpiresAt:        result.ExpiresAt,
		ExpiresInSeconds: remaining / 1000,
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

	c.JSON(http.StatusOK, gin.H{
		"deviceId":       d.ID,
		"name":           d.DeviceName,
		"model":          d.Model,
		"manufacturer":   d.Manufacturer,
		"osVersion":      d.OSVersion,
		"organizationId": d.OrganizationID,
		"tier":           int(d.Tier),
	})
}

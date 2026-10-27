package command

import (
	"errors"
	"net/http"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/openapi"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"

	"github.com/gin-gonic/gin"
)

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.CommandResultReport
	_ openapi.CommandResultAck
	_ openapi.ErrorResponse
)

// commandResultRequest is the JSON body for
// POST /v1/device/:imei/command/:dispatchId/result.
type commandResultRequest struct {
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
	Success bool   `json:"success"`
}

// ReportResult handles POST /v1/device/:imei/command/:dispatchId/result.
//
// This is the HTTP fallback for a device that finished a command while its
// WebSocket was down, or that reconnected after finishing. The primary path is
// the `command_result` frame on the device stream, which needs no round trip.
// Both paths converge on the same service call, so the lifecycle guard applies
// either way and a duplicate report is a no-op rather than an error.
//
// @Summary      Report command result
// @Description  Device reports the terminal outcome of a dispatched command (HTTP fallback for a dropped WebSocket)
// @Tags         commands
// @Accept       json
// @Produce      json
// @Param        imei        path  string  true  "device IMEI"
// @Param        dispatchId  path  string  true  "dispatch ID"
// @Param        body        body  openapi.CommandResultReport  true  "result payload"
// @Success      200  {object}  openapi.CommandResultAck  "result recorded"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      404  {object}  openapi.ErrorResponse  "command not found"
// @Failure      500  {object}  openapi.ErrorResponse  "internal error"
// @Router       /device/{imei}/command/{dispatchId}/result [post]
func (h *ExecuteHandler) ReportResult(c *gin.Context) {
	deviceID := c.Param("imei")
	dispatchID := c.Param("dispatchId")
	if deviceID == "" || dispatchID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "device imei and dispatch id are required"))

		return
	}

	var req commandResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid JSON in request body"))

		return
	}

	output := req.Output
	if output == "" {
		output = req.Error
	}

	err := h.commandService.ReportResult(c.Request.Context(), deviceID, dispatchID, req.Success, output)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, openapi.CommandResultAck{
			DispatchID: dispatchID,
			Recorded:   true,
			ServerTime: time.Now().Unix(),
		})
	case errors.Is(err, application.ErrCommandNotFound):
		// Unknown dispatch ID, or one belonging to a different device. The same
		// response is returned either way, so the endpoint cannot be used to probe
		// which dispatch IDs exist.
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "command not found"))
	default:
		h.log.Error("failed to record command result",
			"deviceId", deviceID, "dispatchId", dispatchID, "err", err)
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to record command result"))
	}
}

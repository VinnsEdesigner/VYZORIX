// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"

	"github.com/gin-gonic/gin"
)

// RecordingHandler serves terminal session recordings (asciinema casts).
//
// Without this, "recording" produced files nobody could retrieve. Access is
// deliberately narrow: the operator must be a member of the device's
// organization, and the session ID must be a well-formed UUID, so a crafted ID
// cannot reach outside the recording directory.
type RecordingHandler struct {
	devices      PTYDeviceResolver
	log          *slog.Logger
	recordingDir string
}

// NewRecordingHandler creates the recording handler. A nil resolver or an empty
// directory disables the endpoint.
func NewRecordingHandler(devices PTYDeviceResolver, recordingDir string, log *slog.Logger) *RecordingHandler {
	if log == nil {
		log = slog.Default()
	}

	return &RecordingHandler{devices: devices, recordingDir: recordingDir, log: log}
}

// Enabled reports whether recording retrieval is available.
func (h *RecordingHandler) Enabled() bool {
	return h != nil && h.devices != nil && h.recordingDir != ""
}

// Handle serves GET /v1/dashboard/device/:imei/recordings/:sessionId.
//
// The device is in the path so the org check is the same one the terminal uses:
// an operator may only read recordings for devices in their own organization.
func (h *RecordingHandler) Handle(c *gin.Context) {
	deviceID := c.Param("imei")
	sessionID := c.Param("sessionId")

	if deviceID == "" || sessionID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "device imei and session id are required"))

		return
	}

	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "organization context required"))

		return
	}

	// The device must belong to the caller's organization.
	if h.devices.DeviceOrganization(c.Request.Context(), deviceID) != orgID {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "device not found in organization"))

		return
	}

	// The session ID is the filename. Constrain it to a UUID so a value such as
	// "../../etc/passwd" cannot escape the directory, and resolve the final path
	// anyway as a second line of defence.
	if !hub.IsValidPTYSessionID(sessionID) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationInvalidFormat, "invalid session id"))

		return
	}

	path := filepath.Join(h.recordingDir, sessionID+".cast")

	// Defence in depth: confirm the resolved path is still inside the recording
	// directory before serving it.
	cleanDir := filepath.Clean(h.recordingDir)
	if !strings.HasPrefix(filepath.Clean(path), cleanDir+string(os.PathSeparator)) {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationInvalidFormat, "invalid session id"))

		return
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "recording not found"))

		return
	}

	// Casts are plain text; serve them as a download with a stable filename so a
	// browser saves rather than renders them.
	c.Header("Content-Disposition", `attachment; filename="`+sessionID+`.cast"`)
	c.Header("Content-Type", "application/x-asciicast")
	c.File(path)
}

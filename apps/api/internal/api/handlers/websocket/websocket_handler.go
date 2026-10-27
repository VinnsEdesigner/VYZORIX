// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"log/slog"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	config "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"

	"github.com/gin-gonic/gin"
)

// StreamHandler handles WebSocket connections for device streaming.
// This is the main entry point for the device streaming API.
type StreamHandler struct {
	log       *slog.Logger
	hub       *hub.Hub
	upgrader  *StreamUpgrader
	presenter *Presenter
	config    config.Config
}

// NewStreamHandler creates a new StreamHandler with all dependencies.
func NewStreamHandler(
	log *slog.Logger,
	cfg config.Config,
	h *hub.Hub,
	hmacVerifier cryptohmac.Verifier,
	auditLogger *audit.Logger,
) *StreamHandler {
	upgrader := NewStreamUpgrader(log, cfg, hmacVerifier)
	presenter := NewPresenter(auditLogger, log)

	return &StreamHandler{
		log:       log,
		config:    cfg,
		hub:       h,
		upgrader:  upgrader,
		presenter: presenter,
	}
}

// Handle handles GET /v1/device/:imei/stream.
// This is the HTTP entry point that upgrades to WebSocket.
func (h *StreamHandler) Handle(c *gin.Context) {
	deviceID := c.Param("imei")
	if deviceID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "device imei required"))
		return
	}

	// Attempt WebSocket upgrade.
	_, client, err := h.upgrader.Upgrade(c, deviceID)
	if err != nil {
		// Log failure via presenter.
		if h.config.EnforceHMAC {
			h.presenter.LogHMACFailed(c.Request.Context(), deviceID)
		} else {
			h.presenter.LogUpgradeFailed(c.Request.Context(), deviceID, err.Error())
		}

		return
	}

	// Defensive: ensure client is not nil.
	if client == nil {
		h.presenter.LogUpgradeFailed(c.Request.Context(), deviceID, "client was nil after upgrade")
		return
	}

	// Set hub on client.
	client.Hub = h.hub

	// Register client with hub.
	h.hub.Register(client)

	// Log connection and audit.
	h.presenter.LogConnect(c.Request.Context(), deviceID)
	h.presenter.AuditDeviceConnect(c.Request.Context(), deviceID)

	// Start pumps - ReadPump blocks, so run WritePump in goroutine.
	go client.WritePump()
	client.ReadPump()
}

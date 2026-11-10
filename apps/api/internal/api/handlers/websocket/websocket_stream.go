// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	config "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// StreamUpgrader handles WebSocket upgrade logic including origin validation and device signature verification.
type StreamUpgrader struct {
	originValidator *infraauth.OriginValidator
	log             *slog.Logger
	upgrader        websocket.Upgrader
	hmacVerifier    cryptohmac.Verifier
	// devicePublicKeys resolves a device's registered signing key, which is what
	// the connect handshake is verified against.
	devicePublicKeys DevicePublicKeyProvider
	config           config.Config
	allowDevAuth     bool
}

// DevicePublicKeyProvider resolves a device's PEM public key, or "" when the
// device is unknown or has not enrolled.
type DevicePublicKeyProvider func(deviceID string) string

// NewStreamUpgrader creates a new StreamUpgrader.
func NewStreamUpgrader(
	log *slog.Logger,
	cfg config.Config,
	hmacVerifier cryptohmac.Verifier,
	devicePublicKeys DevicePublicKeyProvider,
) *StreamUpgrader {
	originValidator := infraauth.NewOriginValidator(cfg.AllowedOrigins)
	originValidator.SetLogger(log)

	allowDevAuth := false
	if cfg.Env != "production" && !cfg.EnforceHMAC {
		log.Warn("WebSocket authentication is disabled - this is insecure and should not be used in production",
			"env", cfg.Env,
			"enforceHMAC", cfg.EnforceHMAC,
		)
		allowDevAuth = true
	}

	return &StreamUpgrader{
		originValidator:  originValidator,
		hmacVerifier:     hmacVerifier,
		devicePublicKeys: devicePublicKeys,
		config:           cfg,
		log:              log,
		allowDevAuth:     allowDevAuth,
		upgrader: websocket.Upgrader{
			CheckOrigin:      originValidator.CheckOrigin(),
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// Upgrade attempts to upgrade an HTTP connection to WebSocket.
// Returns the WebSocket connection and registered client on success.
// Returns an error and sends appropriate HTTP response on failure.
func (u *StreamUpgrader) Upgrade(c *gin.Context, deviceID string) (*websocket.Conn, *hub.Client, error) {

	// In development, only skip if allowDevAuth is true (meaning EnforceHMAC=false was set).
	if u.config.Env == "production" || u.config.EnforceHMAC {
		// Use query param based verification for WebSocket since headers can't be set during upgrade.
		if err := u.verifyDeviceConnect(c.Request, deviceID); err != nil {
			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "WebSocket connect signature verification failed"))
			return nil, nil, err
		}
	} else if !u.allowDevAuth {
		_ = c.Error(
			// This case handles production with EnforceHMAC=false (shouldn't happen but is defensive).
			apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "WebSocket authentication is required"))
		return nil, nil, http.ErrNotSupported
	}

	// Perform WebSocket upgrade.
	conn, err := u.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, nil, err
	}

	// Create client for hub.
	client := &hub.Client{
		DeviceID: deviceID,
		Conn:     conn,
		Send:     make(chan command.CommandFrame, 32),
		PtySend:  make(chan []byte, hub.DefaultPTYClientBuffer),
		Hub:      nil, // Will be set by caller.

		Done: make(chan struct{}),
	}

	return conn, client, nil
}

// verifyDeviceConnect verifies the connect handshake against the device's
// registered public key.
//
// The device signs the connect canonical string with the private key it generated
// at first boot; the server verifies with the public half it received at enroll.
// No shared secret, so nothing has to be delivered — which is why this replaces
// the HMAC handshake.
func (u *StreamUpgrader) verifyDeviceConnect(r *http.Request, deviceID string) error {
	if u.devicePublicKeys == nil {
		return errors.New("no device public key provider configured")
	}

	publicKey := u.devicePublicKeys(deviceID)
	if publicKey == "" {
		return errors.New("device has no registered public key; enroll first")
	}

	ts := r.URL.Query().Get("sig_timestamp")
	nonce := r.URL.Query().Get("sig_nonce")
	sig := r.URL.Query().Get("sig_signature")
	if ts == "" || nonce == "" || sig == "" {
		return errors.New("missing signature query parameters")
	}

	// Freshness, then replay. Without these a captured handshake would replay
	// forever: the signature alone is valid for all time. The window matches the
	// device HTTP path so a device with a skewed clock behaves the same on both.
	window := 5 * time.Minute
	if u.hmacVerifier.Window > 0 {
		window = u.hmacVerifier.Window
	}

	milli, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("connect timestamp is not Unix milliseconds")
	}

	now := time.Now()
	signedAt := time.UnixMilli(milli)
	if signedAt.Before(now.Add(-window)) || signedAt.After(now.Add(window)) {
		return errors.New("connect timestamp outside the replay window")
	}

	if u.hmacVerifier.Nonces != nil && !u.hmacVerifier.Nonces.Use("connect:"+deviceID+":"+nonce, now) {
		return errors.New("connect nonce replayed")
	}

	if !cryptohmac.VerifyConnectSignature(publicKey, deviceID, ts, nonce, sig) {
		return errors.New("connect signature invalid")
	}

	return nil
}

package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	"github.com/gin-gonic/gin"
)

// healthHandler handles health check requests.
func (s *Server) healthHandler(c *gin.Context) {
	dbOk := false

	var dbErr error

	if s.DB != nil {
		if err := s.DB.Ping(); err == nil {
			dbOk = true
		} else {
			dbErr = err
		}
	}

	if s.lifecycleManager != nil {
		workers := s.lifecycleManager.Health()
		allHealthy := true
		for _, healthy := range workers {
			if !healthy {
				allHealthy = false
			}
		}
		if !allHealthy {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "degraded",
				"workers":   workers,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"workers":   workers,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	connectedDevices := 0
	if s.hub != nil {
		connectedDevices = s.hub.ClientCount()
	}

	version := ""
	if v, err := s.readVersion(); err == nil {
		version = v
	}

	status := http.StatusOK
	if !dbOk {
		status = http.StatusServiceUnavailable
	}

	response := map[string]any{
		"ok":               dbOk,
		"database":         map[bool]string{true: "ok", false: "down"}[dbOk],
		"dbOk":             dbOk,
		"serverTime":       time.Now().UnixMilli(),
		"connectedDevices": connectedDevices,
		"version":          version,
	}
	if dbErr != nil {
		response["dbError"] = dbErr.Error()
	}

	c.JSON(status, response)
}

// readVersion reads the version from version.json file.
func (s *Server) readVersion() (string, error) {
	body, err := os.ReadFile(filepath.Join(s.config.DataDir, "version.json"))
	if err != nil {
		return "", err
	}

	var v struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(body, &v); err != nil {
		return "", err
	}

	return v.Version, nil
}

// versionHandler serves the version.json file.
func (s *Server) versionHandler(c *gin.Context) {
	s.serveStaticFile(c, filepath.Join(s.config.DataDir, "version.json"), "application/json; charset=utf-8")
}

// changelogHandler serves the changelog.json file.
func (s *Server) changelogHandler(c *gin.Context) {
	s.serveStaticFile(c, filepath.Join(s.config.DataDir, "changelog.json"), "application/json; charset=utf-8")
}

// apkHandler serves APK download files.
func (s *Server) apkHandler(c *gin.Context) {
	name := c.Param("name")
	s.serveDownload(c, strings.TrimPrefix(name, "/"))
}

// binHandler serves binary download files.
func (s *Server) binHandler(c *gin.Context) {
	name := c.Param("name")
	s.serveDownload(c, strings.TrimPrefix(name, "/"))
}

// serveDownload serves a file download with APK headers.
func (s *Server) serveDownload(c *gin.Context, name string) {
	if name == "" || strings.ContainsAny(name, "/\\") {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid filename"))
		return
	}

	c.Header("Content-Type", "application/vnd.android.package-archive")
	c.Header("Cache-Control", "no-store")
	c.File(filepath.Join(s.config.BinDir, name))
}

// serveStaticFile serves a static file with specified content type.
func (s *Server) serveStaticFile(c *gin.Context, path, ct string) {
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "no-store")
	c.File(path)
}

// dashboardHandler serves the dashboard or landing page.
func (s *Server) dashboardHandler(c *gin.Context) {
	path := c.Request.URL.Path

	// All non-API paths → serve the landing page first (fallback).
	// SSR middleware handles "/" when enabled (proxies to SSR or serves fallback landing.html).
	clean := strings.TrimPrefix(filepath.Clean(path), "/")
	if clean == "." || clean == "" {
		clean = "landing.html"
	}

	candidate := filepath.Join(s.config.PublicDir, clean)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		c.File(candidate)
		return
	}

	c.File(filepath.Join(s.config.PublicDir, "landing.html"))
}

// requireHMAC authenticates a device's HTTP request with the keypair model.
//
// The name is kept for the route wiring's sake, but the mechanism is no longer
// HMAC: the device signs with the private key it generated at first boot, and the
// server verifies with the public half it received at enroll. There is no shared
// secret, so there is nothing to deliver and nothing to look up per device beyond
// the registered public key. See doc/REGISTRATION_CONTRACT.md §5.
//
// The signed string is crypto.DeviceCanonicalString, deliberately distinct from
// the server's own canonical form, so a device signature can never be replayed as
// a server one.
func (s *Server) requireHMAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		// A WebSocket upgrade is authenticated by the upgrader from query parameters,
		// not from these headers: an upgrade carries no body and its signature is
		// checked in verifyDeviceConnect. Skipping here keeps exactly one authenticator
		// per transport instead of two that can disagree.
		if isWebSocketUpgrade(c.Request) {
			c.Next()

			return
		}

		deviceID := c.Param("imei")
		if deviceID == "" {
			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "device ID required for device signature verification"))
			c.Abort()

			return
		}

		body, err := readAndRestoreBody(c.Request)
		if err != nil {
			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "Invalid request"))
			c.Abort()

			return
		}

		if err := s.verifyDeviceRequest(c, deviceID, body); err != nil {
			if !s.config.EnforceHMAC {
				// Development bypass, matching the previous behaviour: a device that
				// cannot sign yet is allowed through, which is what makes local work
				// possible without a provisioned key.
				c.Next()

				return
			}

			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "Invalid request"))
			c.Abort()

			return
		}

		c.Set("hmac_body", body)
		c.Next()
	}
}

// verifyDeviceRequest checks a device's request signature against its registered
// public key, with the timestamp window and nonce replay guard the previous HMAC
// path had.
func (s *Server) verifyDeviceRequest(c *gin.Context, deviceID string, body []byte) error {
	ts := c.GetHeader("X-Vyzorix-Timestamp")
	nonce := c.GetHeader("X-Vyzorix-Nonce")
	sig := c.GetHeader("X-Vyzorix-Signature")
	if ts == "" || nonce == "" || sig == "" {
		return errors.New("missing device signature headers")
	}

	// The device must be enrolled: no registered key means nothing to verify against.
	publicKey, err := s.devicePublicKey(c, deviceID)
	if err != nil || publicKey == "" {
		return errors.New("device has no registered public key")
	}

	// Window and replay protection, shared with the rest of the device surface so
	// there is one nonce cache rather than several that can disagree.
	window := 5 * time.Minute
	if s.hmacVerifier != nil && s.hmacVerifier.Window > 0 {
		window = s.hmacVerifier.Window
	}

	milli, parseErr := strconv.ParseInt(ts, 10, 64)
	if parseErr != nil {
		return errors.New("timestamp is not Unix milliseconds")
	}

	now := time.Now()
	signedAt := time.UnixMilli(milli)
	if signedAt.Before(now.Add(-window)) || signedAt.After(now.Add(window)) {
		return errors.New("timestamp outside the replay window")
	}

	if s.hmacVerifier != nil && s.hmacVerifier.Nonces != nil &&
		!s.hmacVerifier.Nonces.Use(deviceID+":"+nonce, now) {
		return errors.New("replayed nonce")
	}

	return cryptohmac.VerifyDeviceSignature(
		publicKey,
		c.Request.Method,
		c.Request.URL.Path,
		deviceID,
		ts,
		nonce,
		body,
		sig,
	)
}

// isWebSocketUpgrade reports whether the request is a WebSocket handshake.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// devicePublicKey resolves the device's registered PEM public key, or "" when the
// device is unknown or has not enrolled.
func (s *Server) devicePublicKey(c *gin.Context, deviceID string) (string, error) {
	if s.DeviceRepo == nil {
		return "", nil
	}

	dev, err := s.DeviceRepo.FindByID(c.Request.Context(), deviceID)
	if err != nil || dev == nil {
		return "", err
	}

	return dev.PublicKey, nil
}

// readAndRestoreBody reads the request body and puts it back so the handler can
// read it again. A device signature covers the body, so it must be read here
// before the handler binds it.
func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}

// requireStrictHMAC checks the operator's strictHmac setting and enforces HMAC.
func (s *Server) requireStrictHMAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		op := middleware.GetOperatorFromContext(c)
		if op == nil {
			c.Next()
			return
		}

		if !op.ClientSettings.StrictHmac {
			c.Next()
			return
		}

		_, err := s.hmacVerifier.ReadAndVerifyHTTP(c.Request)
		if err != nil {
			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "strictHmac is enabled: HMAC signature verification failed"))
			c.Abort()

			return
		}

		c.Next()
	}
}

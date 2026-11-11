package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.UpdaterVersionManifestResult
	_ openapi.UpdaterCheckResult
	_ openapi.DownloadProgressResult
	_ openapi.ErrorResponse
)

// VersionManifest represents the OTA version manifest served to Android clients.
type VersionManifest struct {
	Version      string `json:"version"`
	APKFilename  string `json:"apk_filename"`
	APKSHA256    string `json:"apk_sha256"`
	ReleaseNotes string `json:"release_notes"`
	VersionCode  int    `json:"version_code"`
	APKSizeBytes int64  `json:"apk_size_bytes"`
}

// Handler handles OTA update distribution endpoints.
type Handler struct {
	log              *slog.Logger
	manifest         *VersionManifest
	progress         *ProgressStore
	latestFromGitHub func(ctx context.Context) (*VersionManifest, error)
	dataDir          string
	binDir           string
	config           config.Config
}

// NewHandler creates a new UpdaterHandler.
func NewHandler(log *slog.Logger, cfg config.Config) *Handler {
	return &Handler{
		log:      log,
		config:   cfg,
		dataDir:  cfg.DataDir,
		binDir:   cfg.BinDir,
		progress: NewProgressStore(DefaultProgressTTL),
	}
}

// SetGitHubSource wires the GitHub release as the source of the current version.
//
// Without it the server answers from the baked `version.json`, which is only as
// current as the last deploy. With it, `check-update` reports the newest published
// agent release, so shipping an update no longer requires a server redeploy.
func (h *Handler) SetGitHubSource(latest func(ctx context.Context) (*VersionManifest, error)) {
	h.latestFromGitHub = latest
}

// ProgressStore exposes the store for the periodic sweep worker.
func (h *Handler) ProgressStore() *ProgressStore { return h.progress }

// loadManifest loads the version manifest for APK integrity verification.

func (h *Handler) loadManifest() error {
	data, err := os.ReadFile(filepath.Join(h.dataDir, "version.json"))
	if err != nil {
		return err
	}

	var manifest VersionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}

	h.manifest = &manifest
	return nil
}

// ensureManifestLoaded loads the manifest if not already loaded.
func (h *Handler) ensureManifestLoaded() {
	if h.manifest == nil {
		if err := h.loadManifest(); err != nil {
			h.log.Warn("failed to load manifest for APK verification", "error", err)
		}
	}
}

// Version handles GET /api/v1/version.
// @Summary      OTA version manifest
// @Description  Returns the current OTA version manifest for Android clients
// @Tags         updater
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.UpdaterVersionManifestResult  "version manifest"
// @Failure      404  {object}  openapi.ErrorResponse  "manifest not found"
// @Router       /version [get]
func (h *Handler) Version(c *gin.Context) {
	h.log.Info("ota version request", "path", c.Request.URL.Path)
	h.serveJSON(c, filepath.Join(h.dataDir, "version.json"))
}

// Changelog handles GET /api/v1/changelog.
// @Summary      OTA changelog
// @Description  Returns the release changelog served to Android clients
// @Tags         updater
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.UpdateChangelogResult  "changelog"
// @Failure      404  {object}  openapi.ErrorResponse  "changelog not found"
// @Router       /changelog [get]
func (h *Handler) Changelog(c *gin.Context) {
	h.log.Info("ota changelog request", "path", c.Request.URL.Path)
	h.serveJSON(c, filepath.Join(h.dataDir, "changelog.json"))
}

// APK handles GET /api/v1/apk/:filename.
// @Summary      Download APK
// @Description  Serves APK files with optional Range support for resume
// @Tags         updater
// @Produce      octet-stream
// @Param        name  path  string  true  "APK filename"
// @Success      200  {file}  binary  "APK binary"
// @Failure      400  {object}  openapi.ErrorResponse  "filename required"
// @Failure      404  {object}  openapi.ErrorResponse  "file not found"
// @Router       /apk/{name} [get]
func (h *Handler) APK(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "filename required"))
		return
	}
	// Remove leading slash if present.
	filename = strings.TrimPrefix(filename, "/")
	h.serveAPK(c, filename)
}

// Bin handles GET /bin/:filename.
// @Summary      Download binary artifact
// @Description  Serves binary artifacts (same as APK but different path prefix)
// @Tags         updater
// @Produce      octet-stream
// @Param        name  path  string  true  "binary filename"
// @Success      200  {file}  binary  "binary artifact"
// @Failure      400  {object}  openapi.ErrorResponse  "filename required"
// @Failure      404  {object}  openapi.ErrorResponse  "file not found"
// @Router       /bin/{name} [get]
func (h *Handler) Bin(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "filename required"))
		return
	}
	// Remove leading slash if present.
	filename = strings.TrimPrefix(filename, "/")
	h.serveAPK(c, filename)
}

// currentManifest resolves the version the server should report as current.
//
// The GitHub release is preferred: it is the one place a release is published, so
// reading it means the server always knows the newest agent without a redeploy. The
// baked manifest is the fallback: a deployment with no GitHub configured, or a
// transient GitHub failure, still answers with the last-known version rather than
// erroring.
func (h *Handler) currentManifest(ctx context.Context) (*VersionManifest, error) {
	if h.latestFromGitHub != nil {
		m, err := h.latestFromGitHub(ctx)
		if err == nil && m != nil {
			return m, nil
		}
		if err != nil {
			h.log.Warn("could not resolve the current version from GitHub; using the baked manifest", "err", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(h.dataDir, "version.json"))
	if err != nil {
		return nil, err
	}

	var manifest VersionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// CheckUpdate handles GET /api/v1/check-update.
// @Summary      Check for update
// @Description  Checks if an update is available for a device by version code
// @Tags         updater
// @Accept       json
// @Produce      json
// @Param        version_code  query int  false  "client version code"
// @Success      200  {object}  openapi.UpdaterCheckResult  "update check result"
// @Failure      500  {object}  openapi.ErrorResponse  "manifest not loaded"
// @Router       /check-update [get]
func (h *Handler) CheckUpdate(c *gin.Context) {
	versionCode := c.Query("version_code")
	h.log.Info("update check", "version_code", versionCode)

	// The current version is the newest published agent release, so shipping an
	// update does not require a server redeploy. The baked manifest is the
	// fallback for a deployment with no GitHub source configured (or when GitHub
	// is unreachable): a stale answer is better than a 500.
	manifest, err := h.currentManifest(c.Request.Context())
	if err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "cannot resolve the current version"))
		return
	}

	clientCode := 0
	if versionCode != "" {
		var parseErr error
		clientCode, parseErr = strconv.Atoi(versionCode)
		if parseErr != nil {
			h.log.Warn("invalid client version code", "versionCode", versionCode, "err", parseErr)
		}
	}

	// Flatten response: update_available at top level, version fields directly accessible.
	c.JSON(http.StatusOK, openapi.UpdaterCheckResult{
		UpdateAvailable: manifest.VersionCode > clientCode,
		Version:         manifest.Version,
		VersionCode:     manifest.VersionCode,
		ApkFilename:     manifest.APKFilename,
		ApkSha256:       manifest.APKSHA256,
		ReleaseNotes:    manifest.ReleaseNotes,
		ApkSizeBytes:    manifest.APKSizeBytes,
	})
}

// DownloadProgress handles POST /api/v1/download-progress.
// @Summary      Report download progress
// @Description  Tracks download progress for analytics
// @Tags         updater
// @Accept       json
// @Produce      json
// @Param        body  body  openapi.DownloadProgressRequest  true  "download progress"
// @Success      200  {object}  openapi.DownloadProgressResult  "recorded"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid request"
// @Router       /download-progress [post]
func (h *Handler) DownloadProgress(c *gin.Context) {
	var req struct {
		DeviceID    string `json:"deviceId"`
		Filename    string `json:"filename"`
		Progress    int    `json:"progress"`
		BytesLoaded int64  `json:"bytesLoaded"`
		TotalBytes  int64  `json:"totalBytes"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "Invalid request"))
		return
	}

	// A device id is required: progress is keyed by it, so a report without one
	// cannot be stored or read back. The byte counts are what the read endpoint
	// reports; `progress` is accepted but the percent is derived from the bytes so
	// the two cannot disagree.
	if req.DeviceID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "deviceId is required"))
		return
	}

	h.progress.Record(req.DeviceID, req.Filename, req.BytesLoaded, req.TotalBytes)

	h.log.Info("download progress",
		"deviceId", req.DeviceID,
		"filename", req.Filename,
		"bytesLoaded", req.BytesLoaded,
		"totalBytes", req.TotalBytes,
		"percent", req.Progress,
	)
	c.JSON(http.StatusOK, openapi.DownloadProgressResult{Recorded: true})
}

// GetDownloadProgress handles GET /v1/download-progress/:deviceId.
//
// The read half of the report endpoint: an operator (or the dashboard) asks how far
// along a device's update is. Before this, `POST /download-progress` only logged the
// report — the server accepted progress and could not answer for it, so the data had
// no consumer.
//
// @Summary      Read a device's update download progress
// @Description  Returns the latest reported OTA download progress for a device
// @Tags         updater
// @Produce      json
// @Param        deviceId  path  string  true  "device id"
// @Success      200  {object}  map[string]any  "progress, or downloading=false when none"
// @Failure      400  {object}  openapi.ErrorResponse  "deviceId required"
// @Router       /download-progress/{deviceId} [get]
func (h *Handler) GetDownloadProgress(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "deviceId is required"))
		return
	}

	progress, ok := h.progress.Get(deviceID)
	if !ok {
		// No live download. This is a normal answer, not a 404: a device that is
		// not updating is a valid state, and the caller wants to know that rather
		// than to distinguish "never reported" from "finished".
		c.JSON(http.StatusOK, gin.H{"deviceId": deviceID, "downloading": false})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deviceId":    progress.DeviceID,
		"downloading": true,
		"filename":    progress.Filename,
		"bytesLoaded": progress.BytesLoaded,
		"totalBytes":  progress.TotalBytes,
		"percent":     progress.Percent(),
		"updatedAt":   progress.UpdatedAt.UnixMilli(),
	})
}

// serveJSON serves a static JSON file with proper headers.
func (h *Handler) serveJSON(c *gin.Context, path string) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.File(path)
}

// serveAPK serves an APK file with proper headers and Range support.
// Security: Verifies APK hash against manifest before serving.

func (h *Handler) serveAPK(c *gin.Context, filename string) {
	// Use filepath.Base to normalize path and prevent traversal attacks.
	// This handles .., %2F, %5C and other encoded sequences.
	baseFilename := filepath.Base(filename)

	// Ensure the result is a valid filename (not empty, no path separators).
	if baseFilename == "" || baseFilename == "." || baseFilename == ".." ||
		strings.ContainsAny(baseFilename, "/\\") {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "invalid filename"))
		return
	}

	fpath := filepath.Join(h.binDir, baseFilename)

	// This ensures the APK hasn't been tampered with since the manifest was created.
	h.ensureManifestLoaded()
	if h.manifest != nil && h.manifest.APKFilename == baseFilename && h.manifest.APKSHA256 != "" {
		actualHash, err := h.computeFileHash(fpath)
		if err != nil {
			h.log.Error("failed to compute APK hash", "error", err, "file", fpath)
			_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to verify APK integrity"))
			return
		}

		// Constant-time comparison to prevent timing attacks.
		if !secureCompare(h.manifest.APKSHA256, actualHash) {
			h.log.Error("APK integrity check failed - hash mismatch with manifest",
				"file", baseFilename,
				"manifest_hash", h.manifest.APKSHA256,
				"actual_hash", actualHash,
			)
			_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "APK integrity verification failed - file may have been tampered with"))

			return
		}

		h.log.Info("APK integrity verified against manifest", "file", baseFilename)
	}

	// Also verify if client provides expected SHA256 (client-side verification).
	if clientHash := c.GetHeader("X-APK-SHA256"); clientHash != "" {
		actualHash, err := h.computeFileHash(fpath)
		if err != nil {
			h.log.Error("failed to compute APK hash", "error", err, "file", fpath)
			_ = c.Error(apperrors.NewServerError(apperrors.CodeInternalServerError, "failed to verify APK integrity"))
			return
		}

		// Constant-time comparison to prevent timing attacks.
		if !secureCompare(clientHash, actualHash) {
			h.log.Warn("APK hash mismatch", "expected", clientHash, "actual", actualHash)
			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzInsufficientPermissions, "APK integrity check failed"))

			return
		}

		h.log.Info("APK hash verified", "file", filename, "hash", actualHash[:16]+"...")
	}

	if c.Request.Method == http.MethodGet {
		c.Header("Content-Type", "application/vnd.android.package-archive")
		c.Header("Cache-Control", "no-store")
		c.File(fpath)

		return
	}

	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "application/vnd.android.package-archive")
		c.File(fpath)

		return
	}
	_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "GET or HEAD required"))
}

// computeFileHash computes the SHA256 hash of a file.
func (h *Handler) computeFileHash(fpath string) (string, error) {
	file, err := os.Open(fpath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// secureCompare performs a constant-time comparison of two strings.
// This prevents timing attacks when comparing sensitive values like hashes.
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

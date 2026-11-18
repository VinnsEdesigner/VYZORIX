package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	application "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	appauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

// Compile-time references for swaggo-annotated openapi DTO types.
var (
	_ openapi.ProfileActionResult
	_ openapi.ProfileSummaryResult
)

// Avatar upload constraints. The avatar is shown at ~40px, so a generous cap
// still rejects an accidental full-resolution photo, and only image types a
// browser renders are accepted (never SVG, which can carry script).
const (
	maxAvatarBytes = 2 << 20 // 2 MiB
	avatarDirName  = "avatars"
)

var avatarContentTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// ProfileHandler serves the profile page's read and mutation endpoints.
type ProfileHandler struct {
	authService    *appauth.AuthService
	sessionManager *infraauth.SessionManager
	userRepo       user.Repository
	auditLogger    *audit.Logger
	presenter      *response.Presenter
	config         config.Config
}

// NewProfileHandler creates a new ProfileHandler.
func NewProfileHandler(
	authService *appauth.AuthService,
	sessionManager *infraauth.SessionManager,
	userRepo user.Repository,
	auditLogger *audit.Logger,
	presenter *response.Presenter,
	cfg config.Config,
) *ProfileHandler {
	return &ProfileHandler{
		authService:    authService,
		sessionManager: sessionManager,
		userRepo:       userRepo,
		auditLogger:    auditLogger,
		presenter:      presenter,
		config:         cfg,
	}
}

func (h *ProfileHandler) getUserID(c *gin.Context) (string, error) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		return "", application.ErrUnauthorized
	}

	return op.ID, nil
}

// Summary handles GET /v1/auth/me/profile-summary.
// @Summary      Profile summary
// @Description  Counts for the profile page: active sessions and audited actions
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.ProfileSummaryResult  "counts"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/profile-summary [get]
func (h *ProfileHandler) Summary(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	// Active sessions only — the count the profile shows is "signed-in devices".
	var activeSessions int
	if h.sessionManager != nil {
		if sessions, err := h.sessionManager.ListActiveSessions(c.Request.Context(), userID); err == nil {
			activeSessions = len(sessions)
		}
	}

	// Audited actions. A missing audit store counts 0 rather than failing the
	// whole summary — the profile still renders.
	var actions int
	if h.auditLogger != nil {
		if n, err := h.auditLogger.CountForUser(c.Request.Context(), userID); err == nil {
			actions = n
		}
	}

	h.presenter.OK(c, openapi.ProfileSummaryResult{
		ActiveSessions: activeSessions,
		Actions:        actions,
	})
}

// UploadAvatar handles POST /v1/auth/me/avatar (multipart/form-data, field "file").
// @Summary      Upload avatar
// @Description  Stores an uploaded profile image and points the user's avatar at it
// @Tags         auth
// @Accept       multipart/form-data
// @Produce      json
// @Success      200  {object}  openapi.ProfileActionResult  "updated profile"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid image"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      500  {object}  openapi.ErrorResponse  "storage error"
// @Router       /auth/me/avatar [post]
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		h.presenter.Unauthorized(c, "not authenticated")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.presenter.BadRequest(c, "an image file is required")
		return
	}

	if fileHeader.Size > maxAvatarBytes {
		h.presenter.BadRequest(c, "image is too large (max 2 MB)")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.presenter.InternalError(c, "could not read the uploaded file")
		return
	}
	defer func() { _ = file.Close() }()

	// Sniff the real content type from the bytes rather than trusting the
	// multipart header, which a client controls. 512 bytes is
	// http.DetectContentType's window.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		h.presenter.InternalError(c, "could not read the uploaded file")
		return
	}
	head = head[:n]

	contentType := strings.TrimSpace(strings.Split(http.DetectContentType(head), ";")[0])
	ext, ok := avatarContentTypes[contentType]
	if !ok {
		h.presenter.BadRequest(c, "unsupported image type (png, jpeg, gif or webp)")
		return
	}

	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		h.presenter.InternalError(c, "could not read the uploaded file")
		return
	}

	// A random filename, not the client's: the uploaded name is untrusted input
	// that could contain path separators or a misleading extension.
	name, err := randomAvatarName(userID, ext)
	if err != nil {
		h.presenter.InternalError(c, "could not store the image")
		return
	}

	dir := filepath.Join(h.config.PublicDir, avatarDirName)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		h.presenter.InternalError(c, "could not store the image")
		return
	}

	dst := filepath.Join(dir, name)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		h.presenter.InternalError(c, "could not store the image")
		return
	}

	if _, copyErr := io.Copy(out, io.LimitReader(file, maxAvatarBytes)); copyErr != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		h.presenter.InternalError(c, "could not store the image")
		return
	}
	if closeErr := out.Close(); closeErr != nil {
		_ = os.Remove(dst)
		h.presenter.InternalError(c, "could not store the image")
		return
	}

	avatarURL := fmt.Sprintf("/%s/%s", avatarDirName, name)

	op, err := h.authService.UpdateUserAvatar(c.Request.Context(), userID, avatarURL)
	if err != nil {
		_ = os.Remove(dst)
		h.presenter.InternalError(c, "could not update the profile")
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.SettingsChanged(c.Request.Context(), userID, "profile", "avatar_updated", c.ClientIP(), c.GetHeader("User-Agent"), nil)
	}

	h.presenter.OK(c, openapi.ProfileActionResult{
		ID:        op.ID,
		Name:      op.Name,
		AvatarURL: op.AvatarURL,
		Email:     op.Email,
	})
}

// randomAvatarName builds "<userID>-<random><ext>". The user prefix keeps a
// directory listing legible; the random suffix stops one upload from guessing
// another's URL and makes a replacement a new URL (so caches and the browser
// refetch rather than showing the previous image).
func randomAvatarName(userID, ext string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, userID)

	return fmt.Sprintf("%s-%s%s", safe, hex.EncodeToString(buf), ext), nil
}

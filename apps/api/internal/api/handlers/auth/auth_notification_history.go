package auth

import (
	"strconv"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	infranotification "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/notification"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

const (
	historyDefaultLimit = 25
	historyMaxLimit     = 100
)

// NotificationHistoryHandler serves a user's own notification audit trail.
type NotificationHistoryHandler struct {
	repo      *infranotification.Repository
	userRepo  user.Repository
	presenter *response.Presenter
}

// NewNotificationHistoryHandler creates a new NotificationHistoryHandler.
func NewNotificationHistoryHandler(repo *infranotification.Repository, userRepo user.Repository, presenter *response.Presenter) *NotificationHistoryHandler {
	return &NotificationHistoryHandler{repo: repo, userRepo: userRepo, presenter: presenter}
}

// List handles GET /v1/auth/me/notifications/history.
// @Summary      List notification history
// @Description  Returns the current user's delivered and failed notifications, newest first
// @Tags         notifications
// @Produce      json
// @Param        channel  query string false "filter by channel (email, slack, webhook)"
// @Param        status   query string false "filter by outcome (sent, failed)"
// @Param        limit    query int    false "page size (max 100)"
// @Param        offset   query int    false "page offset"
// @Success      200  {object}  openapi.NotificationHistoryResult  "history"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/notifications/history [get]
func (h *NotificationHistoryHandler) List(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}

	limit := clampInt(c.Query("limit"), historyDefaultLimit, 1, historyMaxLimit)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<30)
	channel := c.Query("channel")

	// status narrows to one outcome; anything else means "both".
	var success *bool
	switch c.Query("status") {
	case "sent":
		value := true
		success = &value
	case "failed":
		value := false
		success = &value
	}

	entries, err := h.repo.ListByUser(c.Request.Context(), op.ID, channel, success, limit, offset)
	if err != nil {
		h.presenter.InternalError(c, "failed to load notification history")
		return
	}
	total, err := h.repo.CountByUser(c.Request.Context(), op.ID, channel, success)
	if err != nil {
		h.presenter.InternalError(c, "failed to count notification history")
		return
	}

	readAt, err := h.userRepo.GetNotificationsReadAt(c.Request.Context(), op.ID)
	if err != nil {
		h.presenter.InternalError(c, "failed to load notification read state")
		return
	}
	unread, err := h.repo.CountByUserSince(c.Request.Context(), op.ID, readAt)
	if err != nil {
		h.presenter.InternalError(c, "failed to count unread notifications")
		return
	}

	result := openapi.NotificationHistoryResult{
		Entries: make([]openapi.NotificationHistoryEntry, 0, len(entries)),
		Total:   total,
		Unread:  unread,
		Limit:   limit,
		Offset:  offset,
	}
	for _, entry := range entries {
		result.Entries = append(result.Entries, openapi.NotificationHistoryEntry{
			ID:        entry.ID,
			EventType: entry.EventType,
			Channel:   entry.Channel,
			DeviceID:  entry.DeviceID,
			ErrorMsg:  entry.ErrorMsg,
			SentAt:    entry.SentAt.UnixMilli(),
			Success:   entry.Success,
		})
	}

	h.presenter.OK(c, result)
}

// MarkRead handles POST /v1/auth/me/notifications/history/read.
// @Summary      Mark notifications read
// @Description  Stamps the read watermark so the unread count returns to zero
// @Tags         notifications
// @Produce      json
// @Success      200  {object}  openapi.MarkNotificationsReadResult  "unread count"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/notifications/history/read [post]
func (h *NotificationHistoryHandler) MarkRead(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}

	if err := h.userRepo.SetNotificationsReadAt(c.Request.Context(), op.ID, time.Now().UnixMilli()); err != nil {
		h.presenter.InternalError(c, "failed to mark notifications read")
		return
	}

	h.presenter.OK(c, openapi.MarkNotificationsReadResult{Unread: 0})
}

// clampInt parses value as an int and clamps it to [low, high]; a bad value falls
// back to fallback.
func clampInt(value string, fallback, low, high int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < low {
		return low
	}
	if parsed > high {
		return high
	}

	return parsed
}

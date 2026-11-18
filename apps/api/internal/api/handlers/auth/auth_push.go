package auth

import (
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/notification"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/push"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

// PushHandler serves the browser Web Push subscription endpoints.
type PushHandler struct {
	repo      push.Repository
	notifier  *notification.Service
	presenter *response.Presenter
	publicKey string
}

// NewPushHandler creates a new PushHandler.
func NewPushHandler(repo push.Repository, notifier *notification.Service, publicKey string, presenter *response.Presenter) *PushHandler {
	return &PushHandler{repo: repo, notifier: notifier, presenter: presenter, publicKey: publicKey}
}

// PublicKey handles GET /v1/auth/me/notifications/push/public-key.
// @Summary      Get the Web Push public key
// @Description  Returns the VAPID public key the browser needs to subscribe, or enabled=false when push is not configured
// @Tags         notifications
// @Produce      json
// @Success      200  {object}  openapi.PushPublicKeyResult  "public key"
// @Router       /auth/me/notifications/push/public-key [get]
func (h *PushHandler) PublicKey(c *gin.Context) {
	h.presenter.OK(c, openapi.PushPublicKeyResult{
		PublicKey: h.publicKey,
		Enabled:   h.publicKey != "",
	})
}

// Subscribe handles POST /v1/auth/me/notifications/push/subscribe.
// @Summary      Register a browser push subscription
// @Description  Stores the browser's PushSubscription so notifications can be delivered to it
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        body  body  openapi.PushSubscriptionRequest  true  "push subscription"
// @Success      200  {object}  openapi.SuccessResult  "subscribed"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid subscription"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/notifications/push/subscribe [post]
func (h *PushHandler) Subscribe(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}

	var req openapi.PushSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid subscription")
		return
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		h.presenter.BadRequest(c, "endpoint and keys are required")
		return
	}

	if err := h.repo.Upsert(c.Request.Context(), &push.Subscription{
		Endpoint:  req.Endpoint,
		UserID:    op.ID,
		P256dh:    req.Keys.P256dh,
		Auth:      req.Keys.Auth,
		UserAgent: c.GetHeader("User-Agent"),
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		h.presenter.InternalError(c, "failed to store subscription")
		return
	}

	h.presenter.OK(c, openapi.SuccessResult{Success: true})
}

// Test handles POST /v1/auth/me/notifications/push/test.
// @Summary      Send a test push notification
// @Description  Delivers a test notification to every browser the user has subscribed
// @Tags         notifications
// @Produce      json
// @Success      200  {object}  openapi.PushTestResult  "delivery count"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/notifications/push/test [post]
func (h *PushHandler) Test(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}
	if h.notifier == nil {
		h.presenter.InternalError(c, "push is not configured")
		return
	}

	delivered, err := h.notifier.SendTestPush(c.Request.Context(), op.ID)
	if err != nil {
		h.presenter.InternalError(c, "failed to send test push")
		return
	}

	h.presenter.OK(c, openapi.PushTestResult{Delivered: delivered})
}

// Unsubscribe handles POST /v1/auth/me/notifications/push/unsubscribe.
// @Summary      Remove a browser push subscription
// @Description  Deletes a stored PushSubscription (the user disabling this browser)
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        body  body  openapi.PushSubscriptionRequest  true  "push subscription"
// @Success      200  {object}  openapi.SuccessResult  "unsubscribed"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/notifications/push/unsubscribe [post]
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}

	var req openapi.PushSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Endpoint == "" {
		h.presenter.BadRequest(c, "endpoint is required")
		return
	}

	// Only the owner may remove a subscription. The endpoint is unguessable, but
	// this still scopes the delete to the caller's own rows.
	subs, err := h.repo.ListByUser(c.Request.Context(), op.ID)
	if err != nil {
		h.presenter.InternalError(c, "failed to load subscriptions")
		return
	}
	for _, sub := range subs {
		if sub.Endpoint == req.Endpoint {
			if err := h.repo.DeleteByEndpoint(c.Request.Context(), req.Endpoint); err != nil {
				h.presenter.InternalError(c, "failed to remove subscription")
				return
			}
			break
		}
	}

	h.presenter.OK(c, openapi.SuccessResult{Success: true})
}

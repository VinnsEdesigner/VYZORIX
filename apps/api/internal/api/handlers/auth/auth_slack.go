package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	infraoauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security/oauth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"

	"github.com/gin-gonic/gin"
)

// Compile-time reference for the swaggo-annotated response DTO.
var _ openapi.SlackChannelsResult

// SlackConnect starts the Slack link-up for the signed-in user.
// @Summary      Connect Slack
// @Description  Redirects to Slack so the user can authorise Vyzorix
// @Tags         notifications
// @Success      307  {string}  string  "redirect to Slack"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Failure      501  {object}  openapi.ErrorResponse  "Slack not configured"
// @Router       /auth/me/notifications/slack/connect [get]
func (h *OAuthHandler) SlackConnect(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}
	if h.config.SlackOAuthClientID == "" || h.config.SlackOAuthClientSecret == "" {
		h.presenter.NotImplemented(c, "Slack is not configured on this server")
		return
	}
	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	redirectURL := h.config.FrontendURL + "/profile?tab=notifications&slack=connected"
	if _, err := h.oauthStateRepo.CreateForUser(c.Request.Context(), state, redirectURL, "slack", op.ID); err != nil {
		h.presenter.InternalError(c, "failed to create OAuth state")
		return
	}

	callbackURL := h.config.BaseURL + "/v1/auth/me/notifications/slack/callback"
	// Bot scopes the notification channel needs: post to channels the bot is in,
	// and read the channel list so the user can pick one.
	scope := "chat:write,channels:read,groups:read"
	slackURL := fmt.Sprintf(
		"https://slack.com/oauth/v2/authorize?client_id=%s&scope=%s&redirect_uri=%s&state=%s",
		url.QueryEscape(h.config.SlackOAuthClientID),
		url.QueryEscape(scope),
		url.QueryEscape(callbackURL),
		url.QueryEscape(state),
	)
	c.Redirect(http.StatusTemporaryRedirect, slackURL)
}

// SlackCallback completes the Slack link-up and stores the workspace + token.
// @Summary      Slack OAuth callback
// @Description  Exchanges the code and stores the linked Slack workspace, then redirects back to the console
// @Tags         notifications
// @Success      307  {string}  string  "redirect back to the console"
// @Router       /auth/me/notifications/slack/callback [get]
func (h *OAuthHandler) SlackCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		h.presenter.BadRequest(c, "missing code or state")
		return
	}
	redirectURL, stateID, userID, err := h.oauthStateRepo.ValidateWithUser(c.Request.Context(), state)
	if err != nil || userID == "" {
		h.presenter.BadRequest(c, "invalid or expired OAuth state")
		return
	}
	_ = h.oauthStateRepo.Delete(c.Request.Context(), stateID)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	token, err := infraoauth.ExchangeSlackCode(ctx, code, infraoauth.SlackConfig{
		ClientID:     h.config.SlackOAuthClientID,
		ClientSecret: h.config.SlackOAuthClientSecret,
		RedirectURI:  h.config.BaseURL + "/v1/auth/me/notifications/slack/callback",
	})
	if err != nil {
		h.logger.Error("slack token exchange failed", "error", err)
		c.Redirect(http.StatusTemporaryRedirect, redirectURL+"&slack=error")
		return
	}

	if err := h.settingsSvc.LinkSlackWorkspace(ctx, userID, token.AccessToken, token.Team.ID, token.Team.Name); err != nil {
		h.logger.Error("failed to store slack link-up", "error", err)
		c.Redirect(http.StatusTemporaryRedirect, redirectURL+"&slack=error")
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// SlackChannels lists the channels the linked bot can post to.
// @Summary      List Slack channels
// @Description  Returns the channels the linked Slack bot can post to
// @Tags         notifications
// @Produce      json
// @Success      200  {object}  openapi.SlackChannelsResult  "channels"
// @Failure      401  {object}  openapi.ErrorResponse  "not authenticated"
// @Router       /auth/me/notifications/slack/channels [get]
func (h *OAuthHandler) SlackChannels(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		h.presenter.Unauthorized(c, "authentication required")
		return
	}
	settings, err := h.settingsSvc.GetNotificationSettings(c.Request.Context(), op.ID)
	if err != nil {
		h.presenter.InternalError(c, "failed to load notification settings")
		return
	}
	if settings.Slack.AccessToken == "" {
		h.presenter.OK(c, openapi.SlackChannelsResult{Channels: []openapi.SlackChannel{}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	channels, err := infraoauth.ListSlackChannels(ctx, settings.Slack.AccessToken)
	if err != nil {
		h.presenter.InternalError(c, "failed to list Slack channels")
		return
	}
	result := openapi.SlackChannelsResult{Channels: make([]openapi.SlackChannel, 0, len(channels))}
	for _, ch := range channels {
		result.Channels = append(result.Channels, openapi.SlackChannel{ID: ch.ID, Name: ch.Name})
	}
	h.presenter.OK(c, result)
}

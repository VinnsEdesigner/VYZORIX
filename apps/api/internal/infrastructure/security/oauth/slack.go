package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SlackConfig holds the Slack app credentials used for the link-up.
type SlackConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// SlackTokenResponse is the oauth.v2.access response.
type SlackTokenResponse struct {
	Team struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		ID    string `json:"id"`
		Scope string `json:"scope"`
	} `json:"authed_user"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	BotUserID   string `json:"bot_user_id"`
	AppID       string `json:"app_id"`
	OK          bool   `json:"ok"`
}

// SlackChannel is a channel the bot can post to.
type SlackChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ExchangeSlackCode exchanges an authorization code for a bot access token.
func ExchangeSlackCode(ctx context.Context, code string, config SlackConfig) (*SlackTokenResponse, error) {
	form := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"code":          {code},
		"redirect_uri":  {config.RedirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/oauth.v2.access", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack token exchange failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var tokenResp SlackTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode slack response: %w", err)
	}
	if !tokenResp.OK {
		return nil, fmt.Errorf("slack rejected the exchange: %s", tokenResp.Error)
	}

	return &tokenResp, nil
}

// ListSlackChannels returns the channels the bot token can see.
func ListSlackChannels(ctx context.Context, token string) ([]SlackChannel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://slack.com/api/conversations.list?limit=200&types=public_channel,private_channel", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var decoded struct {
		Error    string         `json:"error"`
		Channels []SlackChannel `json:"channels"`
		OK       bool           `json:"ok"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if !decoded.OK {
		return nil, fmt.Errorf("slack rejected the channel list: %s", decoded.Error)
	}

	return decoded.Channels, nil
}

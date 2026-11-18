// Package notification provides the notification service for sending alerts.
package notification

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/push"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/email"
	infranotification "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/notification"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/webhook"
)

// EventType represents the type of event that triggered a notification.
type EventType string

const (
	EventTypeThresholdBreach     EventType = "threshold_breach"
	EventTypeDeviceOffline       EventType = "device_offline"
	EventTypeDeviceOnline        EventType = "device_online"
	EventTypeUpdateAvailable     EventType = "update_available"
	EventTypeCommandFailed       EventType = "command_failed"
	EventTypeRegistrationRequest EventType = "registration_request"
	EventTypeError               EventType = "error"
)

// EventData contains the data for a notification event.
type EventData struct {
	Timestamp     time.Time
	AlertType     string
	Threshold     string
	UserID        string
	UserEmail     string
	UserName      string
	EventType     EventType
	CurrentValue  string
	DeviceName    string
	CommandName   string
	FailureReason string
	UpdateVersion string
	RequesterName string
	ErrorMessage  string
	DeviceID      string
}

// Service handles sending notifications based on user preferences.
type Service struct {
	userRepo        user.Repository
	emailService    *email.Service
	webhookClient   *webhook.Client
	auditRepo       *infranotification.Repository
	logger          *slog.Logger
	baseURL         string
	slackWebhookURL string
	httpClient      *http.Client
	pushRepo        push.Repository
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubject    string
}

// NewService creates a new notification service.
func NewService(
	userRepo user.Repository,
	emailSvc *email.Service,
	webhookClient *webhook.Client,
	auditRepo *infranotification.Repository,
	baseURL string,
	logger *slog.Logger,
) *Service {
	return NewServiceWithSlack(userRepo, emailSvc, webhookClient, auditRepo, baseURL, "", logger)
}

// NewServiceWithSlack is NewService with the Slack incoming-webhook URL supplied
// by config rather than read from the environment.
func NewServiceWithSlack(
	userRepo user.Repository,
	emailSvc *email.Service,
	webhookClient *webhook.Client,
	auditRepo *infranotification.Repository,
	baseURL string,
	slackWebhookURL string,
	logger *slog.Logger,
) *Service {
	return &Service{
		userRepo:        userRepo,
		emailService:    emailSvc,
		webhookClient:   webhookClient,
		auditRepo:       auditRepo,
		baseURL:         baseURL,
		logger:          logger,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		slackWebhookURL: slackWebhookURL,
	}
}

// SetPushDelivery enables browser Web Push: the subscription store and the VAPID
// keypair used to sign deliveries. Without it the push channel is a no-op, which
// keeps a deployment that has not configured VAPID working.
func (s *Service) SetPushDelivery(repo push.Repository, publicKey, privateKey, subject string) {
	s.pushRepo = repo
	s.vapidPublicKey = publicKey
	s.vapidPrivateKey = privateKey
	s.vapidSubject = subject
}

// http returns the HTTP client, creating one lazily so a Service built without
// the constructor (tests) still works.
func (s *Service) http() *http.Client {
	if s.httpClient == nil {
		s.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return s.httpClient
}

// SendNotification sends a notification based on user preferences and event type.
func (s *Service) SendNotification(ctx context.Context, data EventData) error {
	// Get user's notification settings if not provided.
	if data.UserEmail == "" || data.UserName == "" {
		op, err := s.userRepo.FindByID(ctx, data.UserID)
		if err != nil {
			return fmt.Errorf("failed to get operator: %w", err)
		}
		if data.UserEmail == "" {
			data.UserEmail = op.Email
		}
		if data.UserName == "" {
			data.UserName = op.Name
		}
	}

	// Get user's notification settings.
	settings, err := s.userRepo.GetNotifications(ctx, data.UserID)
	if err != nil {
		return fmt.Errorf("failed to get notification settings: %w", err)
	}

	// Check if notifications are enabled.
	if !settings.Enabled {
		s.logger.Debug("notifications disabled for operator", "userID", data.UserID)
		return nil
	}

	// Send email if the channel is on AND the event type is enabled. The channels
	// list is the gate; shouldSendEmail is the per-event filter.
	if s.channelEnabled(settings, "email") && s.eventEnabled(settings, data.EventType) {
		if err := s.sendEmail(ctx, data, settings); err != nil {
			s.logger.Error("failed to send email notification", "error", err, "userID", data.UserID)
			// Don't return error - try the other channels even if email fails.
		}
	}

	// Send to Slack if the channel is on AND the event is one Slack should carry.
	// command_failed is excluded from Slack on purpose: a busy fleet would post a
	// message per failed command. Email keeps it (digest-friendly).
	if s.channelEnabled(settings, "slack") && data.EventType != EventTypeCommandFailed && s.eventEnabled(settings, data.EventType) {
		if err := s.sendSlack(ctx, data, settings); err != nil {
			s.logger.Error("failed to send slack notification", "error", err, "userID", data.UserID)
		}
	}

	// Send a browser push if the channel is on AND the event is one push carries.
	// command_failed is excluded for the same reason as Slack: a busy fleet would
	// raise a notification per failed command.
	if s.channelEnabled(settings, "push") && data.EventType != EventTypeCommandFailed && s.eventEnabled(settings, data.EventType) {
		if err := s.sendPush(ctx, data); err != nil {
			s.logger.Error("failed to send push notification", "error", err, "userID", data.UserID)
		}
	}

	// Send webhook if configured.
	if err := s.sendWebhook(ctx, data, settings); err != nil {
		s.logger.Error("failed to send webhook notification", "error", err, "userID", data.UserID)
		// Don't return error for webhook failures.
	}

	return nil
}

// channelEnabled reports whether a delivery channel is switched on for the user.
// Channels is the authoritative list; an empty list means "email only" so an
// account that never touched the setting still gets email.
func (s *Service) channelEnabled(settings *user.NotificationSettings, channel string) bool {
	if len(settings.Channels) == 0 {
		return channel == "email"
	}
	for _, c := range settings.Channels {
		if c == channel {
			return true
		}
	}
	return false
}

// notificationRecipient returns the address to email: the user's custom address
// when set, otherwise their account email.
func notificationRecipient(data EventData, settings *user.NotificationSettings) string {
	if settings.EmailAddress != "" {
		return settings.EmailAddress
	}
	return data.UserEmail
}

// eventEnabled reports whether the per-event toggle allows this event type.
// It reads the email event set, which is the shared per-event configuration for
// every channel (email and Slack alike).
func (s *Service) eventEnabled(settings *user.NotificationSettings, eventType EventType) bool {
	switch eventType {
	case EventTypeThresholdBreach:
		return settings.Email.ThresholdBreach
	case EventTypeDeviceOffline:
		return settings.Email.DeviceOffline
	case EventTypeDeviceOnline:
		return settings.Email.DeviceOnline
	case EventTypeUpdateAvailable:
		return settings.Email.UpdateAvailable
	case EventTypeCommandFailed:
		return settings.Email.CommandFailed
	case EventTypeRegistrationRequest:
		return settings.Email.RegistrationRequest
	case EventTypeError:
		// Error events are always sent if notifications are enabled.
		return true
	default:
		return false
	}
}

// shouldSendWebhook checks if webhook should be sent for this event type.
func (s *Service) shouldSendWebhook(settings *user.NotificationSettings, eventType EventType) bool {
	if !settings.Webhook.Enabled || settings.Webhook.URL == "" {
		return false
	}

	// If no types specified, send all.
	if len(settings.Webhook.Types) == 0 {
		return true
	}

	// Check if this event type is in the allowed types.
	for _, t := range settings.Webhook.Types {
		if t == string(eventType) {
			return true
		}
	}

	return false
}

// sendEmail sends an email notification to the user's chosen address.
func (s *Service) sendEmail(ctx context.Context, data EventData, settings *user.NotificationSettings) error {
	if s.emailService == nil {
		s.logger.Warn("email service not configured")
		return nil
	}

	to := notificationRecipient(data, settings)
	emailData := email.NotificationData{
		UserName:      data.UserName,
		DeviceID:      data.DeviceID,
		DeviceName:    data.DeviceName,
		AlertType:     data.AlertType,
		CurrentValue:  data.CurrentValue,
		Threshold:     data.Threshold,
		CommandName:   data.CommandName,
		FailureReason: data.FailureReason,
		UpdateVersion: data.UpdateVersion,
		RequesterName: data.RequesterName,
		ErrorMessage:  data.ErrorMessage,
		Timestamp:     data.Timestamp.Format("2006-01-02 15:04:05 MST"),
		BaseURL:       s.baseURL,
	}

	var err error

	switch data.EventType {
	case EventTypeThresholdBreach:
		err = s.emailService.SendThresholdBreachEmail(ctx, to, emailData)
	case EventTypeDeviceOffline:
		err = s.emailService.SendDeviceOfflineEmail(ctx, to, emailData)
	case EventTypeDeviceOnline:
		err = s.emailService.SendDeviceOnlineEmail(ctx, to, emailData)
	case EventTypeUpdateAvailable:
		err = s.emailService.SendUpdateAvailableEmail(ctx, to, emailData)
	case EventTypeRegistrationRequest:
		err = s.emailService.SendRegistrationRequestEmail(ctx, to, emailData)
	case EventTypeError:
		err = s.emailService.SendErrorAlertEmail(ctx, to, emailData)
	default:
		s.logger.Warn("unknown event type for email", "eventType", data.EventType)
		return nil
	}

	if err != nil {
		s.auditNotification(ctx, data, "email", false, err.Error())
		return err
	}

	s.auditNotification(ctx, data, "email", true, "")
	s.logger.Info("email notification sent",
		"eventType", data.EventType,
		"userID", data.UserID,
		"deviceID", data.DeviceID,
	)

	return nil
}

// SendTestPush delivers a single "this is a test" push to every subscription the
// user has registered, and reports how many were attempted. It is used by the
// console's "Send test notification" button so a user can prove delivery without
// waiting for a real event.
func (s *Service) SendTestPush(ctx context.Context, userID string) (int, error) {
	if s.pushRepo == nil || s.vapidPrivateKey == "" {
		return 0, nil
	}

	subs, err := s.pushRepo.ListByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}

	payload, err := json.Marshal(map[string]string{
		"title": "Vyzorix test notification",
		"body":  "Browser push is working. You will be notified here.",
		"url":   "/notifications",
	})
	if err != nil {
		return 0, err
	}

	subject := s.vapidSubject
	if subject == "" {
		subject = "mailto:alerts@vyzorix.local"
	}

	delivered := 0
	for _, sub := range subs {
		resp, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
		}, &webpush.Options{
			Subscriber:      subject,
			VAPIDPublicKey:  s.vapidPublicKey,
			VAPIDPrivateKey: s.vapidPrivateKey,
			TTL:             3600,
		})
		if err != nil {
			s.logger.Warn("test push failed", "error", err, "userID", userID)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			_ = s.pushRepo.DeleteByEndpoint(ctx, sub.Endpoint)
			continue
		}
		delivered++
	}

	return delivered, nil
}

// sendPush delivers a browser Web Push notification to every subscription the
// user has registered. A subscription the push service reports as gone (404/410)
// is removed, so a stale browser does not accumulate.
func (s *Service) sendPush(ctx context.Context, data EventData) error {
	if s.pushRepo == nil || s.vapidPrivateKey == "" {
		return nil
	}

	subs, err := s.pushRepo.ListByUser(ctx, data.UserID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]string{
		"title": s.slackText(data),
		"body":  data.DeviceName,
		"url":   "/notifications",
	})
	if err != nil {
		return err
	}

	subject := s.vapidSubject
	if subject == "" {
		subject = "mailto:alerts@vyzorix.local"
	}

	for _, sub := range subs {
		resp, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
		}, &webpush.Options{
			Subscriber:      subject,
			VAPIDPublicKey:  s.vapidPublicKey,
			VAPIDPrivateKey: s.vapidPrivateKey,
			TTL:             3600,
		})
		if err != nil {
			s.logger.Warn("push delivery failed", "error", err, "userID", data.UserID)
			continue
		}
		_ = resp.Body.Close()
		// 404/410 means the subscription is no longer valid; drop it.
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			_ = s.pushRepo.DeleteByEndpoint(ctx, sub.Endpoint)
		}
	}

	return nil
}

// sendSlack delivers a notification to Slack, using the bot token captured by
// the Slack link-up when present, otherwise the configured incoming webhook.
func (s *Service) sendSlack(ctx context.Context, data EventData, settings *user.NotificationSettings) error {
	text := s.slackText(data)

	// Preferred path: the linked Slack workspace, posting as the bot the user
	// authorised during the OAuth link-up.
	if settings.Slack.Connected && settings.Slack.AccessToken != "" && settings.Slack.ChannelID != "" {
		return s.postSlackBotMessage(ctx, settings.Slack.AccessToken, settings.Slack.ChannelID, text)
	}

	// Fallback: the org-wide incoming webhook, when one is configured.
	if s.slackWebhookURL != "" && s.webhookClient != nil {
		body, err := json.Marshal(map[string]string{"text": text})
		if err != nil {
			return err
		}
		return s.webhookClient.SendRaw(ctx, s.slackWebhookURL, "", body)
	}

	return fmt.Errorf("slack channel enabled but no workspace linked and no webhook configured")
}

// slackText renders the event as a short Slack message.
func (s *Service) slackText(data EventData) string {
	switch data.EventType {
	case EventTypeThresholdBreach:
		return fmt.Sprintf(":warning: *Threshold breach* on %s — %s is %s (limit %s)", data.DeviceName, data.AlertType, data.CurrentValue, data.Threshold)
	case EventTypeDeviceOffline:
		return fmt.Sprintf(":red_circle: *Device offline* — %s", data.DeviceName)
	case EventTypeDeviceOnline:
		return fmt.Sprintf(":large_green_circle: *Device online* — %s", data.DeviceName)
	case EventTypeUpdateAvailable:
		return fmt.Sprintf(":arrow_up: *Update available* — %s (version %s)", data.DeviceName, data.UpdateVersion)
	case EventTypeRegistrationRequest:
		return fmt.Sprintf(":inbox_tray: *Registration request* from %s", data.RequesterName)
	case EventTypeError:
		return fmt.Sprintf(":rotating_light: *Error* — %s", data.ErrorMessage)
	default:
		return fmt.Sprintf(":bell: Notification: %s", data.EventType)
	}
}

// postSlackBotMessage posts a message with chat.postMessage using a bot token.
func (s *Service) postSlackBotMessage(ctx context.Context, token, channel, text string) error {
	payload, err := json.Marshal(map[string]string{"channel": channel, "text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.http().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	// Slack returns 200 with {"ok":false,"error":"..."} on a logical failure.
	var decoded struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err == nil && !decoded.OK {
		return fmt.Errorf("slack error: %s", decoded.Error)
	}

	return nil
}

// sendWebhook sends a webhook notification.
func (s *Service) sendWebhook(ctx context.Context, data EventData, settings *user.NotificationSettings) error {
	if s.webhookClient == nil {
		s.logger.Warn("webhook client not configured")
		return nil
	}

	if !s.shouldSendWebhook(settings, data.EventType) {
		return nil
	}

	payload := &webhook.Payload{
		Type:      webhook.EventType(data.EventType),
		Timestamp: data.Timestamp,
		DeviceID:  data.DeviceID,
		UserID:    data.UserID,
		Data: map[string]interface{}{
			"deviceName":    data.DeviceName,
			"alertType":     data.AlertType,
			"currentValue":  data.CurrentValue,
			"threshold":     data.Threshold,
			"commandName":   data.CommandName,
			"failureReason": data.FailureReason,
			"updateVersion": data.UpdateVersion,
			"requesterName": data.RequesterName,
			"errorMessage":  data.ErrorMessage,
		},
	}

	err := s.webhookClient.Send(ctx, settings.Webhook.URL, settings.Webhook.Secret, payload)
	if err != nil {
		s.auditNotification(ctx, data, "webhook", false, err.Error())
		return err
	}

	s.auditNotification(ctx, data, "webhook", true, "")
	s.logger.Info("webhook notification sent",
		"eventType", data.EventType,
		"userID", data.UserID,
		"deviceID", data.DeviceID,
		"webhookURL", settings.Webhook.URL,
	)

	return nil
}

// auditNotification logs a notification to the audit trail.
func (s *Service) auditNotification(ctx context.Context, data EventData, channel string, success bool, errorMsg string) {
	if s.auditRepo == nil {
		return
	}

	entry := &infranotification.AuditEntry{
		ID:        generateID(),
		UserID:    data.UserID,
		EventType: string(data.EventType),
		Channel:   channel,
		DeviceID:  data.DeviceID,
		Success:   success,
		ErrorMsg:  errorMsg,
		SentAt:    time.Now(),
	}

	if err := s.auditRepo.LogEntry(ctx, entry); err != nil {
		s.logger.Error("failed to audit notification", "error", err)
	}
}

// generateID generates a unique ID for audit entries.
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "audit_" + hex.EncodeToString(b)
}

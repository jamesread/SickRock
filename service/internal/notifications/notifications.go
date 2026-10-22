package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// NotificationService handles sending notifications via various channels
type NotificationService struct {
	repo             *repo.Repository
	telegramBotToken string // Telegram bot API token (from environment)
}

// NewNotificationService creates a new notification service
func NewNotificationService(repo *repo.Repository) *NotificationService {
	return &NotificationService{
		repo:             repo,
		telegramBotToken: getTelegramBotToken(),
	}
}

// getTelegramBotToken retrieves the Telegram bot token from environment
func getTelegramBotToken() string {
	// Read from environment variable SICKROCK_TELEGRAM_BOT_TOKEN
	return os.Getenv("SICKROCK_TELEGRAM_BOT_TOKEN")
}

// SendNotification sends a notification for a specific event
func (ns *NotificationService) SendNotification(ctx context.Context, eventCode string, data map[string]any) error {
	// Get all subscriptions for this event
	subscriptions, err := ns.repo.GetSubscriptionsForEvent(ctx, eventCode)
	if err != nil {
		return fmt.Errorf("failed to get subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		log.WithField("event_code", eventCode).Debug("No subscriptions found for event")
		return nil
	}

	// Build notification message
	message := ns.buildMessage(eventCode, data)

	for _, sub := range subscriptions {
		ns.sendOneSubscription(ctx, eventCode, message, data, sub.Channel)
	}
	return nil
}

func (ns *NotificationService) sendOneSubscription(ctx context.Context, eventCode, message string, data map[string]any, channel repo.UserNotificationChannel) {
	if err := ns.sendToChannel(ctx, channel, message, data); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"event_code":   eventCode,
			"channel_id":   channel.ID,
			"channel_type": channel.ChannelType,
		}).Error("Failed to send notification")
	}
}

// buildMessage creates a human-readable message from event code and data
func (ns *NotificationService) buildMessage(eventCode string, data map[string]any) string {
	switch eventCode {
	case "user.logged_in":
		return fmt.Sprintf("User %s logged in successfully", stringOr(data, "username", "unknown"))
	case "password.reset_reminder":
		return fmt.Sprintf("Password reset reminder for user %s", stringOr(data, "username", "unknown"))
	case "health.issue_raised":
		return healthIssueMessage(data)
	default:
		return fmt.Sprintf("Notification for event: %s", eventCode)
	}
}

// stringOr returns data[key] when it is a string, otherwise fallback.
func stringOr(data map[string]any, key, fallback string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return fallback
}

func healthIssueMessage(data map[string]any) string {
	message := fmt.Sprintf("SickRock health %s: %s (%s)", stringOr(data, "severity", ""), stringOr(data, "title", ""), stringOr(data, "check", ""))
	if link := stringOr(data, "link", ""); link != "" {
		message += " — fix at " + link
	}
	return message
}

// sendToChannel sends a notification to a specific channel
func (ns *NotificationService) sendToChannel(ctx context.Context, channel repo.UserNotificationChannel, message string, data map[string]any) error {
	switch channel.ChannelType {
	case "telegram":
		return ns.sendTelegram(ctx, channel.ChannelValue, message)
	case "webhook":
		return ns.sendWebhook(ctx, channel.ChannelValue, message, data)
	case "email":
		return ns.sendEmail(ctx, channel.ChannelValue, message, data)
	default:
		return fmt.Errorf("unsupported channel type: %s", channel.ChannelType)
	}
}

// sendTelegram sends a notification via Telegram Bot API
func (ns *NotificationService) sendTelegram(ctx context.Context, telegramID string, message string) error {
	if ns.telegramBotToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ns.telegramBotToken)

	payload := map[string]any{
		"chat_id": telegramID,
		"text":    message,
	}

	resp, err := postJSON(ctx, url, payload)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	log.WithFields(log.Fields{
		"telegram_id": telegramID,
		"message":     message,
	}).Info("Sent Telegram notification")

	return nil
}

func postJSON(ctx context.Context, url string, payload map[string]any) (*http.Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func isHTTPSuccess(status int) bool {
	return status >= 200 && status < 300
}

// sendWebhook sends a notification via HTTP webhook
func (ns *NotificationService) sendWebhook(ctx context.Context, webhookURL string, message string, data map[string]any) error {
	payload := map[string]any{
		"message":   message,
		"timestamp": time.Now().Unix(),
		"data":      data,
	}

	resp, err := postJSON(ctx, webhookURL, payload)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !isHTTPSuccess(resp.StatusCode) {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.WithFields(log.Fields{
		"webhook_url": webhookURL,
		"message":     message,
	}).Info("Sent webhook notification")

	return nil
}

// sendEmail sends a notification via email (placeholder - to be implemented later)
func (ns *NotificationService) sendEmail(_ context.Context, emailAddress string, message string, _ map[string]any) error {
	// TODO: Implement email sending
	// For now, just log that email would be sent
	log.WithFields(log.Fields{
		"email":   emailAddress,
		"message": message,
	}).Info("Email notification (not yet implemented)")

	return fmt.Errorf("email notifications are not yet implemented")
}

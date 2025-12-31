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
	repo        *repo.Repository
	telegramBotToken string // Telegram bot API token (from environment)
}

// NewNotificationService creates a new notification service
func NewNotificationService(repo *repo.Repository) *NotificationService {
	return &NotificationService{
		repo: repo,
		telegramBotToken: getTelegramBotToken(),
	}
}

// getTelegramBotToken retrieves the Telegram bot token from environment
func getTelegramBotToken() string {
	// Read from environment variable SICKROCK_TELEGRAM_BOT_TOKEN
	return os.Getenv("SICKROCK_TELEGRAM_BOT_TOKEN")
}

// SendNotification sends a notification for a specific event
func (ns *NotificationService) SendNotification(ctx context.Context, eventCode string, data map[string]interface{}) error {
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

	// Send notifications to each subscription
	for _, sub := range subscriptions {
		err := ns.sendToChannel(ctx, sub.Channel, message, data)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"event_code": eventCode,
				"channel_id": sub.Channel.ID,
				"channel_type": sub.Channel.ChannelType,
			}).Error("Failed to send notification")
			// Continue with other channels even if one fails
		}
	}

	return nil
}

// buildMessage creates a human-readable message from event code and data
func (ns *NotificationService) buildMessage(eventCode string, data map[string]interface{}) string {
	switch eventCode {
	case "user.logged_in":
		username := "unknown"
		if u, ok := data["username"].(string); ok {
			username = u
		}
		return fmt.Sprintf("User %s logged in successfully", username)
	case "password.reset_reminder":
		username := "unknown"
		if u, ok := data["username"].(string); ok {
			username = u
		}
		return fmt.Sprintf("Password reset reminder for user %s", username)
	default:
		return fmt.Sprintf("Notification for event: %s", eventCode)
	}
}

// sendToChannel sends a notification to a specific channel
func (ns *NotificationService) sendToChannel(ctx context.Context, channel repo.UserNotificationChannel, message string, data map[string]interface{}) error {
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
	
	payload := map[string]interface{}{
		"chat_id": telegramID,
		"text":    message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	log.WithFields(log.Fields{
		"telegram_id": telegramID,
		"message":     message,
	}).Info("Sent Telegram notification")

	return nil
}

// sendWebhook sends a notification via HTTP webhook
func (ns *NotificationService) sendWebhook(ctx context.Context, webhookURL string, message string, data map[string]interface{}) error {
	payload := map[string]interface{}{
		"message": message,
		"timestamp": time.Now().Unix(),
		"data": data,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.WithFields(log.Fields{
		"webhook_url": webhookURL,
		"message":     message,
	}).Info("Sent webhook notification")

	return nil
}

// sendEmail sends a notification via email (placeholder - to be implemented later)
func (ns *NotificationService) sendEmail(ctx context.Context, emailAddress string, message string, data map[string]interface{}) error {
	// TODO: Implement email sending
	// For now, just log that email would be sent
	log.WithFields(log.Fields{
		"email":   emailAddress,
		"message": message,
	}).Info("Email notification (not yet implemented)")

	return fmt.Errorf("email notifications are not yet implemented")
}

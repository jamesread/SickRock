package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repo "github.com/jamesread/SickRock/internal/repo"
)

func TestBuildMessage(t *testing.T) {
	ns := &NotificationService{}

	assert.Equal(t, "User bob logged in successfully", ns.buildMessage("user.logged_in", map[string]any{"username": "bob"}))
	assert.Equal(t, "User unknown logged in successfully", ns.buildMessage("user.logged_in", map[string]any{}))
	assert.Equal(t, "Password reset reminder for user bob", ns.buildMessage("password.reset_reminder", map[string]any{"username": "bob"}))
	assert.Equal(t, "SickRock health warning: Broken link (nav) — fix at /admin",
		ns.buildMessage("health.issue_raised", map[string]any{"severity": "warning", "title": "Broken link", "check": "nav", "link": "/admin"}))
	assert.Equal(t, "SickRock health error: X (c)",
		ns.buildMessage("health.issue_raised", map[string]any{"severity": "error", "title": "X", "check": "c"}))
	assert.Equal(t, "Notification for event: other", ns.buildMessage("other", nil))
}

func TestSendTestDeliversToTheChannel(t *testing.T) {
	var got map[string]any
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer hook.Close()
	ns := &NotificationService{}

	err := ns.SendTest(context.Background(), repo.UserNotificationChannel{ChannelType: "webhook", ChannelValue: hook.URL})
	require.NoError(t, err)
	assert.Equal(t, TestNotificationMessage, got["message"])
	assert.Equal(t, map[string]any{"event": "test"}, got["data"])
}

func TestSendTestReportsFailures(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failing.Close()
	ns := &NotificationService{}

	err := ns.SendTest(context.Background(), repo.UserNotificationChannel{ChannelType: "webhook", ChannelValue: failing.URL})
	require.EqualError(t, err, "webhook returned status 502")

	err = ns.SendTest(context.Background(), repo.UserNotificationChannel{ChannelType: "telegram", ChannelValue: "42"})
	require.EqualError(t, err, "telegram bot token not configured")
}

func TestWithoutURLHidesTheRequestURL(t *testing.T) {
	err := &url.Error{Op: "Post", URL: "https://api.telegram.org/botSECRET-TOKEN/sendMessage", Err: errors.New("dial tcp: i/o timeout")}
	wrapped := fmt.Errorf("failed to send telegram message: %w", withoutURL(err))
	assert.NotContains(t, wrapped.Error(), "SECRET-TOKEN")
	assert.Equal(t, "failed to send telegram message: dial tcp: i/o timeout", wrapped.Error())
	plain := errors.New("other")
	assert.Same(t, plain, withoutURL(plain))
}

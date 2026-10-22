package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

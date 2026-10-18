package health

import (
	"context"
	"strings"
)

// DefaultAdminPasswordCheck reports when the built-in admin account still has its default password.
type DefaultAdminPasswordCheck struct {
	UsesDefaultPassword func(ctx context.Context) (bool, error)
}

func (c DefaultAdminPasswordCheck) ID() string    { return "security.default_admin_password" }
func (c DefaultAdminPasswordCheck) Title() string { return "Default admin password" }

func (c DefaultAdminPasswordCheck) Run(ctx context.Context) ([]Issue, error) {
	usesDefault, err := c.UsesDefaultPassword(ctx)
	if err != nil || !usesDefault {
		return nil, err
	}
	return []Issue{{
		Key:      "admin",
		Severity: SeverityError,
		Title:    `The "admin" user still has the default password`,
		Detail:   "Anyone who can reach SickRock can sign in as an administrator. Change the admin password, or disable the account.",
		Link:     "/admin/iam/users",
	}}, nil
}

// RiskySettingsCheck reports configuration that should not be left on in normal running.
type RiskySettingsCheck struct {
	Getenv   func(string) string
	LogLevel string
}

func (c RiskySettingsCheck) ID() string    { return "config.risky_settings" }
func (c RiskySettingsCheck) Title() string { return "Risky configuration settings" }

func (c RiskySettingsCheck) Run(ctx context.Context) ([]Issue, error) {
	var issues []Issue
	if c.Getenv("SICKROCK_RESET_ADMIN_PASSWORD") != "" {
		issues = append(issues, Issue{
			Key:      "env:SICKROCK_RESET_ADMIN_PASSWORD",
			Severity: SeverityWarning,
			Title:    "SICKROCK_RESET_ADMIN_PASSWORD is set",
			Detail:   `The admin password is reset to "admin" every time SickRock starts. Unset the environment variable once you have regained access.`,
		})
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "trace":
		issues = append(issues, Issue{
			Key:      "log-level",
			Severity: SeverityInfo,
			Title:    "Log level is " + strings.ToLower(c.LogLevel),
			Detail:   "Verbose logging can include request details and grows log files quickly. Use info or higher in normal running (SICKROCK_LOG_LEVEL / log_level).",
		})
	}
	return issues, nil
}

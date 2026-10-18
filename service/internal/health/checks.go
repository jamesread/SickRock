package health

import (
	"context"

	"github.com/jamesread/SickRock/internal/repo"
)

// Deps are what the default checks need.
type Deps struct {
	Repo *repo.Repository
	// ErrorLog is the logrus hook capturing recent errors (nil disables logs.recent_errors).
	ErrorLog *ErrorLog
	// AdminUsesDefaultPassword reports whether the "admin" user still has the default password
	// (nil disables security.default_admin_password).
	AdminUsesDefaultPassword func(ctx context.Context) (bool, error)
	// Getenv reads environment variables (os.Getenv in production).
	Getenv func(string) string
	// LogLevel is the configured log level.
	LogLevel string
}

// DefaultChecks returns every check, in the order they run and are listed. Add new checks here.
func DefaultChecks(deps Deps) []Check {
	checks := []Check{
		TableConfigurationDuplicateNamesCheck{Repo: deps.Repo},
		TableConfigurationMissingTableCheck{Repo: deps.Repo},
		NavigationBrokenLinksCheck{Repo: deps.Repo},
		BookmarksOrphanedCheck{Repo: deps.Repo},
		RelationsDanglingCheck{Repo: deps.Repo},
		CalendarFeedsFailingCheck{Repo: deps.Repo},
	}
	if deps.AdminUsesDefaultPassword != nil {
		checks = append(checks, DefaultAdminPasswordCheck{UsesDefaultPassword: deps.AdminUsesDefaultPassword})
	}
	if deps.Getenv != nil {
		checks = append(checks, RiskySettingsCheck{Getenv: deps.Getenv, LogLevel: deps.LogLevel})
	}
	if deps.ErrorLog != nil {
		checks = append(checks, RecentErrorsCheck{Log: deps.ErrorLog})
	}
	return checks
}

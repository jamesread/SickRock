// Package health holds SickRock's health checks. The engine that runs them, stores their
// issues and sends notifications is github.com/jamesread/armature-health.
//
// To add a check: implement armhealth.Check in a new file in this package and add it to
// DefaultChecks below.
package health

import (
	"context"

	armhealth "github.com/jamesread/armature-health"
	"github.com/jamesread/armature-health/checks"
	"github.com/jamesread/armature-health/errorlog"

	"github.com/jamesread/SickRock/internal/repo"
)

// Deps are what the default checks need.
type Deps struct {
	Repo *repo.Repository
	// ErrorLog is the logrus hook capturing recent errors (nil disables logs.recent_errors).
	ErrorLog *errorlog.ErrorLog
	// AdminUsesDefaultPassword reports whether the "admin" user still has the default password
	// (nil disables security.default_admin_password).
	AdminUsesDefaultPassword func(ctx context.Context) (bool, error)
	// Getenv reads environment variables (os.Getenv in production).
	Getenv func(string) string
	// LogLevel is the configured log level.
	LogLevel string
}

// DefaultChecks returns every check, in the order they run and are listed. Add new checks here.
func DefaultChecks(deps Deps) []armhealth.Check {
	checkList := []armhealth.Check{
		TableConfigurationDuplicateNamesCheck{Repo: deps.Repo},
		TableConfigurationMissingTableCheck{Repo: deps.Repo},
		NavigationBrokenLinksCheck{Repo: deps.Repo},
		BookmarksOrphanedCheck{Repo: deps.Repo},
		RelationsDanglingCheck{Repo: deps.Repo},
		CalendarFeedsFailingCheck{Repo: deps.Repo},
	}
	if deps.AdminUsesDefaultPassword != nil {
		checkList = append(checkList, checks.DefaultPassword{
			Product:             "SickRock",
			Link:                "/admin/iam/users",
			UsesDefaultPassword: deps.AdminUsesDefaultPassword,
		})
	}
	if deps.Getenv != nil {
		checkList = append(checkList, checks.RiskySettings{
			Getenv:          deps.Getenv,
			LogLevel:        deps.LogLevel,
			LogLevelSetting: "SICKROCK_LOG_LEVEL / log_level",
			EnvVars: []checks.RiskyEnvVar{{
				Name:   "SICKROCK_RESET_ADMIN_PASSWORD",
				Detail: `The admin password is reset to "admin" every time SickRock starts. Unset the environment variable once you have regained access.`,
			}},
		})
	}
	if deps.ErrorLog != nil {
		checkList = append(checkList, errorlog.RecentErrorsCheck{Log: deps.ErrorLog})
	}
	return checkList
}

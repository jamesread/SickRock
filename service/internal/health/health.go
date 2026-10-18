// Package health runs registered checks in the background and stores the issues they find,
// so admins can see and fix problems (broken configuration, risky settings, recent errors)
// before users run into them.
//
// To add a check: implement Check in a new file in this package and add it to DefaultChecks
// in checks.go.
package health

import "context"

// Severity of an issue. Only warnings and errors are notified and counted in the badge.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Issue is one problem found by a check.
type Issue struct {
	// Key identifies the issue within its check across runs (e.g. "nav:42"). An issue with
	// the same key on the next run is the same issue; a missing key means it was resolved.
	Key      string
	Severity Severity
	// Title is a one-line summary.
	Title string
	// Detail explains what is wrong and how to fix it.
	Detail string
	// Link is a frontend path where the problem can be fixed (optional).
	Link string
	// Count is how many times the problem occurred, when that is meaningful (e.g. logged
	// errors). Zero means 1.
	Count int
}

// Check finds issues of one kind.
type Check interface {
	// ID is stable and unique, e.g. "navigation.broken_links".
	ID() string
	// Title is shown in the UI.
	Title() string
	// Run returns every issue currently present. An error means the check could not run;
	// its previously found issues are then left as they are.
	Run(ctx context.Context) ([]Issue, error)
}

// Notifier sends a notification event (implemented by notifications.NotificationService).
type Notifier interface {
	SendNotification(ctx context.Context, eventCode string, data map[string]interface{}) error
}

// NotificationEventIssueRaised is sent once when a warning or error issue first appears.
const NotificationEventIssueRaised = "health.issue_raised"

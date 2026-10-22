package health

import (
	"context"
	"fmt"

	"github.com/jamesread/SickRock/internal/repo"
	armhealth "github.com/jamesread/armature-health"
)

// CalendarFeedsFailingCheck finds enabled calendar feeds whose last refresh failed.
type CalendarFeedsFailingCheck struct {
	Repo *repo.Repository
}

func (c CalendarFeedsFailingCheck) ID() string    { return "calendar_feeds.failing" }
func (c CalendarFeedsFailingCheck) Title() string { return "Failing calendar feeds" }

func (c CalendarFeedsFailingCheck) Run(ctx context.Context) ([]armhealth.Issue, error) {
	feeds, err := c.Repo.ListRssCalendarFeeds(ctx)
	if err != nil {
		return nil, err
	}
	return failingCalendarFeedIssues(feeds), nil
}

func failingCalendarFeedIssues(feeds []repo.RssCalendarFeed) []armhealth.Issue {
	var issues []armhealth.Issue
	for _, feed := range feeds {
		if issue, ok := failingCalendarFeedIssue(feed); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func failingCalendarFeedIssue(feed repo.RssCalendarFeed) (armhealth.Issue, bool) {
	if !feed.Enabled || feed.LastRefreshStatus != "error" {
		return armhealth.Issue{}, false
	}
	when := "on its last refresh"
	if feed.LastRefreshAt.Valid {
		when = "at " + feed.LastRefreshAt.Time.UTC().Format("2006-01-02 15:04 MST")
	}
	return armhealth.Issue{
		Key:      fmt.Sprintf("feed:%d", feed.ID),
		Severity: armhealth.SeverityWarning,
		Title:    fmt.Sprintf("Calendar feed %q is failing", feed.Name),
		Detail: fmt.Sprintf("Importing into %q failed %s: %s",
			feed.TableConfiguration, when, feed.LastRefreshMessage),
		Link: fmt.Sprintf("/admin/rss-calendar-feeds/%d", feed.ID),
	}, true
}

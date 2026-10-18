package health

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

type fakeCheck struct {
	issues []Issue
	err    error
	panics bool
}

func (c *fakeCheck) ID() string    { return "test.fake" }
func (c *fakeCheck) Title() string { return "Fake" }
func (c *fakeCheck) Run(context.Context) ([]Issue, error) {
	if c.panics {
		panic("boom")
	}
	return c.issues, c.err
}

type recordingNotifier struct {
	mu     sync.Mutex
	titles []string
}

func (n *recordingNotifier) SendNotification(_ context.Context, eventCode string, data map[string]interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if eventCode == NotificationEventIssueRaised {
		n.titles = append(n.titles, data["title"].(string))
	}
	return nil
}

func newTestService(t *testing.T, check Check) (*Service, *repo.Repository, *recordingNotifier, *time.Time) {
	t.Helper()
	r := repo.NewRepository(testdb.NewMigratedSQLite(t))
	notifier := &recordingNotifier{}
	svc := NewService(r, []Check{check}, notifier)
	now := time.Unix(1_800_000_000, 0)
	svc.now = func() time.Time { return now }
	return svc, r, notifier, &now
}

func openIssues(t *testing.T, r *repo.Repository) []repo.HealthIssue {
	t.Helper()
	issues, err := r.ListHealthIssues(context.Background(), false, true)
	if err != nil {
		t.Fatal(err)
	}
	return issues
}

func TestReconcileLifecycle(t *testing.T) {
	check := &fakeCheck{issues: []Issue{
		{Key: "a", Severity: SeverityError, Title: "A broke"},
		{Key: "b", Severity: SeverityInfo, Title: "B is odd"},
	}}
	svc, r, notifier, now := newTestService(t, check)
	ctx := context.Background()

	svc.RunAll(ctx)
	svc.notifications.Wait()
	if got := len(openIssues(t, r)); got != 2 {
		t.Fatalf("open issues after first run: %d", got)
	}
	if len(notifier.titles) != 1 || notifier.titles[0] != "A broke" {
		t.Fatalf("expected one notification for the error only, got %v", notifier.titles)
	}

	// Seen again: updated, not re-notified.
	*now = now.Add(time.Hour)
	check.issues[0].Title = "A still broke"
	svc.RunAll(ctx)
	svc.notifications.Wait()
	issues := openIssues(t, r)
	if len(issues) != 2 || len(notifier.titles) != 1 {
		t.Fatalf("second run: %d issues, notifications %v", len(issues), notifier.titles)
	}
	for _, issue := range issues {
		if issue.IssueKey == "a" && (issue.Title != "A still broke" || issue.LastSeen != now.Unix() || issue.FirstSeen == issue.LastSeen) {
			t.Fatalf("issue a not refreshed: %+v", issue)
		}
	}

	// Gone: resolved.
	check.issues = check.issues[1:]
	svc.RunAll(ctx)
	if got := openIssues(t, r); len(got) != 1 || got[0].IssueKey != "b" {
		t.Fatalf("expected only b open, got %+v", got)
	}

	// Recurs: a new row and a new notification.
	check.issues = append(check.issues, Issue{Key: "a", Severity: SeverityError, Title: "A broke again"})
	svc.RunAll(ctx)
	svc.notifications.Wait()
	all, _ := r.ListHealthIssues(ctx, true, true)
	if len(all) != 3 || len(notifier.titles) != 2 {
		t.Fatalf("recurrence: %d rows, notifications %v", len(all), notifier.titles)
	}
}

func TestAcknowledgedIssuesLeaveTheSummary(t *testing.T) {
	check := &fakeCheck{issues: []Issue{
		{Key: "a", Severity: SeverityError, Title: "A"},
		{Key: "b", Severity: SeverityWarning, Title: "B"},
	}}
	svc, r, _, _ := newTestService(t, check)
	ctx := context.Background()
	svc.RunAll(ctx)

	summary, _ := r.GetHealthSummary(ctx)
	if summary.Errors != 1 || summary.Warnings != 1 {
		t.Fatalf("summary %+v", summary)
	}
	issues := openIssues(t, r)
	var errorID int64
	for _, issue := range issues {
		if issue.IssueKey == "a" {
			errorID = issue.ID
		}
	}
	if err := r.SetHealthIssueAcknowledged(ctx, errorID, true, 1, 1); err != nil {
		t.Fatal(err)
	}
	summary, _ = r.GetHealthSummary(ctx)
	if summary.Errors != 0 || summary.Warnings != 1 {
		t.Fatalf("summary after acknowledging %+v", summary)
	}
	if unacked, _ := r.ListHealthIssues(ctx, false, false); len(unacked) != 1 {
		t.Fatalf("expected the acknowledged issue to be hidden, got %+v", unacked)
	}

	// Acknowledged issues still resolve normally.
	check.issues = nil
	svc.RunAll(ctx)
	if got := openIssues(t, r); len(got) != 0 {
		t.Fatalf("expected everything resolved, got %+v", got)
	}
}

func TestFailingChecksKeepTheirIssues(t *testing.T) {
	check := &fakeCheck{issues: []Issue{{Key: "a", Severity: SeverityWarning, Title: "A"}}}
	svc, r, _, _ := newTestService(t, check)
	ctx := context.Background()
	svc.RunAll(ctx)

	for name, broken := range map[string]*fakeCheck{
		"error": {err: errors.New("database unavailable")},
		"panic": {panics: true},
	} {
		check.err, check.panics, check.issues = broken.err, broken.panics, nil
		svc.RunAll(ctx)
		if got := openIssues(t, r); len(got) != 1 {
			t.Fatalf("%s: issues should be left alone, got %+v", name, got)
		}
		runs, _ := r.ListHealthCheckRuns(ctx)
		if len(runs) != 1 || runs[0].Status != "failed" || runs[0].Error == "" {
			t.Fatalf("%s: run not recorded as failed: %+v", name, runs)
		}
	}
}

func TestRunUnknownCheck(t *testing.T) {
	svc, _, _, _ := newTestService(t, &fakeCheck{})
	if err := svc.Run(context.Background(), "nope"); err == nil {
		t.Fatal("expected an error for an unknown check")
	}
}

func TestErrorLogGroupsMessagesByFingerprint(t *testing.T) {
	l := NewErrorLog()
	now := time.Unix(1_800_000_000, 0)
	l.now = func() time.Time { return now }

	l.record("Failed to create item: Error 1364 (HY000): Field 'starts' doesn't have a default value")
	l.record("Failed to create item: Error 1364 (HY000): Field 'finishes' doesn't have a default value")
	l.record(`Failed to load feed "abc": status 503 for 3f2c1a9e-1b2c-4d5e-8f90-123456789abc`)
	l.record(`Failed to load feed "xyz": status 502 for 00000000-1b2c-4d5e-8f90-123456789abc`)
	l.record("Something else entirely")

	issues, _ := RecentErrorsCheck{Log: l}.Run(context.Background())
	if len(issues) != 3 {
		t.Fatalf("expected 3 groups, got %d: %+v", len(issues), issues)
	}
	counts := map[int]int{}
	for _, issue := range issues {
		counts[issue.Count]++
	}
	if counts[2] != 2 || counts[1] != 1 {
		t.Fatalf("unexpected grouping %+v", issues)
	}

	now = now.Add(25 * time.Hour)
	if issues, _ := (RecentErrorsCheck{Log: l}).Run(context.Background()); len(issues) != 0 {
		t.Fatalf("errors older than a day should drop out, got %+v", issues)
	}
}

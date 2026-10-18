package health

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jamesread/SickRock/internal/repo"
)

const (
	defaultCheckTimeout = 30 * time.Second
	resolvedRetention   = 30 * 24 * time.Hour
	notifyTimeout       = 30 * time.Second
)

// Service runs checks and reconciles their issues with the stored ones.
type Service struct {
	store        *repo.Repository
	checks       []Check
	notifier     Notifier
	now          func() time.Time
	checkTimeout time.Duration

	// runMu serialises runs so a manual "run now" and the background job never reconcile
	// the same check at once.
	runMu sync.Mutex
	// notifications tracks in-flight notification sends (waited on by tests).
	notifications sync.WaitGroup
}

// NewService creates a service for the given checks. notifier may be nil.
func NewService(store *repo.Repository, checks []Check, notifier Notifier) *Service {
	return &Service{
		store:        store,
		checks:       checks,
		notifier:     notifier,
		now:          time.Now,
		checkTimeout: defaultCheckTimeout,
	}
}

// Checks returns the registered checks in order.
func (s *Service) Checks() []Check {
	return s.checks
}

// CheckTitle returns a check's title, or its ID when it is not registered (e.g. removed).
func (s *Service) CheckTitle(checkID string) string {
	for _, check := range s.checks {
		if check.ID() == checkID {
			return check.Title()
		}
	}
	return checkID
}

// RunAll runs every check, then prunes long-resolved issues.
func (s *Service) RunAll(ctx context.Context) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	for _, check := range s.checks {
		s.runCheck(ctx, check)
	}
	if err := s.store.PruneResolvedHealthIssues(ctx, s.now().Add(-resolvedRetention).Unix()); err != nil {
		log.WithError(err).Warn("Health: failed to prune resolved issues")
	}
}

// Run runs one check by ID.
func (s *Service) Run(ctx context.Context, checkID string) error {
	for _, check := range s.checks {
		if check.ID() == checkID {
			s.runMu.Lock()
			defer s.runMu.Unlock()
			s.runCheck(ctx, check)
			return nil
		}
	}
	return fmt.Errorf("unknown health check %q", checkID)
}

// RunPeriodically runs all checks after initialDelay and then every interval. It blocks.
func (s *Service) RunPeriodically(initialDelay, interval time.Duration) {
	log.WithField("interval", interval).Info("Health check job started")
	time.Sleep(initialDelay)
	for {
		s.RunAll(context.Background())
		time.Sleep(interval)
	}
}

func (s *Service) runCheck(ctx context.Context, check Check) {
	started := s.now()
	issues, err := s.safeRun(ctx, check)
	run := repo.HealthCheckRun{
		CheckID:    check.ID(),
		LastRunAt:  started.Unix(),
		DurationMs: s.now().Sub(started).Milliseconds(),
		Status:     "ok",
	}

	if err == nil {
		err = s.reconcile(ctx, check, issues)
	}
	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
		log.WithError(err).WithField("check", check.ID()).Warn("Health check failed")
	}
	if saveErr := s.store.SaveHealthCheckRun(ctx, run); saveErr != nil {
		log.WithError(saveErr).WithField("check", check.ID()).Warn("Health: failed to save check run")
	}
}

// safeRun runs a check with a timeout and turns panics into errors.
func (s *Service) safeRun(ctx context.Context, check Check) (issues []Issue, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.checkTimeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("check panicked: %v", r)
			log.WithField("check", check.ID()).Errorf("Health check panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return check.Run(ctx)
}

// reconcile stores new issues, refreshes ones seen again, and resolves ones no longer found.
func (s *Service) reconcile(ctx context.Context, check Check, issues []Issue) error {
	existing, err := s.store.ListOpenHealthIssuesForCheck(ctx, check.ID())
	if err != nil {
		return fmt.Errorf("load open issues: %w", err)
	}
	openByKey := make(map[string]repo.HealthIssue, len(existing))
	for _, issue := range existing {
		openByKey[issue.IssueKey] = issue
	}

	now := s.now().Unix()
	seen := map[string]bool{}
	for _, issue := range issues {
		if issue.Key == "" || seen[issue.Key] {
			continue
		}
		seen[issue.Key] = true

		stored := repo.HealthIssue{
			CheckID:     check.ID(),
			IssueKey:    issue.Key,
			Severity:    string(normalizeSeverity(issue.Severity)),
			Title:       issue.Title,
			Detail:      issue.Detail,
			Link:        issue.Link,
			LastSeen:    now,
			Occurrences: max(issue.Count, 1),
		}

		if open, ok := openByKey[issue.Key]; ok {
			stored.ID = open.ID
			if err := s.store.UpdateSeenHealthIssue(ctx, stored); err != nil {
				return fmt.Errorf("update issue %s: %w", issue.Key, err)
			}
			continue
		}

		stored.FirstSeen = now
		if _, err := s.store.InsertHealthIssue(ctx, stored); err != nil {
			return fmt.Errorf("insert issue %s: %w", issue.Key, err)
		}
		s.notify(check, stored)
	}

	for key, open := range openByKey {
		if !seen[key] {
			if err := s.store.ResolveHealthIssue(ctx, open.ID, now); err != nil {
				return fmt.Errorf("resolve issue %s: %w", key, err)
			}
		}
	}
	return nil
}

// notify sends the issue-raised event for new warnings and errors, without blocking the run.
func (s *Service) notify(check Check, issue repo.HealthIssue) {
	if s.notifier == nil || issue.Severity == string(SeverityInfo) {
		return
	}
	data := map[string]interface{}{
		"check":    check.Title(),
		"check_id": check.ID(),
		"severity": issue.Severity,
		"title":    issue.Title,
		"link":     issue.Link,
	}
	s.notifications.Add(1)
	go func() {
		defer s.notifications.Done()
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		if err := s.notifier.SendNotification(ctx, NotificationEventIssueRaised, data); err != nil {
			log.WithError(err).WithField("check", check.ID()).Warn("Health: failed to send notification")
		}
	}()
}

func normalizeSeverity(severity Severity) Severity {
	switch severity {
	case SeverityError, SeverityWarning:
		return severity
	default:
		return SeverityInfo
	}
}

package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/health"
	"github.com/jamesread/SickRock/internal/repo"
)

// SetHealthService attaches the health check service (see internal/health).
func (s *SickRockServer) SetHealthService(svc *health.Service) {
	s.health = svc
}

// NotificationService returns the service used to send notification events.
func (s *SickRockServer) NotificationService() health.Notifier {
	return s.notificationService
}

func (s *SickRockServer) requireHealth() error {
	if s.health == nil {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("health checks are not running"))
	}
	return nil
}

func (s *SickRockServer) ListHealthIssues(ctx context.Context, req *connect.Request[sickrockpb.ListHealthIssuesRequest]) (*connect.Response[sickrockpb.ListHealthIssuesResponse], error) {
	if err := s.requireHealth(); err != nil {
		return nil, err
	}
	issues, err := s.repo.ListHealthIssues(ctx, req.Msg.GetIncludeResolved(), req.Msg.GetIncludeAcknowledged())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	checks, err := s.healthCheckStatuses(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*sickrockpb.HealthIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, healthIssueToProto(issue, s.health.CheckTitle(issue.CheckID)))
	}
	return connect.NewResponse(&sickrockpb.ListHealthIssuesResponse{Issues: out, Checks: checks}), nil
}

func (s *SickRockServer) GetHealthSummary(ctx context.Context, req *connect.Request[sickrockpb.GetHealthSummaryRequest]) (*connect.Response[sickrockpb.GetHealthSummaryResponse], error) {
	summary, err := s.repo.GetHealthSummary(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.GetHealthSummaryResponse{
		Errors:   int32(summary.Errors),
		Warnings: int32(summary.Warnings),
		Infos:    int32(summary.Infos),
	}), nil
}

func (s *SickRockServer) RunHealthChecks(ctx context.Context, req *connect.Request[sickrockpb.RunHealthChecksRequest]) (*connect.Response[sickrockpb.RunHealthChecksResponse], error) {
	if err := s.requireHealth(); err != nil {
		return nil, err
	}
	if checkID := req.Msg.GetCheckId(); checkID != "" {
		if err := s.health.Run(ctx, checkID); err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	} else {
		s.health.RunAll(ctx)
	}
	checks, err := s.healthCheckStatuses(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.RunHealthChecksResponse{Checks: checks}), nil
}

func (s *SickRockServer) AcknowledgeHealthIssue(ctx context.Context, req *connect.Request[sickrockpb.AcknowledgeHealthIssueRequest]) (*connect.Response[sickrockpb.AcknowledgeHealthIssueResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetHealthIssueAcknowledged(ctx, req.Msg.GetId(), req.Msg.GetAcknowledged(), userID, time.Now().Unix()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&sickrockpb.AcknowledgeHealthIssueResponse{}), nil
}

// healthCheckStatuses lists every registered check with its last run.
func (s *SickRockServer) healthCheckStatuses(ctx context.Context) ([]*sickrockpb.HealthCheckStatus, error) {
	runs, err := s.repo.ListHealthCheckRuns(ctx)
	if err != nil {
		return nil, err
	}
	runByID := map[string]repo.HealthCheckRun{}
	for _, run := range runs {
		runByID[run.CheckID] = run
	}

	checks := s.health.Checks()
	out := make([]*sickrockpb.HealthCheckStatus, 0, len(checks))
	for _, check := range checks {
		status := &sickrockpb.HealthCheckStatus{CheckId: check.ID(), Title: check.Title(), Status: "pending"}
		if run, ok := runByID[check.ID()]; ok {
			status.LastRunUnix = run.LastRunAt
			status.DurationMs = run.DurationMs
			status.Status = run.Status
			status.Error = run.Error
		}
		out = append(out, status)
	}
	return out, nil
}

func healthIssueToProto(issue repo.HealthIssue, checkTitle string) *sickrockpb.HealthIssue {
	return &sickrockpb.HealthIssue{
		Id:                 issue.ID,
		CheckId:            issue.CheckID,
		CheckTitle:         checkTitle,
		IssueKey:           issue.IssueKey,
		Severity:           issue.Severity,
		Title:              issue.Title,
		Detail:             issue.Detail,
		Link:               issue.Link,
		FirstSeenUnix:      issue.FirstSeen,
		LastSeenUnix:       issue.LastSeen,
		Occurrences:        int32(issue.Occurrences),
		ResolvedAtUnix:     issue.ResolvedAt.Int64,
		AcknowledgedAtUnix: issue.AcknowledgedAt.Int64,
	}
}

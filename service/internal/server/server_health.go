package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	armhealth "github.com/jamesread/armature-health"
	"github.com/jamesread/armature-health/store"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

// SetHealthService attaches the health check service (see internal/health).
func (s *SickRockServer) SetHealthService(svc *armhealth.Service) {
	s.health = svc
}

// NotificationService returns the service used to send notification events.
func (s *SickRockServer) NotificationService() armhealth.Notifier {
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
	issues, err := s.health.ListIssues(ctx, req.Msg.GetIncludeResolved(), req.Msg.GetIncludeAcknowledged())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	checks, err := s.healthCheckStatuses(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&sickrockpb.ListHealthIssuesResponse{Issues: healthIssuesToProto(issues), Checks: checks}), nil
}

func (s *SickRockServer) GetHealthSummary(ctx context.Context, req *connect.Request[sickrockpb.GetHealthSummaryRequest]) (*connect.Response[sickrockpb.GetHealthSummaryResponse], error) {
	if err := s.requireHealth(); err != nil {
		return nil, err
	}
	summary, err := s.health.Summary(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.GetHealthSummaryResponse{
		Errors:   toInt32(summary.Errors),
		Warnings: toInt32(summary.Warnings),
		Infos:    toInt32(summary.Infos),
	}), nil
}

func (s *SickRockServer) RunHealthChecks(ctx context.Context, req *connect.Request[sickrockpb.RunHealthChecksRequest]) (*connect.Response[sickrockpb.RunHealthChecksResponse], error) {
	if err := s.requireHealth(); err != nil {
		return nil, err
	}
	if err := s.runHealthCheckRequest(ctx, req.Msg.GetCheckId()); err != nil {
		return nil, err
	}
	checks, err := s.healthCheckStatuses(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.RunHealthChecksResponse{Checks: checks}), nil
}

func (s *SickRockServer) AcknowledgeHealthIssue(ctx context.Context, req *connect.Request[sickrockpb.AcknowledgeHealthIssueRequest]) (*connect.Response[sickrockpb.AcknowledgeHealthIssueResponse], error) {
	if err := s.requireHealth(); err != nil {
		return nil, err
	}
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.health.Acknowledge(ctx, req.Msg.GetId(), req.Msg.GetAcknowledged(), int64(userID)); err != nil {
		return nil, healthAcknowledgeError(err)
	}
	return connect.NewResponse(&sickrockpb.AcknowledgeHealthIssueResponse{}), nil
}

func (s *SickRockServer) runHealthCheckRequest(ctx context.Context, checkID string) error {
	if checkID == "" {
		s.health.RunAll(ctx)
		return nil
	}
	if err := s.health.Run(ctx, checkID); err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return nil
}

func healthAcknowledgeError(err error) error {
	if errors.Is(err, store.ErrIssueNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

func healthIssuesToProto(issues []armhealth.IssueView) []*sickrockpb.HealthIssue {
	out := make([]*sickrockpb.HealthIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, healthIssueToProto(issue))
	}
	return out
}

// healthCheckStatuses lists every registered check with its last run.
func (s *SickRockServer) healthCheckStatuses(ctx context.Context) ([]*sickrockpb.HealthCheckStatus, error) {
	statuses, err := s.health.Statuses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*sickrockpb.HealthCheckStatus, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, &sickrockpb.HealthCheckStatus{
			CheckId:     status.CheckID,
			Title:       status.Title,
			Status:      status.Status,
			LastRunUnix: status.LastRunAt,
			DurationMs:  status.DurationMs,
			Error:       status.Error,
		})
	}
	return out, nil
}

func healthIssueToProto(issue armhealth.IssueView) *sickrockpb.HealthIssue {
	return &sickrockpb.HealthIssue{
		Id:                 issue.ID,
		CheckId:            issue.CheckID,
		CheckTitle:         issue.CheckTitle,
		IssueKey:           issue.IssueKey,
		Severity:           issue.Severity,
		Title:              issue.Title,
		Detail:             issue.Detail,
		Link:               issue.Link,
		FirstSeenUnix:      issue.FirstSeen,
		LastSeenUnix:       issue.LastSeen,
		Occurrences:        toInt32(issue.Occurrences),
		ResolvedAtUnix:     issue.ResolvedAt.Int64,
		AcknowledgedAtUnix: issue.AcknowledgedAt.Int64,
	}
}

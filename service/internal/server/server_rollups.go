package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// MaxRollupSourceIDs is the most source row ids a RunRollup request may list.
const MaxRollupSourceIDs = 20000

// ListRollups lists the rollups whose source table the caller may view.
func (s *SickRockServer) ListRollups(ctx context.Context, req *connect.Request[sickrockpb.ListRollupsRequest]) (*connect.Response[sickrockpb.ListRollupsResponse], error) {
	if !isAuthenticated(s.authUserRBAC(ctx)) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	rollups, err := s.repo.ListRollups(ctx, strings.TrimSpace(req.Msg.GetSourceTable()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list rollups: %w", err))
	}
	out := make([]*sickrockpb.Rollup, 0, len(rollups))
	for _, ru := range rollups {
		if s.requireDataTableAccess(ctx, ru.SourceTable, repo.ResourceActionView) != nil {
			continue
		}
		out = append(out, &sickrockpb.Rollup{
			Id:          ru.ID,
			Name:        ru.Name,
			SourceTable: ru.SourceTable,
			DetailTable: ru.DetailTable,
			TargetTable: ru.TargetTable,
		})
	}
	return connect.NewResponse(&sickrockpb.ListRollupsResponse{Rollups: out}), nil
}

// RunRollup replaces the rows of a rollup's target table with the grouped and summed detail rows
// of its matching source rows. It needs rollup.run, view access to the source and detail tables,
// and insert and delete access to the target, because every run empties it.
func (s *SickRockServer) RunRollup(ctx context.Context, req *connect.Request[sickrockpb.RunRollupRequest]) (*connect.Response[sickrockpb.RunRollupResponse], error) {
	ru, err := s.rollupToRun(ctx, req.Msg.GetRollupId())
	if err != nil {
		return nil, err
	}
	filter, err := rollupFilterFromRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	res, err := s.repo.RunRollup(ctx, ru, filter)
	if err != nil {
		return nil, rollupRunError(err)
	}
	log.WithFields(log.Fields{
		"rollup": ru.ID, "target": ru.TargetTable, "sourceRows": res.SourceRows, "rowsWritten": res.RowsWritten,
	}).Info("Ran rollup")
	return connect.NewResponse(&sickrockpb.RunRollupResponse{
		TargetTable: ru.TargetTable,
		RowsWritten: int64(res.RowsWritten),
		SourceRows:  int64(res.SourceRows),
		DetailRows:  int64(res.DetailRows),
		Skipped:     res.Skipped,
	}), nil
}

// rollupToRun loads a rollup the caller may run.
func (s *SickRockServer) rollupToRun(ctx context.Context, id int64) (*repo.Rollup, error) {
	if err := s.requireRollupRun(ctx); err != nil {
		return nil, err
	}
	ru, err := s.repo.GetRollup(ctx, id)
	if errors.Is(err, repo.ErrRollupNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load rollup: %w", err))
	}
	return ru, s.requireRollupTableAccess(ctx, ru)
}

func rollupRunError(err error) error {
	if errors.Is(err, repo.ErrInvalidRollup) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("run rollup: %w", err))
}

func (s *SickRockServer) requireRollupRun(ctx context.Context) error {
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) || (au.RBAC != nil && au.RBAC.Has(iam.PermissionRollupRun)) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", iam.PermissionRollupRun))
}

// requireRollupTableAccess checks the caller may read the rollup's source and detail tables and
// replace the rows of its target.
func (s *SickRockServer) requireRollupTableAccess(ctx context.Context, ru *repo.Rollup) error {
	checks := []struct {
		table  string
		action repo.ResourceGrantAction
	}{
		{ru.SourceTable, repo.ResourceActionView},
		{ru.DetailTable, repo.ResourceActionView},
		{ru.TargetTable, repo.ResourceActionInsert},
		{ru.TargetTable, repo.ResourceActionDelete},
	}
	for _, c := range checks {
		if err := s.requireDataTableAccess(ctx, c.table, c.action); err != nil {
			return err
		}
	}
	return nil
}

func rollupFilterFromRequest(msg *sickrockpb.RunRollupRequest) (repo.RollupSourceFilter, error) {
	filter := repo.RollupSourceFilter{
		DateColumn:       strings.TrimSpace(msg.GetDateColumn()),
		Where:            msg.GetWhere(),
		SourceIDs:        msg.GetSourceIds(),
		LimitToSourceIDs: msg.GetLimitToSourceIds(),
	}
	if len(filter.SourceIDs) > MaxRollupSourceIDs {
		return filter, fmt.Errorf("at most %d source ids", MaxRollupSourceIDs)
	}
	var err error
	filter.From, filter.To, err = parseRollupRange(filter.DateColumn, msg.GetFrom(), msg.GetTo())
	return filter, err
}

// parseRollupRange reads a run's date bounds, which need a date column.
func parseRollupRange(dateColumn, rawFrom, rawTo string) (from, to time.Time, err error) {
	if from, err = parseRollupBound(rawFrom); err != nil {
		return from, to, fmt.Errorf("from: %w", err)
	}
	if to, err = parseRollupBound(rawTo); err != nil {
		return from, to, fmt.Errorf("to: %w", err)
	}
	return from, to, requireRangeColumn(dateColumn, from, to)
}

// requireRangeColumn refuses date bounds without a date column to apply them to.
func requireRangeColumn(dateColumn string, from, to time.Time) error {
	if dateColumn == "" && (!from.IsZero() || !to.IsZero()) {
		return errors.New("a date range needs a date column")
	}
	return nil
}

// parseRollupBound reads a wall-clock date or date and time; empty is the zero time (an open end).
func parseRollupBound(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a date (YYYY-MM-DD)", raw)
}

package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/repo"
)

const (
	exportsManagePermission     = "exports.manage"
	tableReadOnlyExportsTCName  = "table_read_only_exports"
)

func (s *SickRockServer) requireExportsManageForTable(ctx context.Context, tcName string) error {
	if tcName != tableReadOnlyExportsTCName {
		return nil
	}
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && (au.RBAC.IsSuperuser || au.RBAC.Has(exportsManagePermission)) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("exports.manage permission required"))
}

func (s *SickRockServer) userCanViewReadOnlyCalendarExport(ctx context.Context, exp *repo.ReadOnlyCalendarExport) (bool, error) {
	if exp == nil {
		return false, nil
	}
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return false, nil
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return true, nil
	}
	if au.RBAC != nil && au.RBAC.Has(exportsManagePermission) {
		return true, nil
	}
	if len(exp.AllowedGroupIDs) == 0 {
		return false, nil
	}
	groupIDs, err := s.auth.Store.ListUserGroupIDsForUser(ctx, au.User.ID)
	if err != nil {
		return false, err
	}
	allowed := map[int]bool{}
	for _, id := range exp.AllowedGroupIDs {
		allowed[id] = true
	}
	for _, gid := range groupIDs {
		if allowed[gid] {
			return true, nil
		}
	}
	return false, nil
}

func (s *SickRockServer) authorizeReadOnlyCalendarExport(ctx context.Context, exp *repo.ReadOnlyCalendarExport) error {
	if exp == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("export not found"))
	}
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	ok, err := s.userCanViewReadOnlyCalendarExport(ctx, exp)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("check export access: %w", err))
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not allowed to view this export"))
	}
	return nil
}

func (s *SickRockServer) ListAccessibleReadOnlyExports(ctx context.Context, _ *connect.Request[sickrockpb.ListAccessibleReadOnlyExportsRequest]) (*connect.Response[sickrockpb.ListAccessibleReadOnlyExportsResponse], error) {
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	all, err := s.repo.ListEnabledReadOnlyCalendarExports(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list exports: %w", err))
	}
	out := make([]*sickrockpb.ReadOnlyExportSummary, 0, len(all))
	for i := range all {
		exp := &all[i]
		ok, err := s.userCanViewReadOnlyCalendarExport(ctx, exp)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("check export access: %w", err))
		}
		if !ok {
			continue
		}
		title := strings.TrimSpace(exp.Title)
		if title == "" {
			title = exp.Slug
		}
		out = append(out, &sickrockpb.ReadOnlyExportSummary{
			Slug:               exp.Slug,
			Title:              title,
			TableConfiguration: exp.TableConfiguration,
			DisplayMode:        exp.DisplayMode,
		})
	}
	return connect.NewResponse(&sickrockpb.ListAccessibleReadOnlyExportsResponse{Exports: out}), nil
}

func (s *SickRockServer) GetReadOnlyCalendarExport(ctx context.Context, req *connect.Request[sickrockpb.GetReadOnlyCalendarExportRequest]) (*connect.Response[sickrockpb.GetReadOnlyCalendarExportResponse], error) {
	slug := strings.TrimSpace(req.Msg.GetSlug())
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	exp, err := s.repo.GetReadOnlyCalendarExportBySlug(ctx, slug)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, err
	}
	if !exp.Enabled {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("export not found"))
	}

	if err := s.authorizeReadOnlyCalendarExport(ctx, exp); err != nil {
		return nil, err
	}

	actorID, actorName := s.auditActorFromContext(ctx)
	s.recordAudit(ctx, repo.AuditLogEntry{
		EventType:     audit.EventExportView,
		Message:       "read-only calendar export",
		ActorUserID:   actorID,
		ActorUsername: actorName,
		RelatedTC:     exp.TableConfiguration,
		Details:       map[string]string{"slug": slug, "display_mode": exp.DisplayMode},
		Success:       true,
	})

	items, err := s.repo.ListItemsForReadOnlyExport(ctx, exp)
	if err != nil {
		return nil, err
	}

	out := make([]*sickrockpb.Item, 0, len(items))
	for _, it := range items {
		out = append(out, repoItemToProto(it, exp.DisplayMode == "free_busy"))
	}

	var tableViewID int32
	if exp.TableViewID.Valid {
		tableViewID = int32(exp.TableViewID.Int64)
	}

	return connect.NewResponse(&sickrockpb.GetReadOnlyCalendarExportResponse{
		Title:              exp.Title,
		DisplayMode:        exp.DisplayMode,
		WeekendsOnly:       exp.WeekendsOnly,
		TableConfiguration: exp.TableConfiguration,
		TableViewId:        tableViewID,
		Items:              out,
	}), nil
}

func repoItemToProto(it repo.Item, freeBusy bool) *sickrockpb.Item {
	additionalFields := make(map[string]string)
	for key, value := range it.Fields {
		if value == nil {
			continue
		}
		if freeBusy {
			continue
		}
		if timeVal, ok := value.(time.Time); ok {
			additionalFields[key] = timeVal.Format("2006-01-02 15:04:05")
		} else {
			additionalFields[key] = fmt.Sprintf("%v", value)
		}
	}

	var srCreatedRelative, srUpdatedRelative int32
	if !it.SrCreated.IsZero() {
		srCreatedRelative = safeInt64ToInt32(int64(time.Since(it.SrCreated).Seconds()))
	}
	if !it.SrUpdated.IsZero() {
		srUpdatedRelative = safeInt64ToInt32(int64(time.Since(it.SrUpdated).Seconds()))
	}

	return &sickrockpb.Item{
		Id:                it.ID,
		SrCreated:         it.SrCreated.Unix(),
		SrCreatedRelative: srCreatedRelative,
		SrUpdated:         it.SrUpdated.Unix(),
		SrUpdatedRelative: srUpdatedRelative,
		AdditionalFields:  additionalFields,
	}
}

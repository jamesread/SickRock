package server

import (
	"context"
	"fmt"
	"slices"
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
	tableRssCalendarFeedsTCName = "table_rss_calendar_feeds"
)

func isExportsManagedTable(tcName string) bool {
	return tcName == tableReadOnlyExportsTCName || tcName == tableRssCalendarFeedsTCName
}

func (s *SickRockServer) requireExportsManageForTable(ctx context.Context, tcName string) error {
	if !isExportsManagedTable(tcName) {
		return nil
	}
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	// Has is nil-safe and true for superusers.
	if au.RBAC.Has(exportsManagePermission) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("exports.manage permission required"))
}

func (s *SickRockServer) userCanViewReadOnlyCalendarExport(ctx context.Context, exp *repo.ReadOnlyCalendarExport) (bool, error) {
	if exp == nil {
		return false, nil
	}
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return false, nil
	}
	// Has is nil-safe and true for superusers.
	if au.RBAC.Has(exportsManagePermission) {
		return true, nil
	}
	return s.userInExportGroups(ctx, au.User.ID, exp)
}

// userInExportGroups reports whether the user belongs to one of the export's allowed IAM groups.
func (s *SickRockServer) userInExportGroups(ctx context.Context, userID int, exp *repo.ReadOnlyCalendarExport) (bool, error) {
	if len(exp.AllowedGroupIDs) == 0 {
		return false, nil
	}
	groupIDs, err := s.auth.Store.ListUserGroupIDsForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(groupIDs, func(gid int) bool { return slices.Contains(exp.AllowedGroupIDs, gid) }), nil
}

func (s *SickRockServer) authorizeReadOnlyCalendarExport(ctx context.Context, exp *repo.ReadOnlyCalendarExport) error {
	if exp == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("export not found"))
	}
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
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
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	all, err := s.repo.ListEnabledReadOnlyCalendarExports(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list exports: %w", err))
	}
	out, err := s.viewableExportSummaries(ctx, all)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.ListAccessibleReadOnlyExportsResponse{Exports: out}), nil
}

// viewableExportSummaries returns summaries of the exports the caller may view.
func (s *SickRockServer) viewableExportSummaries(ctx context.Context, all []repo.ReadOnlyCalendarExport) ([]*sickrockpb.ReadOnlyExportSummary, error) {
	out := make([]*sickrockpb.ReadOnlyExportSummary, 0, len(all))
	for i := range all {
		exp := &all[i]
		ok, err := s.userCanViewReadOnlyCalendarExport(ctx, exp)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("check export access: %w", err))
		}
		if ok {
			out = append(out, readOnlyExportSummary(exp))
		}
	}
	return out, nil
}

func readOnlyExportSummary(exp *repo.ReadOnlyCalendarExport) *sickrockpb.ReadOnlyExportSummary {
	title := strings.TrimSpace(exp.Title)
	if title == "" {
		title = exp.Slug
	}
	return &sickrockpb.ReadOnlyExportSummary{
		Slug:               exp.Slug,
		Title:              title,
		TableConfiguration: exp.TableConfiguration,
		DisplayMode:        exp.DisplayMode,
	}
}

func (s *SickRockServer) GetReadOnlyCalendarExport(ctx context.Context, req *connect.Request[sickrockpb.GetReadOnlyCalendarExportRequest]) (*connect.Response[sickrockpb.GetReadOnlyCalendarExportResponse], error) {
	slug := strings.TrimSpace(req.Msg.GetSlug())
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	exp, err := s.enabledReadOnlyCalendarExport(ctx, slug)
	if err != nil {
		return nil, err
	}

	if err = s.authorizeReadOnlyCalendarExport(ctx, exp); err != nil {
		return nil, err
	}

	s.auditExportView(ctx, exp, slug)

	items, err := s.repo.ListItemsForReadOnlyExport(ctx, exp)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&sickrockpb.GetReadOnlyCalendarExportResponse{
		Title:              exp.Title,
		DisplayMode:        exp.DisplayMode,
		WeekendsOnly:       exp.WeekendsOnly,
		TableConfiguration: exp.TableConfiguration,
		TableViewId:        exportTableViewID(exp),
		Items:              exportItemsToProto(items, exp.DisplayMode == "free_busy"),
	}), nil
}

// exportItemsToProto converts export rows to protobuf items.
func exportItemsToProto(items []repo.Item, freeBusy bool) []*sickrockpb.Item {
	out := make([]*sickrockpb.Item, 0, len(items))
	for _, it := range items {
		out = append(out, repoItemToProto(it, freeBusy))
	}
	return out
}

// auditExportView records that the caller viewed an export.
func (s *SickRockServer) auditExportView(ctx context.Context, exp *repo.ReadOnlyCalendarExport, slug string) {
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
}

// exportTableViewID returns the export's calendar view id, or 0 when none is set.
func exportTableViewID(exp *repo.ReadOnlyCalendarExport) int32 {
	if !exp.TableViewID.Valid {
		return 0
	}
	return toInt32(exp.TableViewID.Int64)
}

// enabledReadOnlyCalendarExport loads an export by slug, failing with NotFound when it is missing or disabled.
func (s *SickRockServer) enabledReadOnlyCalendarExport(ctx context.Context, slug string) (*repo.ReadOnlyCalendarExport, error) {
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
	return exp, nil
}

func repoItemToProto(it repo.Item, freeBusy bool) *sickrockpb.Item {
	return &sickrockpb.Item{
		Id:                it.ID,
		SrCreated:         it.SrCreated.Unix(),
		SrCreatedRelative: relativeSeconds(it.SrCreated),
		SrUpdated:         it.SrUpdated.Unix(),
		SrUpdatedRelative: relativeSeconds(it.SrUpdated),
		AdditionalFields:  exportAdditionalFields(it.Fields, freeBusy),
	}
}

// exportAdditionalFields stringifies non-nil row fields; free/busy exports expose none.
func exportAdditionalFields(fields map[string]any, freeBusy bool) map[string]string {
	additionalFields := make(map[string]string)
	if freeBusy {
		return additionalFields
	}
	for key, value := range fields {
		if value == nil {
			continue
		}
		if timeVal, ok := value.(time.Time); ok {
			additionalFields[key] = timeVal.Format("2006-01-02 15:04:05")
		} else {
			additionalFields[key] = fmt.Sprintf("%v", value)
		}
	}
	return additionalFields
}

// relativeSeconds returns whole seconds since t, or 0 when t is unset.
func relativeSeconds(t time.Time) int32 {
	if t.IsZero() {
		return 0
	}
	return safeInt64ToInt32(int64(time.Since(t).Seconds()))
}

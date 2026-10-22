package server

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam/systemroles"
	"github.com/jamesread/SickRock/internal/repo"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/rbac"
)

func (s *SickRockServer) authUserRBAC(ctx context.Context) *armlayer.AuthenticatedUser {
	return s.authUser(ctx)
}

// isAuthenticated reports whether au is a signed-in user.
func isAuthenticated(au *armlayer.AuthenticatedUser) bool {
	return au != nil && au.User != nil
}

// isSuperuser reports whether au holds the RBAC superuser flag.
func isSuperuser(au *armlayer.AuthenticatedUser) bool {
	return au != nil && au.RBAC != nil && au.RBAC.IsSuperuser
}

// fullTableAccess grants every table action.
func fullTableAccess() *sickrockpb.GetTableAccessResponse {
	return &sickrockpb.GetTableAccessResponse{CanView: true, CanInsert: true, CanEdit: true, CanDelete: true}
}

func (s *SickRockServer) GetTableAccess(ctx context.Context, req *connect.Request[sickrockpb.GetTableAccessRequest]) (*connect.Response[sickrockpb.GetTableAccessResponse], error) {
	tableName := strings.TrimSpace(req.Msg.GetTableName())
	if tableName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table_name is required"))
	}
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if access, handled, err := s.systemTableAccess(ctx, tableName); handled {
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(access), nil
	}
	if isSuperuser(au) {
		return connect.NewResponse(fullTableAccess()), nil
	}
	access, err := s.repo.GetUserTableAccess(ctx, au.User.ID, tableName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("table access: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetTableAccessResponse{
		CanView:   access.CanView,
		CanInsert: access.CanInsert,
		CanEdit:   access.CanEdit,
		CanDelete: access.CanDelete,
	}), nil
}

// systemTableAccess returns access to the audit log, read-only exports and system settings tables,
// which are governed by system permissions rather than table grants. handled is false for other tables.
func (s *SickRockServer) systemTableAccess(ctx context.Context, tableName string) (access *sickrockpb.GetTableAccessResponse, handled bool, err error) {
	switch {
	case tableName == audit.TableConfiguration:
		return &sickrockpb.GetTableAccessResponse{CanView: true}, true, s.requireAuditView(ctx)
	case tableName == tableReadOnlyExportsTCName:
		return fullTableAccess(), true, s.requireExportsManageForTable(ctx, tableName)
	case isSystemSettingsTableConfiguration(tableName):
		return fullTableAccess(), true, s.requireSystemSettings(ctx)
	}
	return nil, false, nil
}

func (s *SickRockServer) requireDataTableAccess(ctx context.Context, tableConfiguration string, action repo.ResourceGrantAction) error {
	if err := s.requireAuditLogTableAccess(ctx, tableConfiguration); err != nil {
		return err
	}
	if handled, err := s.requireSpecialTableAccess(ctx, tableConfiguration); handled {
		return err
	}
	return s.requireTableAccess(ctx, tableConfiguration, action)
}

func (s *SickRockServer) requireSpecialTableAccess(ctx context.Context, tableConfiguration string) (bool, error) {
	if tableConfiguration == audit.TableConfiguration {
		return true, nil
	}
	if tableConfiguration == tableReadOnlyExportsTCName {
		return true, s.requireExportsManageForTable(ctx, tableConfiguration)
	}
	if isSystemSettingsTableConfiguration(tableConfiguration) {
		return true, s.requireSystemSettings(ctx)
	}
	return false, nil
}

func (s *SickRockServer) requireTableAccess(ctx context.Context, tableConfiguration string, action repo.ResourceGrantAction) error {
	if err := validateTableAccessTarget(tableConfiguration); err != nil {
		return err
	}
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) {
		s.auditTableAccess(ctx, tableConfiguration, action, "", true, "allowed (superuser)")
		return nil
	}
	return s.requireTableGrant(ctx, au.User.ID, tableConfiguration, action)
}

// validateTableAccessTarget rejects an empty table and the audit log, which is governed by audit.view.
func validateTableAccessTarget(tableConfiguration string) error {
	if tableConfiguration == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table is required"))
	}
	if tableConfiguration == audit.TableConfiguration {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("use audit.view for audit logs"))
	}
	return nil
}

// requireTableGrant checks the user holds a table grant for the action, auditing the outcome.
func (s *SickRockServer) requireTableGrant(ctx context.Context, userID int, tableConfiguration string, action repo.ResourceGrantAction) error {
	switch action {
	case repo.ResourceActionView, repo.ResourceActionInsert, repo.ResourceActionEdit, repo.ResourceActionDelete:
	default:
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("invalid table action"))
	}

	ok, err := s.repo.UserHasTableGrant(ctx, userID, tableConfiguration, action)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("check table grant: %w", err))
	}
	if !ok {
		msg := fmt.Sprintf("no grant for table %s", tableConfiguration)
		s.auditTableAccess(ctx, tableConfiguration, action, "", false, msg)
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s", msg))
	}
	s.auditTableAccess(ctx, tableConfiguration, action, "", true, "allowed")
	return nil
}

func (s *SickRockServer) requireDashboardAccess(ctx context.Context, dashboardName string) error {
	if dashboardName == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("dashboard is required"))
	}
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) {
		return nil
	}
	if !mayViewDashboards(au) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", systemroles.PermissionDashboardView))
	}
	ok, err := s.repo.UserHasDashboardGrant(ctx, au.User.ID, dashboardName, repo.ResourceActionView)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("no grant for dashboard %s", dashboardName))
	}
	return nil
}

// mayViewDashboards reports whether au's role permissions allow viewing dashboards (dashboard.view
// or app access); users without loaded RBAC fall through to the per-dashboard grant check.
func mayViewDashboards(au *armlayer.AuthenticatedUser) bool {
	if au.RBAC == nil {
		return true
	}
	return au.RBAC.Has(systemroles.PermissionDashboardView) || au.RBAC.Has(rbac.PermissionAppAccess)
}

func (s *SickRockServer) requireWorkflowAccess(ctx context.Context, workflowID int, action repo.ResourceGrantAction) error {
	if workflowID <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("workflow_id is required"))
	}
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) {
		return nil
	}

	switch action {
	case repo.ResourceActionView, repo.ResourceActionStart:
	default:
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("invalid workflow action"))
	}

	ok, err := s.repo.UserHasWorkflowGrant(ctx, au.User.ID, workflowID, action)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("no grant for workflow %d", workflowID))
	}
	return nil
}

func (s *SickRockServer) canViewNavigationItem(ctx context.Context, item repo.NavigationItem) bool {
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return false
	}
	if isSuperuser(au) {
		return true
	}
	return s.canViewNavigationTarget(ctx, item)
}

// canViewNavigationTarget checks access to what a navigation item links to: a workflow hub (no
// table/dashboard target), otherwise its dashboard, otherwise its table.
func (s *SickRockServer) canViewNavigationTarget(ctx context.Context, item repo.NavigationItem) bool {
	tableName := nullStringOr(item.TableName, "")
	dashboardName := nullStringOr(item.DashboardName, "")
	switch {
	case item.WorkflowID.Valid && item.WorkflowID.Int64 > 0 && tableName == "" && dashboardName == "":
		return s.requireWorkflowAccess(ctx, int(item.WorkflowID.Int64), repo.ResourceActionView) == nil
	case dashboardName != "":
		return s.requireDashboardAccess(ctx, dashboardName) == nil
	case tableName != "":
		return s.requireTableAccess(ctx, tableName, repo.ResourceActionView) == nil
	}
	return false
}

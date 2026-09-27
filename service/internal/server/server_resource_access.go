package server

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/rbac"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam/systemroles"
	"github.com/jamesread/SickRock/internal/repo"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

func (s *SickRockServer) authUserRBAC(ctx context.Context) *armlayer.AuthenticatedUser {
	return s.authUser(ctx)
}

func (s *SickRockServer) requireGlobalPermission(ctx context.Context, perm string) error {
	au := s.authUserRBAC(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return nil
	}
	if au.RBAC != nil && au.RBAC.Has(perm) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", perm))
}

func (s *SickRockServer) GetTableAccess(ctx context.Context, req *connect.Request[sickrockpb.GetTableAccessRequest]) (*connect.Response[sickrockpb.GetTableAccessResponse], error) {
	tableName := strings.TrimSpace(req.Msg.GetTableName())
	if tableName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table_name is required"))
	}
	au := s.authUserRBAC(ctx)
	if au == nil || au.User == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if tableName == audit.TableConfiguration {
		if s.requireAuditView(ctx) != nil {
			return connect.NewResponse(&sickrockpb.GetTableAccessResponse{}), nil
		}
		return connect.NewResponse(&sickrockpb.GetTableAccessResponse{CanView: true}), nil
	}
	if tableName == tableReadOnlyExportsTCName {
		if err := s.requireExportsManageForTable(ctx, tableName); err != nil {
			return connect.NewResponse(&sickrockpb.GetTableAccessResponse{}), nil
		}
		return connect.NewResponse(&sickrockpb.GetTableAccessResponse{
			CanView: true, CanInsert: true, CanEdit: true, CanDelete: true,
		}), nil
	}
	if isSystemSettingsTableConfiguration(tableName) {
		if s.requireSystemSettings(ctx) != nil {
			return connect.NewResponse(&sickrockpb.GetTableAccessResponse{}), nil
		}
		return connect.NewResponse(&sickrockpb.GetTableAccessResponse{
			CanView: true, CanInsert: true, CanEdit: true, CanDelete: true,
		}), nil
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return connect.NewResponse(&sickrockpb.GetTableAccessResponse{
			CanView: true, CanInsert: true, CanEdit: true, CanDelete: true,
		}), nil
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

func (s *SickRockServer) requireDataTableAccess(ctx context.Context, tableConfiguration string, action repo.ResourceGrantAction) error {
	if err := s.requireAuditLogTableAccess(ctx, tableConfiguration); err != nil {
		return err
	}
	if tableConfiguration == audit.TableConfiguration {
		return nil
	}
	if tableConfiguration == tableReadOnlyExportsTCName {
		return s.requireExportsManageForTable(ctx, tableConfiguration)
	}
	if isSystemSettingsTableConfiguration(tableConfiguration) {
		return s.requireSystemSettings(ctx)
	}
	return s.requireTableAccess(ctx, tableConfiguration, action)
}

func (s *SickRockServer) requireTableAccess(ctx context.Context, tableConfiguration string, action repo.ResourceGrantAction) error {
	if tableConfiguration == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table is required"))
	}
	if tableConfiguration == audit.TableConfiguration {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("use audit.view for audit logs"))
	}
	au := s.authUserRBAC(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		s.auditTableAccess(ctx, tableConfiguration, action, "", true, "allowed (superuser)")
		return nil
	}

	switch action {
	case repo.ResourceActionView, repo.ResourceActionInsert, repo.ResourceActionEdit, repo.ResourceActionDelete:
	default:
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("invalid table action"))
	}

	ok, err := s.repo.UserHasTableGrant(ctx, au.User.ID, tableConfiguration, action)
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
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return nil
	}
	if au.RBAC != nil && !au.RBAC.Has(systemroles.PermissionDashboardView) {
		if au.RBAC == nil || !au.RBAC.Has(rbac.PermissionAppAccess) {
			return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", systemroles.PermissionDashboardView))
		}
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

func (s *SickRockServer) requireWorkflowAccess(ctx context.Context, workflowID int, action repo.ResourceGrantAction) error {
	if workflowID <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("workflow_id is required"))
	}
	au := s.authUserRBAC(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
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
	if au == nil || au.User == nil {
		return false
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return true
	}

	// Workflow hub link (no table/dashboard target)
	if item.WorkflowID.Valid && item.WorkflowID.Int64 > 0 &&
		(!item.TableName.Valid || item.TableName.String == "") &&
		(!item.DashboardName.Valid || item.DashboardName.String == "") {
		wid := int(item.WorkflowID.Int64)
		return s.requireWorkflowAccess(ctx, wid, repo.ResourceActionView) == nil
	}

	if item.DashboardName.Valid && item.DashboardName.String != "" {
		return s.requireDashboardAccess(ctx, item.DashboardName.String) == nil
	}

	if item.TableName.Valid && item.TableName.String != "" {
		return s.requireTableAccess(ctx, item.TableName.String, repo.ResourceActionView) == nil
	}

	return false
}

func workflowIDFromNavItem(item repo.NavigationItem) (int, bool) {
	if !item.WorkflowID.Valid || item.WorkflowID.Int64 <= 0 {
		return 0, false
	}
	return int(item.WorkflowID.Int64), true
}

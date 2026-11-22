package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam"
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

// resourceSuperuser reports whether au's owner is a superuser for resource checks. A
// permission-restricted API key's RBAC is no longer superuser, but its owner still reaches every
// resource the key's scope allows.
func resourceSuperuser(ctx context.Context, au *armlayer.AuthenticatedUser) bool {
	if isSuperuser(au) {
		return true
	}
	scope := iam.APIKeyScopeFromContext(ctx)
	return scope != nil && scope.OwnerIsSuperuser
}

// requireAPIKeyResource checks that the API key behind the request, if any, may use a resource
// with a permission. Requests not made with an API key always pass.
func requireAPIKeyResource(ctx context.Context, resourceType, key, permission string) error {
	scope := iam.APIKeyScopeFromContext(ctx)
	if scope == nil {
		return nil
	}
	if !scope.AllowsPermission(permission) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("this API key does not have the %s permission", permission))
	}
	if !scope.AllowsResource(resourceType, key) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("this API key cannot use %s %s", resourceType, key))
	}
	return nil
}

// apiKeyTableAccess limits access to what the API key behind the request, if any, allows.
func apiKeyTableAccess(ctx context.Context, tableName string, access *sickrockpb.GetTableAccessResponse) *sickrockpb.GetTableAccessResponse {
	readOnly := apiKeyReadOnly(ctx)
	return &sickrockpb.GetTableAccessResponse{
		CanView:   tableActionAllowed(ctx, tableName, access.GetCanView(), repo.ResourceActionView, false),
		CanInsert: tableActionAllowed(ctx, tableName, access.GetCanInsert(), repo.ResourceActionInsert, readOnly),
		CanEdit:   tableActionAllowed(ctx, tableName, access.GetCanEdit(), repo.ResourceActionEdit, readOnly),
		CanDelete: tableActionAllowed(ctx, tableName, access.GetCanDelete(), repo.ResourceActionDelete, readOnly),
	}
}

func apiKeyReadOnly(ctx context.Context) bool {
	au := armlayer.UserFromContext(ctx)
	return au != nil && au.ReadOnly
}

func tableActionAllowed(ctx context.Context, tableName string, base bool, action repo.ResourceGrantAction, blockIfReadOnly bool) bool {
	if !base || blockIfReadOnly {
		return false
	}
	perm, _ := repo.TableActionPermission(action)
	return requireAPIKeyResource(ctx, iam.ResourceTypeTable, tableName, perm) == nil
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
	if access, err, handled := s.lookupSystemTableAccess(ctx, tableName); handled {
		return access, err
	}
	if resourceSuperuser(ctx, au) {
		return connect.NewResponse(apiKeyTableAccess(ctx, tableName, fullTableAccess())), nil
	}
	return s.userTableAccessResponse(ctx, au.User.ID, tableName)
}

func (s *SickRockServer) lookupSystemTableAccess(ctx context.Context, tableName string) (*connect.Response[sickrockpb.GetTableAccessResponse], error, bool) {
	access, handled, err := s.systemTableAccess(ctx, tableName)
	if !handled {
		return nil, nil, false
	}
	if err != nil {
		return nil, err, true
	}
	return connect.NewResponse(access), nil, true
}

func (s *SickRockServer) userTableAccessResponse(ctx context.Context, userID int, tableName string) (*connect.Response[sickrockpb.GetTableAccessResponse], error) {
	access, err := s.repo.GetUserTableAccess(ctx, userID, tableName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("table access: %w", err))
	}
	return connect.NewResponse(apiKeyTableAccess(ctx, tableName, &sickrockpb.GetTableAccessResponse{
		CanView:   access.CanView,
		CanInsert: access.CanInsert,
		CanEdit:   access.CanEdit,
		CanDelete: access.CanDelete,
	})), nil
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
	perm, err := repo.TableActionPermission(action)
	if err != nil {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("invalid table action"))
	}
	return s.authorizeTableAccess(ctx, au, tableConfiguration, action, perm)
}

// authorizeTableAccess applies the API key scope, then lets superusers through, otherwise requires a table grant.
func (s *SickRockServer) authorizeTableAccess(ctx context.Context, au *armlayer.AuthenticatedUser, tableConfiguration string, action repo.ResourceGrantAction, perm string) error {
	if err := requireAPIKeyResource(ctx, iam.ResourceTypeTable, tableConfiguration, perm); err != nil {
		s.auditTableAccess(ctx, tableConfiguration, action, false, err.Error())
		return err
	}
	if resourceSuperuser(ctx, au) {
		s.auditTableAccess(ctx, tableConfiguration, action, true, "allowed (superuser)")
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
		s.auditTableAccess(ctx, tableConfiguration, action, false, msg)
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s", msg))
	}
	s.auditTableAccess(ctx, tableConfiguration, action, true, "allowed")
	return nil
}

func (s *SickRockServer) requireDashboardAccess(ctx context.Context, dashboardName string) error {
	if dashboardName == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("dashboard is required"))
	}
	au, err := s.requireAuthenticatedRBAC(ctx)
	if err != nil {
		return err
	}
	if err := requireAPIKeyResource(ctx, iam.ResourceTypeDashboard, dashboardName, systemroles.PermissionDashboardView); err != nil {
		return err
	}
	if resourceSuperuser(ctx, au) {
		return nil
	}
	return s.requireDashboardGrant(ctx, au, dashboardName)
}

func (s *SickRockServer) requireAuthenticatedRBAC(ctx context.Context) (*armlayer.AuthenticatedUser, error) {
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	return au, nil
}

func (s *SickRockServer) requireDashboardGrant(ctx context.Context, au *armlayer.AuthenticatedUser, dashboardName string) error {
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

var workflowActionPermissions = map[repo.ResourceGrantAction]string{
	repo.ResourceActionView:  systemroles.PermissionWorkflowView,
	repo.ResourceActionStart: systemroles.PermissionWorkflowStart,
}

func (s *SickRockServer) requireWorkflowAccess(ctx context.Context, workflowID int, action repo.ResourceGrantAction) error {
	if workflowID <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("workflow_id is required"))
	}
	au, err := s.requireAuthenticatedRBAC(ctx)
	if err != nil {
		return err
	}
	perm, ok := workflowActionPermissions[action]
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("invalid workflow action"))
	}
	return s.authorizeWorkflowAccess(ctx, au, workflowID, action, perm)
}

// authorizeWorkflowAccess applies the API key scope, then lets superusers through, otherwise requires a workflow grant.
func (s *SickRockServer) authorizeWorkflowAccess(ctx context.Context, au *armlayer.AuthenticatedUser, workflowID int, action repo.ResourceGrantAction, perm string) error {
	if err := requireAPIKeyResource(ctx, iam.ResourceTypeWorkflow, strconv.Itoa(workflowID), perm); err != nil {
		return err
	}
	if resourceSuperuser(ctx, au) {
		return nil
	}
	return s.requireWorkflowGrant(ctx, au.User.ID, workflowID, action)
}

func (s *SickRockServer) requireWorkflowGrant(ctx context.Context, userID, workflowID int, action repo.ResourceGrantAction) error {
	ok, err := s.repo.UserHasWorkflowGrant(ctx, userID, workflowID, action)
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
	// API keys go through the target's checks, which apply the key's scope.
	if isSuperuser(au) && iam.APIKeyScopeFromContext(ctx) == nil {
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
	case isWorkflowHubNav(item, tableName, dashboardName):
		return s.requireWorkflowAccess(ctx, int(item.WorkflowID.Int64), repo.ResourceActionView) == nil
	case dashboardName != "":
		return s.requireDashboardAccess(ctx, dashboardName) == nil
	case tableName != "":
		return s.requireTableAccess(ctx, tableName, repo.ResourceActionView) == nil
	}
	return false
}

func isWorkflowHubNav(item repo.NavigationItem, tableName, dashboardName string) bool {
	return item.WorkflowID.Valid && item.WorkflowID.Int64 > 0 && tableName == "" && dashboardName == ""
}

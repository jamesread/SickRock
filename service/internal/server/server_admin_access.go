package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/armature-iam/rbac"
)

var systemSettingsTableConfigurations = map[string]bool{
	"table_settings":             true,
	"table_navigation":           true,
	"table_apps":                 true,
	"table_app_navigation_links": true,
	"table_configurations":       true,
	"table_workflows":            true,
	"table_dashboards":           true,
	"table_dashboard_components": true,
	"device_codes":               true,
}

func isSystemSettingsTableConfiguration(tcName string) bool {
	return systemSettingsTableConfigurations[tcName]
}

func (s *SickRockServer) requireSystemSettings(ctx context.Context) error {
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) {
		return nil
	}
	if au.RBAC != nil && au.RBAC.Has(rbac.PermissionSystemSettings) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", rbac.PermissionSystemSettings))
}

func (s *SickRockServer) canDiscoverTableConfiguration(ctx context.Context, tcName string) bool {
	switch {
	case tcName == audit.TableConfiguration:
		return s.requireAuditView(ctx) == nil
	case tcName == tableReadOnlyExportsTCName:
		au := s.authUser(ctx)
		// Has is nil-safe and true for superusers.
		return isAuthenticated(au) && au.RBAC.Has(exportsManagePermission)
	case isSystemSettingsTableConfiguration(tcName):
		return s.requireSystemSettings(ctx) == nil
	}
	return true
}

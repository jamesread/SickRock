package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jamesread/armature-iam/rbac"
	"github.com/jamesread/SickRock/internal/audit"
)

var systemSettingsTableConfigurations = map[string]bool{
	"table_settings":        true,
	"table_navigation":      true,
	"table_configurations":  true,
	"table_workflows":       true,
	"table_dashboards":      true,
	"device_codes":          true,
}

func isSystemSettingsTableConfiguration(tcName string) bool {
	return systemSettingsTableConfigurations[tcName]
}

func (s *SickRockServer) requireSystemSettings(ctx context.Context) error {
	au := s.authUserRBAC(ctx)
	if au == nil || au.User == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if au.RBAC != nil && au.RBAC.IsSuperuser {
		return nil
	}
	if au.RBAC != nil && au.RBAC.Has(rbac.PermissionSystemSettings) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", rbac.PermissionSystemSettings))
}

func (s *SickRockServer) requireSystemSettingsForTable(ctx context.Context, tcName string) error {
	if tcName == "" || !isSystemSettingsTableConfiguration(tcName) {
		return nil
	}
	return s.requireSystemSettings(ctx)
}

func (s *SickRockServer) canDiscoverTableConfiguration(ctx context.Context, tcName string) bool {
	if tcName == audit.TableConfiguration {
		return s.requireAuditView(ctx) == nil
	}
	if tcName == tableReadOnlyExportsTCName {
		au := s.authUser(ctx)
		if au == nil || au.User == nil {
			return false
		}
		if au.RBAC != nil && (au.RBAC.IsSuperuser || au.RBAC.Has(exportsManagePermission)) {
			return true
		}
		return false
	}
	if isSystemSettingsTableConfiguration(tcName) {
		return s.requireSystemSettings(ctx) == nil
	}
	return true
}

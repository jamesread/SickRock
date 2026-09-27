package iam

import "github.com/jamesread/SickRock/internal/iam/systemroles"

const (
	PermissionAppRead       = systemroles.PermissionAppRead
	PermissionExportsView   = systemroles.PermissionExportsView
	PermissionWorkflowView  = systemroles.PermissionWorkflowView
	PermissionWorkflowStart = systemroles.PermissionWorkflowStart
	PermissionTableView     = systemroles.PermissionTableView
	PermissionTableInsert   = systemroles.PermissionTableInsert
	PermissionTableEdit     = systemroles.PermissionTableEdit
	PermissionTableDelete   = systemroles.PermissionTableDelete
	PermissionDashboardView   = systemroles.PermissionDashboardView
	PermissionAPIKeysUse      = systemroles.PermissionAPIKeysUse
	PermissionDeviceCodeClaim = systemroles.PermissionDeviceCodeClaim
	PermissionAuditView       = systemroles.PermissionAuditView
	RoleViewer                = systemroles.RoleViewer
)

// IsSystemRole reports SickRock system roles.
func IsSystemRole(name string) bool {
	return systemroles.IsSystemRole(name)
}

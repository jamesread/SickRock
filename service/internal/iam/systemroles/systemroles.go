package systemroles

import "github.com/jamesread/armature-iam/rbac"

const (
	PermissionAppRead     = "app.read"
	PermissionExportsView = "exports.view"

	PermissionWorkflowView  = "workflow.view"
	PermissionWorkflowStart = "workflow.start"
	PermissionTableView     = "table.view"
	PermissionTableInsert   = "table.insert"
	PermissionTableEdit     = "table.edit"
	PermissionTableDelete   = "table.delete"
	PermissionDashboardView = "dashboard.view"
	PermissionAPIKeysUse    = "apikeys.use"
	PermissionDeviceCodeClaim = "devicecode.claim"
	PermissionAuditView       = "audit.view"

	RoleViewer  = "viewer"
	RoleAuditor = "auditor"
)

// IsSystemRole reports SickRock system roles (includes armature superuser and member).
func IsSystemRole(name string) bool {
	return rbac.IsSystemRole(name) || name == RoleViewer || name == RoleAuditor
}

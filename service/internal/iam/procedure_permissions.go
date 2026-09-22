package iam

import (
	"github.com/jamesread/armature-iam/rbac"
	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
)

// RequiredPermission maps Connect procedures to RBAC permission names.
func RequiredPermission(procedureName string) string {
	switch procedureName {
	case sickrockpbconnect.SickRockResetUserPasswordProcedure:
		return rbac.PermissionUsersResetPassword
	case sickrockpbconnect.SickRockListUsersProcedure,
		sickrockpbconnect.SickRockGetUserProcedure:
		return rbac.PermissionUsersView
	case sickrockpbconnect.SickRockCreateUserProcedure:
		return rbac.PermissionUsersCreate
	case sickrockpbconnect.SickRockDeleteUserProcedure:
		return rbac.PermissionUsersDelete
	case sickrockpbconnect.SickRockChangePasswordProcedure:
		return rbac.PermissionAppAccess
	case sickrockpbconnect.SickRockListRbacPermissionsProcedure,
		sickrockpbconnect.SickRockListRbacRolesProcedure,
		sickrockpbconnect.SickRockGetUserRbacRolesProcedure,
		sickrockpbconnect.SickRockGetUserGroupRbacRolesProcedure,
		sickrockpbconnect.SickRockGetRbacRoleUsersProcedure,
		sickrockpbconnect.SickRockGetRbacRoleGroupsProcedure,
		sickrockpbconnect.SickRockGetMyPermissionsAuditProcedure:
		return rbac.PermissionRbacView
	case sickrockpbconnect.SickRockCreateRbacRoleProcedure,
		sickrockpbconnect.SickRockUpdateRbacRoleProcedure,
		sickrockpbconnect.SickRockDeleteRbacRoleProcedure,
		sickrockpbconnect.SickRockSetUserGroupRbacRolesProcedure:
		return rbac.PermissionRbacManage
	case sickrockpbconnect.SickRockListUserGroupsProcedure,
		sickrockpbconnect.SickRockGetUserGroupMembersProcedure:
		return rbac.PermissionUserGroupsView
	case sickrockpbconnect.SickRockCreateUserGroupProcedure,
		sickrockpbconnect.SickRockDeleteUserGroupProcedure,
		sickrockpbconnect.SickRockSetUserGroupMembersProcedure:
		return rbac.PermissionUserGroupsManage
	case sickrockpbconnect.SickRockCreateTableProcedure,
		sickrockpbconnect.SickRockCreateTableConfigurationProcedure,
		sickrockpbconnect.SickRockAddTableColumnProcedure,
		sickrockpbconnect.SickRockChangeColumnTypeProcedure,
		sickrockpbconnect.SickRockChangeColumnNameProcedure,
		sickrockpbconnect.SickRockDropColumnProcedure,
		sickrockpbconnect.SickRockCreateForeignKeyProcedure:
		return rbac.PermissionSystemSettings
	}
	return rbac.PermissionAppAccess
}
